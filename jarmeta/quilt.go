package jarmeta

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type quiltParser struct{}

func (quiltParser) Path() string { return "quilt.mod.json" }

type quiltFile struct {
	Loader struct {
		ID       string         `json:"id"`
		Version  string         `json:"version"`
		Provides []quiltProvide `json:"provides"`
		Jars     []string       `json:"jars"`
		Depends  []quiltDep     `json:"depends"`
		Breaks   []quiltDep     `json:"breaks"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"quilt_loader"`
	Minecraft struct {
		Environment string `json:"environment"`
	} `json:"minecraft"`
}

// quiltProvide is a string id or an object with an "id" and optional "version".
type quiltProvide struct{ ID, Version string }

// UnmarshalJSON implements json.Unmarshaler.
func (p *quiltProvide) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &p.ID)
	}
	var obj struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	p.ID, p.Version = obj.ID, obj.Version
	return nil
}

// quiltVersions is a string, an array (any-of), or {"any":..,"all":..}.
type quiltVersions struct{ Any, All stringList }

// UnmarshalJSON implements json.Unmarshaler.
func (v *quiltVersions) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '{' {
		var obj struct {
			Any stringList `json:"any"`
			All stringList `json:"all"`
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		v.Any, v.All = obj.Any, obj.All
		return nil
	}
	return json.Unmarshal(b, &v.Any)
}

// quiltDep is a string id, an object, or an array of those (an alternative
// group, held in AnyOf).
type quiltDep struct {
	ID       string
	Versions quiltVersions
	Optional bool
	AnyOf    []quiltDep
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *quiltDep) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return errors.New("empty dependency")
	}
	switch b[0] {
	case '"':
		return json.Unmarshal(b, &d.ID)
	case '[':
		return json.Unmarshal(b, &d.AnyOf)
	}
	var obj struct {
		ID       string        `json:"id"`
		Versions quiltVersions `json:"versions"`
		Optional bool          `json:"optional"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	d.ID, d.Versions, d.Optional = obj.ID, obj.Versions, obj.Optional
	return nil
}

func (quiltParser) Parse(data []byte) (LoaderMeta, error) {
	var f quiltFile
	if err := json.Unmarshal(data, &f); err != nil {
		return LoaderMeta{}, fmt.Errorf("decoding json: %w", err)
	}
	q := f.Loader
	if q.ID == "" {
		return LoaderMeta{}, errors.New("missing quilt_loader.id")
	}

	m := LoaderMeta{
		Loader:               LoaderQuilt,
		ModID:                q.ID,
		Version:              q.Version,
		VersionIsPlaceholder: isPlaceholder(q.Version),
		Name:                 q.Metadata.Name,
		Mods:                 []Mod{newMod(q.ID, q.Version)},
		Environment:          quiltEnvironment(f.Minecraft.Environment),
	}
	for _, p := range q.Provides {
		if p.ID != "" {
			m.Provides = append(m.Provides, p.ID)
			if p.Version != "" {
				if m.ProvidesVersions == nil {
					m.ProvidesVersions = map[string]string{}
				}
				m.ProvidesVersions[p.ID] = p.Version
			}
		}
	}
	for _, j := range q.Jars {
		if j != "" {
			m.NestedJars = append(m.NestedJars, j)
		}
	}
	m.Dependencies = append(m.Dependencies, quiltDependencies(q.ID, q.Depends, KindDepends)...)
	m.Dependencies = append(m.Dependencies, quiltDependencies(q.ID, q.Breaks, KindBreaks)...)
	return m, nil
}

// quiltDependencies converts decoded entries to Dependencies. An alternative
// group becomes a Dependency with empty ID and AnyOf set; empty groups and
// id-less entries are dropped.
func quiltDependencies(owner string, deps []quiltDep, kind DependencyKind) []Dependency {
	var out []Dependency
	for _, d := range deps {
		dep := Dependency{
			Owner:     owner,
			Kind:      kind,
			Mandatory: kind == KindDepends && !d.Optional,
			Side:      SideBoth,
		}
		if d.AnyOf != nil {
			dep.AnyOf = quiltDependencies(owner, d.AnyOf, kind)
			if len(dep.AnyOf) == 0 {
				continue
			}
		} else {
			if d.ID == "" {
				continue
			}
			dep.ID = d.ID
			dep.VersionRanges = d.Versions.Any
			dep.VersionRangesAll = d.Versions.All
		}
		out = append(out, dep)
	}
	return out
}

func quiltEnvironment(s string) Environment {
	switch s {
	case "client":
		return EnvClient
	case "dedicated_server", "server":
		return EnvServer
	default:
		return EnvBoth
	}
}
