package core

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
)

type Mod struct {
	Name     string
	FileName string
	Side     ModSide
	Pin      bool
	Download ModDownload
	Update   ModUpdate
	Option   *ModOption

	// for index
	Slug       string
	ModType    string // mods, shaders, resourcepacks, etc.
	HashFormat string
	Alias      string
	Preserve   bool
}

func NewMod(
	slug,
	name,
	fileName string,
	side ModSide,
	modType,
	alias string,
	pin,
	preserve bool,
	update ModUpdate,
	download ModDownload,
	options *ModOption,
) *Mod {
	return &Mod{
		Slug:     slug,
		Name:     name,
		FileName: fileName,
		Side:     side,
		Pin:      pin,
		ModType:  modType,
		Alias:    alias,
		Preserve: preserve,
		Update:   update,
		Download: download,
		Option:   options,
	}
}

func FromModMeta(modMeta ModToml) *Mod {
	return &Mod{
		Name:       modMeta.Name,
		FileName:   modMeta.FileName,
		Side:       modMeta.Side,
		Pin:        modMeta.Pin,
		Download:   modMeta.Download,
		Update:     modMeta.Update,
		Option:     modMeta.Option,
		Slug:       modMeta.slug,
		ModType:    modMeta.metaFolder,
		HashFormat: modMeta.GetHashFormat(),
	}
}

func (m *Mod) GetMetaPath() string {
	return m.ModType + "/" + m.Slug + MetaExtension
}

func (m *Mod) AsModToml() (string, string, error) {
	modToml := m.ToModMeta()

	result, err := modToml.Marshal()
	if err != nil {
		return "", "", err
	}

	return result.String(), result.Hash, nil
}

func (m *Mod) toIndexEntry() (IndexFile, error) {

	_, hash, err := m.AsModToml()
	if err != nil {
		return IndexFile{}, err
	}

	return IndexFile{
		File:       m.GetMetaPath(),
		Hash:       hash,
		HashFormat: m.HashFormat,
		Alias:      m.Alias,
		MetaFile:   true,
		Preserve:   m.Preserve,
	}, nil
}

func (m *Mod) ToModMeta() ModToml {
	modToml := ModToml{
		Name:     m.Name,
		FileName: m.FileName,
		Side:     m.Side,
		Pin:      m.Pin,
		Download: m.Download,
		Update:   m.Update,
		Option:   m.Option,
	}
	modToml.SetMetaPath(m.GetMetaPath())

	return modToml
}

func (m *Mod) GetUpdater() (Updater, error) {
	for k := range m.Update {
		updater, ok := GetUpdater(k)
		if ok {
			return updater, nil
		}
	}
	return nil, fmt.Errorf("no updater found for mod: %s", m.Name)
}

func (m *Mod) DecodeNamedModSourceData(name string, target interface{}) error {

	rawMap, ok := m.Update[name]
	if !ok {
		return fmt.Errorf("no updater named: %s found for mod: %s", name, m.Name)
	}

	err := mapstructure.Decode(rawMap, target)
	return err
}
