package sources

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/core/murmur2"
)

// CfDetectFingerprintMatch pairs a curseforge fingerprint with the on-disk file it was computed from,
// for files that could not be cleanly resolved to an exact mod match.
type CfDetectFingerprintMatch struct {
	Path        string
	Fingerprint uint32
}

// CfDetectResult is the outcome of scanning a directory of mod files and looking them up against the
// curseforge fingerprint API.
type CfDetectResult struct {
	// Mods contains a core.Mod for every file that was matched exactly.
	Mods []*core.Mod
	// MatchedCount is the number of files that were matched exactly (len(Mods)).
	MatchedCount int
	// PartialMatches are fingerprints curseforge only partially matched; the caller couldn't
	// automatically resolve these to a single mod/file.
	PartialMatches []CfDetectFingerprintMatch
	// UnmatchedFiles are fingerprints curseforge didn't recognize at all.
	UnmatchedFiles []CfDetectFingerprintMatch
}

// curseforgeFileHash computes the curseforge murmur2 fingerprint for a mod file's contents.
func curseforgeFileHash(data []byte) uint32 {
	h := murmur2.New()
	_, _ = h.Write(data)
	return h.Sum32()
}

// CurseforgeDetectMods walks dir looking for .jar/.litemod files, hashes them using curseforge's
// murmur2 fingerprint algorithm, and looks up the resulting fingerprints against the curseforge API
// to identify which curseforge mods/files they correspond to.
//
// A nil result and nil error indicates the fingerprint lookup itself failed in a way that was already
// reported (printed) and there is nothing further for the caller to do.
func CurseforgeDetectMods(dir string) (*CfDetectResult, error) {
	var hashes []uint32
	modPaths := make(map[uint32]string)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jar") && !strings.HasSuffix(path, ".litemod") {
			return nil
		}
		fmt.Println("Hashing " + path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := curseforgeFileHash(data)
		hashes = append(hashes, hash)
		modPaths[hash] = path
		return nil
	})
	if err != nil {
		return nil, err
	}
	fmt.Printf("Found %d files, submitting...\n", len(hashes))

	res, err := GetCurseforgeClient().GetFingerprintInfo(hashes)
	if err != nil {
		// Historically this case has been treated as non-fatal: report it and let the caller
		// know there's nothing further to do, rather than aborting the whole command.
		fmt.Println(err)
		return nil, nil
	}

	result := &CfDetectResult{
		MatchedCount: len(res.ExactFingerprints),
	}
	for _, v := range res.PartialMatches {
		result.PartialMatches = append(result.PartialMatches, CfDetectFingerprintMatch{
			Path:        modPaths[v],
			Fingerprint: v,
		})
	}
	for _, v := range res.UnmatchedFingerprints {
		result.UnmatchedFiles = append(result.UnmatchedFiles, CfDetectFingerprintMatch{
			Path:        modPaths[v],
			Fingerprint: v,
		})
	}

	fmt.Println("Retrieving metadata...")
	ids := make([]uint32, len(res.ExactMatches))
	for i, v := range res.ExactMatches {
		ids[i] = v.ID
	}
	modInfos, err := GetCurseforgeClient().GetModInfoMultiple(ids)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve metadata: %w", err)
	}
	modInfosMap := make(map[uint32]CfModInfo)
	for _, v := range modInfos {
		modInfosMap[v.ID] = v
	}

	fmt.Println("Creating metadata files...")
	for _, v := range res.ExactMatches {
		mod, err := CurseforgeNewMod(modInfosMap[v.ID], v.File, false)
		if err != nil {
			return nil, err
		}
		result.Mods = append(result.Mods, mod)
	}

	return result, nil
}
