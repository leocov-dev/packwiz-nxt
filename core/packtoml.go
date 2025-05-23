package core

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
	"path/filepath"
	"strings"
)

// PackToml stores the modpack metadata, usually in pack.toml
type PackToml struct {
	Name        string                            `toml:"name"`
	Author      string                            `toml:"author,omitempty"`
	Version     string                            `toml:"version,omitempty"`
	Description string                            `toml:"description,omitempty"`
	PackFormat  string                            `toml:"pack-format"`
	Index       PackTomlIndex                     `toml:"index"`
	Versions    map[string]string                 `toml:"versions"`
	Export      map[string]map[string]interface{} `toml:"export"`
	Options     map[string]interface{}            `toml:"options"`

	filePath string
}

type PackTomlIndex struct {
	File       string `toml:"file"`
	HashFormat string `toml:"hash-format"`
	Hash       string `toml:"hash,omitempty"`
}

const CurrentPackFormat = "packwiz:1.1.0"

var PackFormatConstraintAccepted = mustParseConstraint("~1")
var PackFormatConstraintSuggestUpgrade = mustParseConstraint("~1.1")

func CreatePackToml(name, author, version string, versions map[string]string) *PackToml {
	return &PackToml{
		Name:       name,
		Author:     author,
		Version:    version,
		PackFormat: CurrentPackFormat,
		Index: PackTomlIndex{
			File: "index.toml",
		},
		Versions: versions,
	}
}

// ValidatePack run some basic validation and migrate the pack if possible.
func ValidatePack(pack *PackToml) error {
	// Check pack-format
	if len(pack.PackFormat) == 0 {
		fmt.Println("Modpack manifest has no pack-format field; assuming packwiz:1.1.0")
		pack.PackFormat = "packwiz:1.1.0"
	}
	// Auto-migrate versions
	if pack.PackFormat == "packwiz:1.0.0" {
		fmt.Println("Automatically migrating pack to packwiz:1.1.0 format...")
		pack.PackFormat = "packwiz:1.1.0"
	}
	if !strings.HasPrefix(pack.PackFormat, "packwiz:") {
		return errors.New("pack-format field does not indicate a valid packwiz pack")
	}
	ver, err := semver.StrictNewVersion(strings.TrimPrefix(pack.PackFormat, "packwiz:"))
	if err != nil {
		return fmt.Errorf("pack-format field is not valid semver: %w", err)
	}
	if !PackFormatConstraintAccepted.Check(ver) {
		return errors.New("the pack is incompatible with this version of packwiz; please update")
	}
	if !PackFormatConstraintSuggestUpgrade.Check(ver) {
		fmt.Println("Modpack has a newer feature number than is supported by this version of packwiz. Update to the latest version of packwiz for new features and bugfixes!")
	}

	// TODO: suggest migration if necessary (primarily for 2.0.0)

	// Read options into viper
	if pack.Options != nil {
		err := viper.MergeConfigMap(pack.Options)
		if err != nil {
			return err
		}
	}

	if len(pack.Index.File) == 0 {
		pack.Index.File = "index.toml"
	}

	return nil
}

func mustParseConstraint(s string) *semver.Constraints {
	c, err := semver.NewConstraint(s)
	if err != nil {
		panic(err)
	}
	return c
}

func (pack *PackToml) RefreshIndexHash(index IndexFS) {
	pack.Index.HashFormat = index.GetHashFormat()
	pack.Index.Hash = index.GetHash()
}

// GetMCVersion gets the version of Minecraft this pack uses, if it has been correctly specified
func (pack *PackToml) GetMCVersion() (string, error) {
	mcVersion, ok := pack.Versions["minecraft"]
	if !ok {
		return "", errors.New("no minecraft version specified in modpack")
	}
	return mcVersion, nil
}

// GetSupportedMCVersions gets the versions of Minecraft this pack allows in downloaded mods, ordered by preference (highest = most desirable)
func (pack *PackToml) GetSupportedMCVersions() ([]string, error) {
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return nil, err
	}
	allVersions := append(append([]string(nil), pack.GetAcceptableGameVersions()...), mcVersion)
	SortAndDedupeVersions(allVersions)
	return allVersions, nil
}

func (pack *PackToml) GetAcceptableGameVersions() []string {
	acceptableVersions, ok := pack.Options["acceptable-game-versions"]
	if !ok {
		return []string{}
	}
	return acceptableVersions.([]string)
}

func (pack *PackToml) SetAcceptableGameVersions(versions []string) {
	SortAndDedupeVersions(versions)
	pack.Options["acceptable-game-versions"] = versions
}

func (pack *PackToml) GetPackName() string {
	if pack.Name == "" {
		return "export"
	} else if pack.Version == "" {
		return pack.Name
	} else {
		return pack.Name + "-" + pack.Version
	}
}

func (pack *PackToml) GetCompatibleLoaders() (loaders []string) {
	if _, hasQuilt := pack.Versions["quilt"]; hasQuilt {
		loaders = append(loaders, "quilt")
		loaders = append(loaders, "fabric") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasFabric := pack.Versions["fabric"]; hasFabric {
		loaders = append(loaders, "fabric")
	}
	if _, hasNeoForge := pack.Versions["neoforge"]; hasNeoForge {
		loaders = append(loaders, "neoforge")
		loaders = append(loaders, "forge") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasForge := pack.Versions["forge"]; hasForge {
		loaders = append(loaders, "forge")
	}
	return
}

func (pack *PackToml) GetLoaders() (loaders []string) {
	if _, hasQuilt := pack.Versions["quilt"]; hasQuilt {
		loaders = append(loaders, "quilt")
	}
	if _, hasFabric := pack.Versions["fabric"]; hasFabric {
		loaders = append(loaders, "fabric")
	}
	if _, hasNeoForge := pack.Versions["neoforge"]; hasNeoForge {
		loaders = append(loaders, "neoforge")
	}
	if _, hasForge := pack.Versions["forge"]; hasForge {
		loaders = append(loaders, "forge")
	}
	return
}

func (pack *PackToml) UpdateHash(_, _ string) {
	// noop for packs
}

func (pack *PackToml) GetFilePath() string {
	return pack.filePath
}

func (pack *PackToml) SetFilePath(path string) {
	pack.filePath = path
}

func (pack *PackToml) GetPackDir() string {
	return filepath.Dir(pack.filePath)
}

func (pack *PackToml) Marshal() (MarshalResult, error) {
	result := MarshalResult{}

	var err error
	result.Value, err = toml.Marshal(pack)
	if err != nil {
		return result, err
	}

	return result, nil
}
