package jarmeta

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

const (
	// MaxNestedJarSize is the hard cap, in bytes, on one nested (jar-in-jar)
	// jar read into memory.
	MaxNestedJarSize = 64 << 20
	// MaxNestedTotalSize is the cap on the nested jar bytes read for one
	// outer jar. Nested jars past the budget are left unopened.
	MaxNestedTotalSize = 256 << 20

	jarjarPath = "META-INF/jarjar/metadata.json"
)

// jarjarFile is the Forge/NeoForge jar-in-jar manifest.
type jarjarFile struct {
	Jars []struct {
		Path string `json:"path"`
	} `json:"jars"`
}

// jarjarPaths returns the nested jar paths declared by the Forge/NeoForge
// jar-in-jar manifest, or nil when the jar has none.
func jarjarPaths(zr *zip.Reader) ([]string, error) {
	f := findEntry(zr, jarjarPath)
	if f == nil {
		return nil, nil
	}
	data, err := readEntry(f)
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %w", ErrMalformed, jarjarPath, err)
	}
	var jj jarjarFile
	if err := json.Unmarshal(data, &jj); err != nil {
		return nil, fmt.Errorf("%w: parsing %s: %w", ErrMalformed, jarjarPath, err)
	}
	var paths []string
	for _, j := range jj.Jars {
		if j.Path != "" {
			paths = append(paths, j.Path)
		}
	}
	return paths, nil
}

// attachNested records the nested jars of every meta and opens them, one
// level deep, to list the mods they bundle. A nested jar that is missing,
// oversized, over budget or unreadable is skipped: it stays in NestedJars
// but is not counted in NestedOpened.
func attachNested(zr *zip.Reader, parsers []formatParser, metas []LoaderMeta) error {
	jj, err := jarjarPaths(zr)
	opened := map[string][]Mod{}
	failed := map[string]bool{}
	budget := int64(MaxNestedTotalSize)

	for i := range metas {
		m := &metas[i]
		if m.Loader == LoaderForge || m.Loader == LoaderNeoForge {
			for _, p := range jj {
				if !slices.Contains(m.NestedJars, p) {
					m.NestedJars = append(m.NestedJars, p)
				}
			}
		}
		var bundled []Mod
		for _, p := range m.NestedJars {
			if failed[p] {
				continue
			}
			mods, ok := opened[p]
			if !ok {
				mods, ok = openNested(zr, parsers, p, &budget)
				if !ok {
					failed[p] = true
					continue
				}
				opened[p] = mods
			}
			m.NestedOpened++
			bundled = append(bundled, mods...)
		}
		m.Bundled = bundled
	}
	return err
}

// openNested reads the nested jar at path and returns the mods its loader
// metadata declares. Nested jars inside it are not followed.
func openNested(zr *zip.Reader, parsers []formatParser, path string, budget *int64) ([]Mod, bool) {
	f := findEntry(zr, path)
	if f == nil || f.UncompressedSize64 > MaxNestedJarSize || int64(f.UncompressedSize64) > *budget {
		return nil, false
	}
	rc, err := f.Open()
	if err != nil {
		return nil, false
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, MaxNestedJarSize+1))
	if err != nil || len(data) > MaxNestedJarSize {
		return nil, false
	}
	*budget -= int64(len(data))

	inner, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, false
	}
	metas, _ := parseZip(inner, parsers)
	var mods []Mod
	seen := map[string]bool{}
	for _, m := range metas {
		for _, mod := range m.Mods {
			if mod.ID != "" && !seen[mod.ID] {
				seen[mod.ID] = true
				mods = append(mods, mod)
			}
		}
	}
	return mods, true
}
