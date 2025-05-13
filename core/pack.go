package core

import (
	"errors"
	"fmt"
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
		PackFormat:  CurrentPackFormat,
		Versions:    versions,
	}
}

func FromPackAndModsMeta(packMeta PackToml, modMetas []*ModToml) *Pack {

	mods := make(map[string]*Mod)
	for _, modMeta := range modMetas {
		mods[modMeta.GetSlug()] = FromModMeta(*modMeta)
	}

	pack := &Pack{
		Name:        packMeta.Name,
		Author:      packMeta.Author,
		Version:     packMeta.Version,
		Description: packMeta.Description,
		PackFormat:  packMeta.PackFormat,
		Versions:    packMeta.Versions,
		Export:      packMeta.Export,
		Options:     packMeta.Options,
		Mods:        mods,
	}

	return pack
}

func (p *Pack) GetExportName() string {
	if p.Name == "" {
		return "export"
	}
	if p.Version == "" {
		return p.Name
	}
	return p.Name + "-" + p.Version
}

func (p *Pack) SetMod(mod *Mod) {
	if p.Mods == nil {
		p.Mods = make(map[string]*Mod)
	}

	p.Mods[mod.Slug] = mod
}

func (p *Pack) GetModsList() []*Mod {
	mods := make([]*Mod, 0, len(p.Mods))
	for _, mod := range p.Mods {
		mods = append(mods, mod)
	}
	return mods
}

func (p *Pack) UpdateAll() error {

	namedUpdaterMap := make(map[string][]*Mod)

	for _, mod := range p.Mods {
		updater, err := mod.GetUpdater()
		if err != nil {
			return err
		}

		namedUpdaterMap[updater.GetName()] = append(namedUpdaterMap[updater.GetName()], mod)
	}

	return nil
}

func (p *Pack) Update(modSlug string) error {
	mod, ok := p.Mods[modSlug]
	if !ok {
		return fmt.Errorf("mod %s not found", modSlug)
	}

	updater, err := mod.GetUpdater()
	if err != nil {
		return err
	}

	check, err := updater.CheckUpdate([]*Mod{mod}, *p)
	if err != nil {
		return err
	}
	if len(check) != 1 {
		return fmt.Errorf("expected 1 updater, got %d", len(check))
	}

	if check[0].UpdateAvailable {
		err = updater.DoUpdate([]*Mod{mod}, []interface{}{check[0].CachedState})
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Pack) ToPackMeta() (PackToml, error) {
	indexToml, err := p.getIndex()
	if err != nil {
		return PackToml{}, err
	}

	packToml := PackToml{
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

	return packToml, nil
}

func (p *Pack) AsPackToml() (string, error) {
	packToml, err := p.ToPackMeta()
	if err != nil {
		return "", err
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

	text, _, err := mod.AsModToml()
	return text, err
}

func (p *Pack) AsIndexMeta() (IndexFS, error) {
	repr, err := p.getIndexRepr()
	if err != nil {
		return IndexFS{}, err
	}

	index := NewIndexFromTomlRepr(repr)
	return index, nil
}

func (p *Pack) getIndexRepr() (IndexTomlRepresentation, error) {
	repr := IndexTomlRepresentation{
		DefaultModHashFormat: "sha256",
		Files:                make(IndexFilesTomlRepresentation, 0),
	}

	for _, mod := range p.Mods {
		entry, err := mod.toIndexEntry()
		if err != nil {
			return IndexTomlRepresentation{}, errors.New(fmt.Sprintf("failed to convert mod %s to index entry: %v", mod.Slug, err))
		}
		repr.Files = append(repr.Files, entry)
	}

	return repr, nil
}

func (p *Pack) getIndex() (PackTomlIndex, error) {
	_, hash, err := p.AsIndexToml()
	if err != nil {
		return PackTomlIndex{}, err
	}

	return PackTomlIndex{
		File:       "index.toml",
		HashFormat: "sha256",
		Hash:       hash,
	}, nil
}

// GetMCVersion gets the version of Minecraft this pack uses, if it has been correctly specified
func (p *Pack) GetMCVersion() (string, error) {
	mcVersion, ok := p.Versions["minecraft"]
	if !ok {
		return "", errors.New("no minecraft version specified in modpack")
	}
	return mcVersion, nil
}

// GetSupportedMCVersions gets the versions of Minecraft this pack allows in downloaded mods, ordered by preference (highest = most desirable)
func (p *Pack) GetSupportedMCVersions() ([]string, error) {
	mcVersion, err := p.GetMCVersion()
	if err != nil {
		return nil, err
	}
	allVersions := append(append([]string(nil), p.GetAcceptableGameVersions()...), mcVersion)
	SortAndDedupeVersions(allVersions)
	return allVersions, nil
}

func (p *Pack) GetAcceptableGameVersions() []string {
	acceptableVersions, ok := p.Options["acceptable-game-versions"]
	if !ok {
		return []string{}
	}
	return acceptableVersions.([]string)
}

func (p *Pack) SetAcceptableGameVersions(versions []string) {
	SortAndDedupeVersions(versions)
	p.Options["acceptable-game-versions"] = versions
}

func (p *Pack) GetCompatibleLoaders() (loaders []string) {
	if _, hasQuilt := p.Versions["quilt"]; hasQuilt {
		loaders = append(loaders, "quilt")
		loaders = append(loaders, "fabric") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasFabric := p.Versions["fabric"]; hasFabric {
		loaders = append(loaders, "fabric")
	}
	if _, hasNeoForge := p.Versions["neoforge"]; hasNeoForge {
		loaders = append(loaders, "neoforge")
		loaders = append(loaders, "forge") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasForge := p.Versions["forge"]; hasForge {
		loaders = append(loaders, "forge")
	}
	return
}
