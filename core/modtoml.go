package core

import (
	"errors"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"path/filepath"
	"strings"
)

type ModSourceData map[string]interface{}
type ModUpdate map[string]ModSourceData

// ModToml stores metadata about a mod. This is written to a TOML file for each mod.
type ModToml struct {
	metaFile string      // The file for the metadata file, used as an ID
	Name     string      `toml:"name"`
	FileName string      `toml:"filename"`
	Side     ModSide     `toml:"side,omitempty"`
	Pin      bool        `toml:"pin,omitempty"`
	Download ModDownload `toml:"download"`
	// Update is a map of maps, of stuff, so you can store arbitrary values on
	// string keys to define updating
	Update     ModUpdate `toml:"update"`
	updateData map[string]interface{}

	Option *ModOption `toml:"option,omitempty"`

	hash       string
	slug       string
	metaFolder string
	alias      string
	preserve   bool
}

const (
	ModeURL string = "url"
	ModeCF  string = "metadata:curseforge"
)

// ModDownload specifies how to download the mod file
type ModDownload struct {
	URL        string `toml:"url,omitempty"`
	HashFormat string `toml:"hash-format"`
	Hash       string `toml:"hash"`
	// Mode defaults to modeURL (i.e. use URL when omitted or empty)
	Mode string `toml:"mode,omitempty"`
}

// ModOption specifies optional metadata for this mod file
type ModOption struct {
	Optional    bool   `toml:"optional"`
	Description string `toml:"description,omitempty"`
	Default     bool   `toml:"default,omitempty"`
}

type ModSide string

// The four possible values of Side (the side that the mod is on) are "server", "client", "both", and "" (equivalent to "both")
const (
	ServerSide    ModSide = "server"
	ClientSide    ModSide = "client"
	UniversalSide ModSide = "both"
	EmptySide     ModSide = ""
)

func (m *ModToml) ReflectUpdateData() error {
	m.updateData = make(map[string]interface{})

	// Horrible reflection library to convert map[string]interface to proper struct
	for k, v := range m.Update {
		updater, ok := GetUpdater(k)
		if ok {
			updateData, err := updater.ParseUpdate(v)
			if err != nil {
				return err
			}
			m.updateData[k] = updateData
		} else {
			return errors.New("Update plugin " + k + " not found!")
		}
	}

	return nil
}

func (m *ModToml) GetSlug() string {
	return m.slug
}

func (m *ModToml) SetSlug(value string) {
	m.slug = value
}

func (m *ModToml) GetMetaFolder() string {
	return m.metaFolder
}

func (m *ModToml) SetMetaFolder(value string) {
	m.metaFolder = value
}

func (m *ModToml) GetMetaRelativePath() string {
	return fmt.Sprintf("%s/%s%s", m.metaFolder, m.slug, MetaExtension)
}

// SetMetaPath sets the file path of a metadata file
func (m *ModToml) SetMetaPath(metaFile string) string {
	m.metaFile = metaFile
	m.SetMetaFolder(filepath.Base(filepath.Dir(metaFile)))

	filename := filepath.Base(metaFile)
	dotIndex := strings.Index(filename, ".")
	if dotIndex == -1 {
		m.SetSlug(filename)
	}
	m.SetSlug(filename[:dotIndex])

	return m.metaFile
}

func (m *ModToml) GetUpdater() (Updater, error) {
	for k := range m.Update {
		updater, ok := GetUpdater(k)
		if ok {
			return updater, nil
		}
	}
	return nil, fmt.Errorf("no updater found for mod: %s", m.Name)
}

// GetParsedUpdateData can be used to retrieve updater-specific information after parsing a mod file
func (m *ModToml) GetParsedUpdateData(updaterName string) (interface{}, bool) {
	upd, ok := m.updateData[updaterName]
	return upd, ok
}

// GetFilePath is a clumsy hack that I made because ModToml already stores it's path anyway
func (m *ModToml) GetFilePath() string {
	return m.metaFile
}

// GetDestFilePath returns the path of the destination file of the mod
func (m *ModToml) GetDestFilePath() string {
	return filepath.Join(filepath.Dir(m.metaFile), filepath.FromSlash(m.FileName))
}

// UpdateHash updates the hash of a mod file, used with ModWriter
func (m *ModToml) UpdateHash(_ string, hash string) {
	m.hash = hash
}

func (m *ModToml) GetHashInfo() (string, string) {
	return m.GetHashFormat(), m.hash
}

func (m *ModToml) IsMetaFile() bool {
	return true
}

func (m *ModToml) AppendUpdateData(key string, value interface{}) {
	if m.updateData == nil {
		m.updateData = make(map[string]interface{})
	}
	m.updateData[key] = value
}

func (m *ModToml) GetHashFormat() string {
	return "sha256"
}

func (m *ModToml) Marshal() (MarshalResult, error) {
	result := MarshalResult{
		HashFormat: m.GetHashFormat(),
	}

	var err error

	result.Value, err = toml.Marshal(m)
	if err != nil {
		return result, err
	}

	stringer, err := GetHashImpl(result.HashFormat)
	if err != nil {
		return result, err
	}

	if _, err := stringer.Write(result.Value); err != nil {
		return result, err
	}

	result.Hash = stringer.String()

	m.UpdateHash(result.HashFormat, result.Hash)

	return result, nil
}
