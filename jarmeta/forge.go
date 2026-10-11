package jarmeta

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// forgeParser handles both Forge mods.toml and NeoForge neoforge.mods.toml,
// which share a layout and differ in how dependency kinds are declared.
type forgeParser struct {
	loader Loader
	path   string
}

func (p forgeParser) Path() string { return p.path }

type forgeFile struct {
	ModLoader    string                `toml:"modLoader"`
	LoaderVer    string                `toml:"loaderVersion"`
	Mods         []forgeMod            `toml:"mods"`
	Dependencies map[string][]forgeDep `toml:"dependencies"`
}

type forgeMod struct {
	ModID       string `toml:"modId"`
	Version     string `toml:"version"`
	DisplayName string `toml:"displayName"`
}

type forgeDep struct {
	ModID        string `toml:"modId"`
	Mandatory    *bool  `toml:"mandatory"`
	Type         string `toml:"type"`
	VersionRange string `toml:"versionRange"`
	Side         string `toml:"side"`
}

func (p forgeParser) Parse(data []byte) (LoaderMeta, error) {
	var f forgeFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return LoaderMeta{}, fmt.Errorf("decoding toml: %w", err)
	}
	if len(f.Mods) == 0 {
		return LoaderMeta{}, errors.New("no [[mods]] declared")
	}

	first := f.Mods[0]
	m := LoaderMeta{
		Loader:               p.loader,
		ModID:                first.ModID,
		Version:              first.Version,
		VersionIsPlaceholder: isPlaceholder(first.Version),
		Name:                 first.DisplayName,
		Environment:          EnvBoth,
		ModLoader:            f.ModLoader,
		LoaderVersionRange:   f.LoaderVer,
	}
	for _, mod := range f.Mods {
		if mod.ModID == "" {
			return LoaderMeta{}, errors.New("[[mods]] entry missing modId")
		}
		m.Mods = append(m.Mods, newMod(mod.ModID, mod.Version))
	}

	for _, owner := range sortedKeys(f.Dependencies) {
		for _, d := range f.Dependencies[owner] {
			if d.ModID == "" {
				continue
			}
			kind, mandatory := forgeKind(d)
			dep := Dependency{
				Owner:     owner,
				ID:        d.ModID,
				Kind:      kind,
				Mandatory: mandatory,
				Side:      forgeSide(d.Side),
			}
			if d.VersionRange != "" {
				dep.VersionRanges = []string{d.VersionRange}
			}
			m.Dependencies = append(m.Dependencies, dep)
		}
	}
	return m, nil
}

// forgeKind maps NeoForge "type" (preferred) or Forge "mandatory" (default
// true when absent) onto a kind and mandatory flag.
func forgeKind(d forgeDep) (DependencyKind, bool) {
	switch strings.ToLower(d.Type) {
	case "required":
		return KindDepends, true
	case "optional":
		return KindDepends, false
	case "incompatible":
		return KindBreaks, false
	case "discouraged":
		return KindConflicts, false
	}
	return KindDepends, d.Mandatory == nil || *d.Mandatory
}

func forgeSide(s string) Side {
	switch strings.ToUpper(s) {
	case "CLIENT":
		return SideClient
	case "SERVER":
		return SideServer
	default:
		return SideBoth
	}
}
