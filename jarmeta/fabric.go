package jarmeta

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
)

type fabricParser struct{}

func (fabricParser) Path() string { return "fabric.mod.json" }

type fabricFile struct {
	ID          string                `json:"id"`
	Version     string                `json:"version"`
	Name        string                `json:"name"`
	Environment string                `json:"environment"`
	Provides    []string              `json:"provides"`
	Depends     map[string]stringList `json:"depends"`
	Recommends  map[string]stringList `json:"recommends"`
	Suggests    map[string]stringList `json:"suggests"`
	Breaks      map[string]stringList `json:"breaks"`
	Conflicts   map[string]stringList `json:"conflicts"`
}

func (fabricParser) Parse(data []byte) (LoaderMeta, error) {
	var f fabricFile
	if err := json.Unmarshal(data, &f); err != nil {
		return LoaderMeta{}, fmt.Errorf("decoding json: %w", err)
	}
	if f.ID == "" {
		return LoaderMeta{}, errors.New("missing mod id")
	}

	m := LoaderMeta{
		Loader:               LoaderFabric,
		ModID:                f.ID,
		Version:              f.Version,
		VersionIsPlaceholder: isPlaceholder(f.Version),
		Name:                 f.Name,
		Mods:                 []Mod{newMod(f.ID, f.Version)},
		Provides:             f.Provides,
		Environment:          fabricEnvironment(f.Environment),
	}
	groups := []struct {
		kind      DependencyKind
		mandatory bool
		deps      map[string]stringList
	}{
		{KindDepends, true, f.Depends},
		{KindRecommends, false, f.Recommends},
		{KindSuggests, false, f.Suggests},
		{KindBreaks, false, f.Breaks},
		{KindConflicts, false, f.Conflicts},
	}
	for _, g := range groups {
		for _, id := range slices.Sorted(maps.Keys(g.deps)) {
			m.Dependencies = append(m.Dependencies, Dependency{
				Owner:         f.ID,
				ID:            id,
				Kind:          g.kind,
				VersionRanges: g.deps[id],
				Mandatory:     g.mandatory,
				Side:          SideBoth,
			})
		}
	}
	return m, nil
}

func fabricEnvironment(s string) Environment {
	switch s {
	case "client":
		return EnvClient
	case "server":
		return EnvServer
	default:
		return EnvBoth
	}
}
