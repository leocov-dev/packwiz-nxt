package fileio

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
	"os"
)

// LoadIndex attempts to load the index file from a path
func LoadIndex(indexFile string) (core.Index, error) {
	// Decode as indexTomlRepresentation then convert to Index
	var rep core.IndexTomlRepresentation
	raw, err := os.ReadFile(indexFile)
	if err != nil {
		return core.Index{}, err
	}
	if err := toml.Unmarshal(raw, &rep); err != nil {
		return core.Index{}, err
	}
	if len(rep.DefaultModHashFormat) == 0 {
		rep.DefaultModHashFormat = "sha256"
	}
	rep.SetFilePath(indexFile)

	index := core.NewIndexFromTomlRepr(rep)
	return index, nil
}

func LoadAllMods(index *core.Index) ([]*core.Mod, error) {
	modPaths := index.GetAllMods()
	mods := make([]*core.Mod, len(modPaths))
	for i, v := range modPaths {
		modData, err := LoadMod(v)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata file %s: %w", v, err)
		}
		mods[i] = &modData
	}
	return mods, nil
}
