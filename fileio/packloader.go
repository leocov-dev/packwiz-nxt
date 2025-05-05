package fileio

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
)

// LoadPackFile loads the modpack metadata to a PackToml struct
func LoadPackFile(packPath string) (core.PackToml, error) {
	var modpack core.PackToml
	raw, err := os.ReadFile(packPath)
	if err != nil {
		return core.PackToml{}, err
	}
	if err := toml.Unmarshal(raw, &modpack); err != nil {
		return core.PackToml{}, err
	}

	modpack.SetFilePath(packPath)

	if err = core.ValidatePack(&modpack); err != nil {
		return core.PackToml{}, err
	}

	return modpack, nil
}

func LoadPackIndexFile(pack *core.PackToml) (core.IndexFS, error) {
	if filepath.IsAbs(pack.Index.File) {
		return LoadIndex(pack.Index.File)
	}
	fileNative := filepath.FromSlash(pack.Index.File)
	return LoadIndex(filepath.Join(pack.GetPackDir(), fileNative))
}
