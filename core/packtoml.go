package core

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/exp/slices"
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
// It returns the pack's Options map (so the caller can merge it into its own
// config store, e.g. viper, if desired) and any non-fatal warning messages
// generated during validation.
func ValidatePack(pack *PackToml) (map[string]interface{}, []string, error) {
	var warnings []string

	// Check pack-format
	if len(pack.PackFormat) == 0 {
		warnings = append(warnings, "Modpack manifest has no pack-format field; assuming packwiz:1.1.0")
		pack.PackFormat = "packwiz:1.1.0"
	}
	// Auto-migrate versions
	if pack.PackFormat == "packwiz:1.0.0" {
		warnings = append(warnings, "Automatically migrating pack to packwiz:1.1.0 format...")
		pack.PackFormat = "packwiz:1.1.0"
	}
	if !strings.HasPrefix(pack.PackFormat, "packwiz:") {
		return nil, warnings, errors.New("pack-format field does not indicate a valid packwiz pack")
	}
	ver, err := semver.StrictNewVersion(strings.TrimPrefix(pack.PackFormat, "packwiz:"))
	if err != nil {
		return nil, warnings, fmt.Errorf("pack-format field is not valid semver: %w", err)
	}
	if !PackFormatConstraintAccepted.Check(ver) {
		return nil, warnings, errors.New("the pack is incompatible with this version of packwiz; please update")
	}
	if !PackFormatConstraintSuggestUpgrade.Check(ver) {
		warnings = append(warnings, "Modpack has a newer feature number than is supported by this version of packwiz. Update to the latest version of packwiz for new features and bugfixes!")
	}

	// TODO: suggest migration if necessary (primarily for 2.0.0)

	if len(pack.Index.File) == 0 {
		pack.Index.File = "index.toml"
	}

	return pack.Options, warnings, nil
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
	return mcVersionFrom(pack.Versions)
}

// GetSupportedMCVersions gets the versions of Minecraft this pack allows in downloaded mods, ordered by preference (highest = most desirable)
func (pack *PackToml) GetSupportedMCVersions() ([]string, error) {
	return supportedMCVersionsFrom(pack.Versions, pack.Options)
}

// GetAcceptableGameVersions returns the pack's "acceptable-game-versions" option as a []string.
// TOML-decoded options stored as interface{} may come back as either []string (if set
// programmatically) or []interface{} (if decoded from a TOML file), so both are handled here
// instead of panicking on an unchecked type assertion.
func (pack *PackToml) GetAcceptableGameVersions() ([]string, error) {
	return acceptableGameVersionsFrom(pack.Options)
}

func (pack *PackToml) SetAcceptableGameVersions(versions []string) {
	setAcceptableGameVersions(pack.Options, versions)
}

// AddAcceptableVersion adds a single version to the pack's acceptable Minecraft versions list.
// It returns an error if the version is already present in the list.
func (pack *PackToml) AddAcceptableVersion(version string) error {
	currentVersions, err := pack.GetAcceptableGameVersions()
	if err != nil {
		return err
	}
	if slices.Contains(currentVersions, version) {
		return fmt.Errorf("version %s is already in the acceptable versions list", version)
	}
	pack.SetAcceptableGameVersions(append(currentVersions, version))
	return nil
}

// RemoveAcceptableVersion removes a single version from the pack's acceptable Minecraft versions list.
// It returns an error if the version is not present in the list.
func (pack *PackToml) RemoveAcceptableVersion(version string) error {
	currentVersions, err := pack.GetAcceptableGameVersions()
	if err != nil {
		return err
	}
	i := slices.Index(currentVersions, version)
	if i == -1 {
		return fmt.Errorf("version %s is not in the acceptable versions list", version)
	}
	pack.SetAcceptableGameVersions(slices.Delete(currentVersions, i, i+1))
	return nil
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
	return compatibleLoadersFrom(pack.Versions)
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
