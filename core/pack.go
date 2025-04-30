package core

import (
	"errors"
	"github.com/Masterminds/semver/v3"
	"github.com/unascribed/FlexVer/go/flexver"
	"path/filepath"
)

// Pack stores the modpack metadata, usually in pack.toml
type Pack struct {
	Name        string `toml:"name"`
	Author      string `toml:"author,omitempty"`
	Version     string `toml:"version,omitempty"`
	Description string `toml:"description,omitempty"`
	PackFormat  string `toml:"pack-format"`
	Index       struct {
		// Path is stored in forward slash format relative to pack.toml
		File       string `toml:"file"`
		HashFormat string `toml:"hash-format"`
		Hash       string `toml:"hash,omitempty"`
	} `toml:"index"`
	Versions map[string]string                 `toml:"versions"`
	Export   map[string]map[string]interface{} `toml:"export"`
	Options  map[string]interface{}            `toml:"options"`

	filePath string
}

const CurrentPackFormat = "packwiz:1.1.0"

var PackFormatConstraintAccepted = mustParseConstraint("~1")
var PackFormatConstraintSuggestUpgrade = mustParseConstraint("~1.1")

func mustParseConstraint(s string) *semver.Constraints {
	c, err := semver.NewConstraint(s)
	if err != nil {
		panic(err)
	}
	return c
}

func (pack *Pack) RefreshIndexHash(format, hash string) {
	pack.Index.HashFormat = format
	pack.Index.Hash = hash
}

// GetMCVersion gets the version of Minecraft this pack uses, if it has been correctly specified
func (pack *Pack) GetMCVersion() (string, error) {
	mcVersion, ok := pack.Versions["minecraft"]
	if !ok {
		return "", errors.New("no minecraft version specified in modpack")
	}
	return mcVersion, nil
}

// GetSupportedMCVersions gets the versions of Minecraft this pack allows in downloaded mods, ordered by preference (highest = most desirable)
func (pack *Pack) GetSupportedMCVersions() ([]string, error) {
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return nil, err
	}
	allVersions := append(append([]string(nil), pack.GetAcceptableGameVersions()...), mcVersion)
	sortAndDedupeVersions(allVersions)
	return allVersions, nil
}

func (pack *Pack) GetAcceptableGameVersions() []string {
	return pack.Options["acceptable-game-versions"].([]string)
}

func (pack *Pack) SetAcceptableGameVersions(versions []string) {
	sortAndDedupeVersions(versions)
	pack.Options["acceptable-game-versions"] = versions
}

func (pack *Pack) GetPackName() string {
	if pack.Name == "" {
		return "export"
	} else if pack.Version == "" {
		return pack.Name
	} else {
		return pack.Name + "-" + pack.Version
	}
}

func (pack *Pack) GetCompatibleLoaders() (loaders []string) {
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

func (pack *Pack) GetLoaders() (loaders []string) {
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

func (pack *Pack) UpdateHash(_, _ string) {
	// noop for packs
}

func (pack *Pack) GetFilePath() string {
	return pack.filePath
}

func (pack *Pack) SetFilePath(path string) {
	pack.filePath = path
}

func (pack *Pack) GetPackDir() string {
	return filepath.Dir(pack.filePath)
}

func sortAndDedupeVersions(versions []string) {
	flexver.VersionSlice(versions).Sort()
	// Deduplicate the sorted array
	if len(versions) > 0 {
		j := 0
		for i := 1; i < len(versions); i++ {
			if versions[i] != versions[j] {
				j++
				versions[j] = versions[i]
			}
		}
		versions = versions[:j+1]
	}
}
