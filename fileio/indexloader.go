package fileio

import (
	"github.com/leocov-dev/fork.packwiz/core"
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
