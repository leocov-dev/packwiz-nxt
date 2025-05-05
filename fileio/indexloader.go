package fileio

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadIndex attempts to load the index file from a path
func LoadIndex(indexFile string) (core.IndexFS, error) {
	// Decode as indexTomlRepresentation then convert to IndexFS
	var rep core.IndexTomlRepresentation
	raw, err := os.ReadFile(indexFile)
	if err != nil {
		return core.IndexFS{}, err
	}
	if err := toml.Unmarshal(raw, &rep); err != nil {
		return core.IndexFS{}, err
	}
	if len(rep.DefaultModHashFormat) == 0 {
		rep.DefaultModHashFormat = "sha256"
	}
	rep.SetFilePath(indexFile)

	index := core.NewIndexFromTomlRepr(rep)
	return index, nil
}

func LoadAllMods(index *core.IndexFS) ([]*core.ModToml, error) {
	modPaths := index.GetAllMods()
	mods := make([]*core.ModToml, len(modPaths))
	for i, v := range modPaths {
		modData, err := LoadMod(v)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata file %s: %w", v, err)
		}
		mods[i] = &modData
	}
	return mods, nil
}

// RefreshIndexFiles updates the hashes of all the files in the index, and adds new files to the index
func RefreshIndexFiles(index *core.IndexFS) error {
	// TODO: If needed, multithreaded hashing
	// for i := 0; i < runtime.NumCPU(); i++ {}

	// Is case-sensitivity a problem?
	pathPF, _ := filepath.Abs(viper.GetString("pack-file"))
	pathIndex, _ := filepath.Abs(index.GetFilePath())

	packRoot := index.GetPackRoot()
	pathIgnore, _ := filepath.Abs(filepath.Join(packRoot, ".packwizignore"))
	ignore, ignoreExists := readGitignore(pathIgnore)

	var fileList []string
	err := filepath.WalkDir(packRoot, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			// TODO: Handle errors on individual files properly
			return err
		}

		// Never ignore pack root itself (gitignore doesn't allow ignoring the root)
		if path == packRoot {
			return nil
		}

		if info.IsDir() {
			// Don't traverse ignored directories (consistent with Git handling of ignored dirs)
			if ignore.MatchesPath(path) {
				return fs.SkipDir
			}
			// Don't add directories to the file list
			return nil
		}
		// Exit if the files are the same as the pack/index files
		absPath, _ := filepath.Abs(path)
		if absPath == pathPF || absPath == pathIndex {
			return nil
		}
		if ignoreExists {
			if absPath == pathIgnore {
				return nil
			}
		}
		if ignore.MatchesPath(path) {
			return nil
		}

		fileList = append(fileList, path)
		return nil
	})
	if err != nil {
		return err
	}

	progressContainer := mpb.New()
	progress := progressContainer.AddBar(int64(len(fileList)),
		mpb.PrependDecorators(
			// simple name decorator
			decor.Name("Refreshing index..."),
			// decor.DSyncWidth bit enables column width synchronization
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			// replace ETA decorator with a "done" message, OnComplete event
			decor.OnComplete(
				// ETA decorator with ewma age of 60
				decor.EwmaETA(decor.ET_STYLE_GO, 60), "done",
			),
		),
	)

	for _, v := range fileList {
		start := time.Now()

		err := UpdateIndexFile(index, v)
		if err != nil {
			return err
		}

		progress.Increment(time.Since(start))
	}
	// Close bar
	progress.SetTotal(int64(len(fileList)), true) // If len = 0, we have to manually set complete to true
	progressContainer.Wait()

	// Check all the files exist, remove them if they don't
	for p, file := range index.Files {
		if !file.MarkedFound() {
			delete(index.Files, p)
		}
	}

	return nil
}

func UpdateIndexFile(in *core.IndexFS, path string) error {
	var hashString string

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	// Hash usage strategy (may change):
	// Just use SHA256, overwrite existing hash regardless of what it is
	// May update later to continue using the same hash that was already being used
	h, err := core.GetHashImpl("sha256")
	if err != nil {
		_ = f.Close()
		return err
	}
	if _, err := io.Copy(h, f); err != nil {
		_ = f.Close()
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	hashString = h.String()

	markAsMetaFile := false
	// If the file has an extension of pw.toml, set markAsMetaFile to true
	if strings.HasSuffix(filepath.Base(path), core.MetaExtension) {
		markAsMetaFile = true
	}

	return in.UpdateFileHashGiven(path, "sha256", hashString, markAsMetaFile)
}
