package core

import (
	"errors"
	"github.com/pelletier/go-toml/v2"
	"path/filepath"
	"regexp"
	"strings"
)

// Mod stores metadata about a mod. This is written to a TOML file for each mod.
type Mod struct {
	metaFile string      // The file for the metadata file, used as an ID
	Name     string      `toml:"name"`
	FileName string      `toml:"filename"`
	Side     string      `toml:"side,omitempty"`
	Pin      bool        `toml:"pin,omitempty"`
	Download ModDownload `toml:"download"`
	// Update is a map of map of stuff, so you can store arbitrary values on string keys to define updating
	Update     map[string]map[string]interface{} `toml:"update"`
	updateData map[string]interface{}

	Option *ModOption `toml:"option,omitempty"`

	hash string
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

// The four possible values of Side (the side that the mod is on) are "server", "client", "both", and "" (equivalent to "both")
const (
	ServerSide    = "server"
	ClientSide    = "client"
	UniversalSide = "both"
	EmptySide     = ""
)

func (m *Mod) ReflectUpdateData() error {
	m.updateData = make(map[string]interface{})

	// Horrible reflection library to convert map[string]interface to proper struct
	for k, v := range m.Update {
		updater, ok := Updaters[k]
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

// SetMetaPath sets the file path of a metadata file
func (m *Mod) SetMetaPath(metaFile string) string {
	m.metaFile = metaFile
	return m.metaFile
}

// GetParsedUpdateData can be used to retrieve updater-specific information after parsing a mod file
func (m *Mod) GetParsedUpdateData(updaterName string) (interface{}, bool) {
	upd, ok := m.updateData[updaterName]
	return upd, ok
}

// GetFilePath is a clumsy hack that I made because Mod already stores it's path anyway
func (m *Mod) GetFilePath() string {
	return m.metaFile
}

// GetDestFilePath returns the path of the destination file of the mod
func (m *Mod) GetDestFilePath() string {
	return filepath.Join(filepath.Dir(m.metaFile), filepath.FromSlash(m.FileName))
}

// UpdateHash updates the hash of a mod file, used with ModWriter
func (m *Mod) UpdateHash(hashFormat string, hash string) {
	m.hash = hash
}

func (m *Mod) GetHashInfo() (string, string) {
	return m.GetHashFormat(), m.hash
}

func (m *Mod) IsMetaFile() bool {
	return true
}

func (m *Mod) AppendUpdateData(key string, value interface{}) {
	if m.updateData == nil {
		m.updateData = make(map[string]interface{})
	}
	m.updateData[key] = value
}

func (m *Mod) GetHashFormat() string {
	return "sha256"
}

func (m *Mod) Marshal() (MarshalResult, error) {
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

var slugifyRegex1 = regexp.MustCompile(`\(.*\)`)
var slugifyRegex2 = regexp.MustCompile(` - .+`)
var slugifyRegex3 = regexp.MustCompile(`[^a-z\d]`)
var slugifyRegex4 = regexp.MustCompile(`-+`)
var slugifyRegex5 = regexp.MustCompile(`^-|-$`)

func SlugifyName(name string) string {
	lower := strings.ToLower(name)
	noBrackets := slugifyRegex1.ReplaceAllString(lower, "")
	noSuffix := slugifyRegex2.ReplaceAllString(noBrackets, "")
	limitedChars := slugifyRegex3.ReplaceAllString(noSuffix, "-")
	noDuplicateDashes := slugifyRegex4.ReplaceAllString(limitedChars, "-")
	noLeadingTrailingDashes := slugifyRegex5.ReplaceAllString(noDuplicateDashes, "")
	return noLeadingTrailingDashes
}
