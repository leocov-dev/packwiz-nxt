package core

import (
	"io"
)

// updaters stores all the updaters that packwiz can use. Add your own update systems to this map, keyed by the configuration name.
var updaters = make(map[string]Updater)

func AddUpdater(updater Updater) {
	updaters[updater.GetName()] = updater
}

func GetUpdater(name string) (Updater, bool) {
	updater, ok := updaters[name]
	return updater, ok
}

// Updater is used to process updates on mods
type Updater interface {
	GetName() string
	// ParseUpdate takes an unparsed interface{} (as a map[string]interface{}), and returns an Updater for a mod file.
	// This can be done using the mapstructure library or your own parsing methods.
	ParseUpdate(map[string]interface{}) (interface{}, error)
	// CheckUpdate checks whether there is an update for each of the mods in the given slice,
	// called for all of the mods that this updater handles
	CheckUpdate([]*Mod, Pack) ([]UpdateCheck, error)
	// DoUpdate carries out the update previously queried in CheckUpdate, on each ModToml's metadata,
	// given pointers to Mods and the value of CachedState for each mod
	DoUpdate([]*Mod, []interface{}) error
}

// UpdateCheck represents the data returned from CheckUpdate for each mod
type UpdateCheck struct {
	// UpdateAvailable is true if an update is available for this mod
	UpdateAvailable bool
	// UpdateString is a string that details the update in some way to the user. Usually this will be in the form of
	// a version change (1.0.0 -> 1.0.1), or a file name change (thanos-skin-1.0.0.jar -> thanos-skin-1.0.1.jar).
	UpdateString string
	// CachedState can be used to preserve per-mod state between CheckUpdate and DoUpdate (e.g. file metadata)
	CachedState interface{}
	// Error stores an error for this specific mod
	// Errors can also be returned from CheckUpdate directly, if the whole operation failed completely (so only 1 error is printed)
	// If an error is returned for a mod, or from CheckUpdate, DoUpdate is not called on that mod / at all
	Error error
}

// MetaDownloaders stores all the metadata-based installers that packwiz can use. Add your own downloaders to this map, keyed by the source name.
var MetaDownloaders = make(map[string]MetaDownloader)

// MetaDownloader specifies a downloader for a ModToml using a "metadata:source" mode
// The calling code should handle caching and hash validation.
type MetaDownloader interface {
	GetFilesMetadata([]*ModToml) ([]MetaDownloaderData, error)
}

// MetaDownloaderData specifies the per-ModToml metadata retrieved for downloading
type MetaDownloaderData interface {
	GetManualDownload() (bool, ManualDownload)
	DownloadFile() (io.ReadCloser, error)
}

type ManualDownload struct {
	Name     string
	FileName string
	URL      string
}

type MarshalResult struct {
	Value      []byte
	HashFormat string
	Hash       string
}

func (m MarshalResult) String() string {
	return string(m.Value)
}

type HashableObject interface {
	Marshal() (MarshalResult, error)
}
