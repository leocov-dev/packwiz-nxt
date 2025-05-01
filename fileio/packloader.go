package fileio

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

// LoadPackFile loads the modpack metadata to a Pack struct
func LoadPackFile(packPath string) (core.Pack, error) {
	var modpack core.Pack
	raw, err := os.ReadFile(packPath)
	if err != nil {
		return core.Pack{}, err
	}
	if err := toml.Unmarshal(raw, &modpack); err != nil {
		return core.Pack{}, err
	}

	modpack.SetFilePath(packPath)

	// Check pack-format
	if len(modpack.PackFormat) == 0 {
		fmt.Println("Modpack manifest has no pack-format field; assuming packwiz:1.1.0")
		modpack.PackFormat = "packwiz:1.1.0"
	}
	// Auto-migrate versions
	if modpack.PackFormat == "packwiz:1.0.0" {
		fmt.Println("Automatically migrating pack to packwiz:1.1.0 format...")
		modpack.PackFormat = "packwiz:1.1.0"
	}
	if !strings.HasPrefix(modpack.PackFormat, "packwiz:") {
		return core.Pack{}, errors.New("pack-format field does not indicate a valid packwiz pack")
	}
	ver, err := semver.StrictNewVersion(strings.TrimPrefix(modpack.PackFormat, "packwiz:"))
	if err != nil {
		return core.Pack{}, fmt.Errorf("pack-format field is not valid semver: %w", err)
	}
	if !core.PackFormatConstraintAccepted.Check(ver) {
		return core.Pack{}, errors.New("the modpack is incompatible with this version of packwiz; please update")
	}
	if !core.PackFormatConstraintSuggestUpgrade.Check(ver) {
		fmt.Println("Modpack has a newer feature number than is supported by this version of packwiz. Update to the latest version of packwiz for new features and bugfixes!")
	}
	// TODO: suggest migration if necessary (primarily for 2.0.0)

	// Read options into viper
	if modpack.Options != nil {
		err := viper.MergeConfigMap(modpack.Options)
		if err != nil {
			return core.Pack{}, err
		}
	}

	if len(modpack.Index.File) == 0 {
		modpack.Index.File = "index.toml"
	}
	return modpack, nil
}

func LoadPackIndexFile(pack *core.Pack) (core.Index, error) {
	if filepath.IsAbs(pack.Index.File) {
		return LoadIndex(pack.Index.File)
	}
	fileNative := filepath.FromSlash(pack.Index.File)
	return LoadIndex(filepath.Join(pack.GetPackDir(), fileNative))
}
