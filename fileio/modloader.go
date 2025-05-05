package fileio

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
	"os"
)

// LoadMod attempts to load a mod file from a path
func LoadMod(modFile string) (core.ModToml, error) {
	var mod core.ModToml
	raw, err := os.ReadFile(modFile)
	if err != nil {
		return mod, err
	}
	if err := toml.Unmarshal(raw, &mod); err != nil {
		return mod, err
	}

	if err = mod.ReflectUpdateData(); err != nil {
		return mod, err
	}

	mod.SetMetaPath(modFile)
	return mod, nil
}
