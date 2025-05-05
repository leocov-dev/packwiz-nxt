package packwiz

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
)

type Mod struct {
	Name     string
	FileName string
	Side     string
	Pin      bool
	Download core.ModDownload
	Update   core.ModUpdate
	Option   *core.ModOption

	// for index
	Slug       string
	ModType    string // mods/shaders/resourcepacks/etc.
	MetaFile   bool
	HashFormat string
	Alias      string
	Preserve   bool
}

func NewMod(
	slug,
	name,
	fileName,
	side,
	modType,
	alias string,
	pin,
	metaFile,
	preserve bool,
	update core.ModUpdate,
	download core.ModDownload,
	options *core.ModOption,
) *Mod {
	return &Mod{
		Slug:     slug,
		Name:     name,
		FileName: fileName,
		Side:     side,
		Pin:      pin,
		ModType:  modType,
		MetaFile: metaFile,
		Alias:    alias,
		Preserve: preserve,
		Update:   update,
		Download: download,
		Option:   options,
	}
}

func (m *Mod) Serialize() (string, string, error) {
	modToml := core.ModToml{
		Name:     m.Name,
		FileName: m.FileName,
		Side:     m.Side,
		Pin:      m.Pin,
		Download: m.Download,
		Update:   m.Update,
		Option:   m.Option,
	}

	result, err := modToml.Marshal()
	if err != nil {
		return "", "", err
	}

	return result.String(), result.Hash, nil
}

func (m *Mod) toIndexEntry() (core.IndexFile, error) {

	_, hash, err := m.Serialize()
	if err != nil {
		return core.IndexFile{}, err
	}

	return core.IndexFile{
		File:       fmt.Sprintf("%s/%s.pw.toml", m.ModType, m.Slug),
		Hash:       hash,
		HashFormat: m.HashFormat,
		Alias:      m.Alias,
		MetaFile:   m.MetaFile,
		Preserve:   m.Preserve,
	}, nil
}
