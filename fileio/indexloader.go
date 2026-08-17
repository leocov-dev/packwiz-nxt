package fileio

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
		rep.DefaultModHashFormat = core.DefaultHashFormat
	}
	rep.SetFilePath(indexFile)

	index, err := core.NewIndexFromTomlRepr(rep)
	if err != nil {
		return core.IndexFS{}, err
	}
	return index, nil
}

func LoadAllMods(index *core.IndexFS) ([]*core.ModToml, error) {
	modPaths, err := index.GetAllMods()
	if err != nil {
		return nil, err
	}
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

// RefreshIndexFiles updates the hashes of all the files in the index, and adds new files to the index.
// packFilePath is the path to the pack.toml file being refreshed against, used to exclude it from the
// file walk. progressFn, if non-nil, is called after each file is processed with the current file
// count, total file count, and the path just processed, allowing the caller to drive its own
// progress reporting (e.g. a terminal progress bar).
func RefreshIndexFiles(index *core.IndexFS, packFilePath string, progressFn func(current, total int, path string)) error {
	// Is case-sensitivity a problem?
	pathPF, err := filepath.Abs(packFilePath)
	if err != nil {
		return err
	}
	pathIndex, err := filepath.Abs(index.GetFilePath())
	if err != nil {
		return err
	}

	packRoot := index.GetPackRoot()
	pathIgnore, err := filepath.Abs(filepath.Join(packRoot, ".packwizignore"))
	if err != nil {
		return err
	}
	ignore, ignoreExists := readGitignore(pathIgnore)

	var fileList []string
	err = filepath.WalkDir(packRoot, func(path string, info os.DirEntry, err error) error {
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
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
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

	if err := hashFilesInto(index, fileList, progressFn); err != nil {
		return err
	}

	// Check all the files exist, remove them if they don't
	for p, file := range index.Files {
		found, err := file.MarkedFound()
		if err != nil {
			return err
		}
		if !found {
			delete(index.Files, p)
		}
	}

	return nil
}

// UpdateIndexFile hashes the file at path and records the result in the index.
func UpdateIndexFile(in *core.IndexFS, path string) error {
	hashString, markAsMetaFile, err := hashFile(path)
	if err != nil {
		return err
	}
	return in.UpdateFileHashGiven(path, core.DefaultHashFormat, hashString, markAsMetaFile)
}

// hashFile computes path's DefaultHashFormat hash and whether it should be marked as a
// meta file, doing no index access - safe to call concurrently across multiple files,
// unlike core.IndexFS.UpdateFileHashGiven, which mutates a plain unsynchronized map.
func hashFile(path string) (hashString string, markAsMetaFile bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}

	// Hash usage strategy (may change):
	// Just use SHA256, overwrite existing hash regardless of what it is
	// May update later to continue using the same hash that was already being used
	h, err := core.GetHashImpl(core.DefaultHashFormat)
	if err != nil {
		_ = f.Close()
		return "", false, err
	}
	if _, err := io.Copy(h, f); err != nil {
		_ = f.Close()
		return "", false, err
	}
	if err := f.Close(); err != nil {
		return "", false, err
	}

	// If the file has an extension of pw.toml, set markAsMetaFile to true
	markAsMetaFile = strings.HasSuffix(filepath.Base(path), core.MetaExtension)

	return h.String(), markAsMetaFile, nil
}

// hashFilesInto hashes each of paths using a bounded pool of runtime.NumCPU() worker
// goroutines (hashing is I/O-bound and safe to parallelize), while index.UpdateFileHashGiven
// - a plain unsynchronized map mutation - is only ever called from this function's own
// goroutine as results arrive. progressFn, if non-nil, is called once per file, in
// whatever order results complete in (no ordering guarantee vs. paths). The first error
// from any worker stops further dispatch and is returned once in-flight work drains.
func hashFilesInto(index *core.IndexFS, paths []string, progressFn func(current, total int, path string)) error {
	total := len(paths)
	if total == 0 {
		return nil
	}

	type result struct {
		path           string
		hashString     string
		markAsMetaFile bool
		err            error
	}

	pathCh := make(chan string)
	resultCh := make(chan result)
	stop := make(chan struct{})
	var stopOnce sync.Once
	cancel := func() { stopOnce.Do(func() { close(stop) }) }

	numWorkers := runtime.NumCPU()
	if numWorkers > total {
		numWorkers = total
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for path := range pathCh {
				hashString, markAsMetaFile, err := hashFile(path)
				resultCh <- result{path: path, hashString: hashString, markAsMetaFile: markAsMetaFile, err: err}
			}
		}()
	}
	go func() {
		defer close(pathCh)
		for _, path := range paths {
			select {
			case pathCh <- path:
			case <-stop:
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var firstErr error
	current := 0
	for res := range resultCh {
		if res.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to hash %s: %w", res.path, res.err)
				cancel()
			}
			continue
		}
		if firstErr != nil {
			// Already stopping - avoid doing more index work, but keep draining resultCh
			// so in-flight workers (dispatched before cancel) don't block forever on send.
			continue
		}
		if err := index.UpdateFileHashGiven(res.path, core.DefaultHashFormat, res.hashString, res.markAsMetaFile); err != nil {
			firstErr = err
			cancel()
			continue
		}
		current++
		if progressFn != nil {
			progressFn(current, total, res.path)
		}
	}

	return firstErr
}
