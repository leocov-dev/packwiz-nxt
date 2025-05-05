package packwiz

import (
	"errors"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
)

type Pack struct {
	Name        string
	Author      string
	Version     string
	Description string
	PackFormat  string
	Versions    map[string]string
	Export      map[string]map[string]interface{}
	Options     map[string]interface{}
	Mods        map[string]*Mod
}

type LoaderInfo map[string]string

func NewPack(
	name string,
	author string,
	version string,
	description string,
	mcVersion string,
	loaderInfo LoaderInfo,
) *Pack {
	versions := map[string]string{
		"minecraft": mcVersion,
	}

	for k, v := range loaderInfo {
		versions[k] = v
	}

	return &Pack{
		Name:        name,
		Author:      author,
		Version:     version,
		Description: description,
		PackFormat:  core.CurrentPackFormat,
		Versions:    versions,
	}
}

func (p *Pack) SetMod(modSlug string, mod *Mod) {
	if p.Mods == nil {
		p.Mods = make(map[string]*Mod)
	}

	p.Mods[modSlug] = mod
}

func (p *Pack) AsPackToml() (string, error) {
	indexToml, err := p.getIndex()
	if err != nil {
		return "", err
	}

	packToml := core.PackToml{
		Name:        p.Name,
		Author:      p.Author,
		Version:     p.Version,
		Description: p.Description,
		PackFormat:  p.PackFormat,
		Index:       indexToml,
		Versions:    p.Versions,
		Export:      p.Export,
		Options:     p.Options,
	}

	result, err := packToml.Marshal()
	if err != nil {
		return "", err
	}

	return string(result.Value), nil
}

func (p *Pack) AsIndexToml() (string, string, error) {
	repr, err := p.getIndexRepr()
	if err != nil {
		return "", "", err
	}

	result, err := repr.Marshal()
	if err != nil {
		return "", "", err
	}

	return result.String(), result.Hash, nil
}

func (p *Pack) AsModToml(modSlug string) (string, error) {
	mod, ok := p.Mods[modSlug]
	if !ok {
		return "", fmt.Errorf("mod %s not found", modSlug)
	}

	text, _, err := mod.Serialize()
	return text, err
}

func (p *Pack) getIndexRepr() (core.IndexTomlRepresentation, error) {
	repr := core.IndexTomlRepresentation{
		DefaultModHashFormat: "sha256",
		Files:                make(core.IndexFilesTomlRepresentation, 0),
	}

	for _, mod := range p.Mods {
		entry, err := mod.toIndexEntry()
		if err != nil {
			return core.IndexTomlRepresentation{}, errors.New(fmt.Sprintf("failed to convert mod %s to index entry: %v", mod.Slug, err))
		}
		repr.Files = append(repr.Files, entry)
	}

	return repr, nil
}

func (p *Pack) getIndex() (core.PackTomlIndex, error) {
	_, hash, err := p.AsIndexToml()
	if err != nil {
		return core.PackTomlIndex{}, err
	}

	return core.PackTomlIndex{
		File:       "index.toml",
		HashFormat: "sha256",
		Hash:       hash,
	}, nil
}
