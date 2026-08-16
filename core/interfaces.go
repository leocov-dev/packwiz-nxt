package core

import (
	"io"
	"sync"
)

// Registry holds the set of Updaters and MetaDownloaders that packwiz can use,
// keyed by their configuration/source name. It is safe for concurrent use by
// multiple goroutines.
//
// A zero-value Registry is not usable; construct one with NewRegistry.
type Registry struct {
	mu              sync.RWMutex
	updaters        map[string]Updater
	metaDownloaders map[string]MetaDownloader
	logger          Logger
}

// NewRegistry creates an empty, ready-to-use Registry.
func NewRegistry() *Registry {
	return &Registry{
		updaters:        make(map[string]Updater),
		metaDownloaders: make(map[string]MetaDownloader),
		logger:          PrintLogger{},
	}
}

// SetLogger overrides the Registry's logger, used to report non-fatal
// warnings/progress during update resolution. Defaults to PrintLogger.
func (r *Registry) SetLogger(l Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = l
}

// Logger returns the Registry's current logger.
func (r *Registry) Logger() Logger {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.logger
}

// AddUpdater registers an Updater, keyed by its GetName() value.
func (r *Registry) AddUpdater(updater Updater) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updaters[updater.GetName()] = updater
}

// GetUpdater looks up an Updater previously registered with AddUpdater.
func (r *Registry) GetUpdater(name string) (Updater, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	updater, ok := r.updaters[name]
	return updater, ok
}

// AddMetaDownloader registers a MetaDownloader, keyed by source name.
func (r *Registry) AddMetaDownloader(source string, downloader MetaDownloader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metaDownloaders[source] = downloader
}

// GetMetaDownloader looks up a MetaDownloader previously registered with AddMetaDownloader.
func (r *Registry) GetMetaDownloader(source string) (MetaDownloader, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	downloader, ok := r.metaDownloaders[source]
	return downloader, ok
}

// DefaultRegistry is the process-wide Registry used by the packwiz CLI and by
// package-level helpers (AddUpdater, GetUpdater, AddMetaDownloader,
// GetMetaDownloader). sources/*-updater.go register themselves here via
// init(). Library consumers that want isolated/concurrent instances should
// construct their own Registry with NewRegistry instead of relying on this.
var DefaultRegistry = NewRegistry()

// AddUpdater registers an Updater on DefaultRegistry, keyed by its GetName() value.
func AddUpdater(updater Updater) {
	DefaultRegistry.AddUpdater(updater)
}

// GetUpdater looks up an Updater on DefaultRegistry.
func GetUpdater(name string) (Updater, bool) {
	return DefaultRegistry.GetUpdater(name)
}

// updaterFor finds the Updater registered on reg for one of update's keys, or false if
// none is registered. It backs Mod.GetUpdater and ModToml.GetUpdater, which both target
// DefaultRegistry - see their doc comments for why a non-default Registry isn't
// currently reachable there.
func updaterFor(update ModUpdate, reg *Registry) (Updater, bool) {
	for k := range update {
		if updater, ok := reg.GetUpdater(k); ok {
			return updater, true
		}
	}
	return nil, false
}

// AddMetaDownloader registers a MetaDownloader on DefaultRegistry, keyed by source name.
func AddMetaDownloader(source string, downloader MetaDownloader) {
	DefaultRegistry.AddMetaDownloader(source, downloader)
}

// GetMetaDownloader looks up a MetaDownloader on DefaultRegistry.
func GetMetaDownloader(source string) (MetaDownloader, bool) {
	return DefaultRegistry.GetMetaDownloader(source)
}

// Updater is used to process updates on mods
type Updater interface {
	GetName() string
	// ParseUpdate takes an unparsed any (as a map[string]any), and returns an Updater for a mod file.
	// This can be done using the mapstructure library or your own parsing methods.
	ParseUpdate(map[string]any) (any, error)
	// CheckUpdate checks whether there is an update for each of the mods in the given slice,
	// called for all of the mods that this updater handles
	CheckUpdate([]*Mod, Pack) ([]UpdateCheck, error)
	// DoUpdate carries out the update previously queried in CheckUpdate, on each ModToml's metadata,
	// given pointers to Mods and the value of CachedState for each mod
	DoUpdate([]*Mod, []any) error
}

// UpdateCheck represents the data returned from CheckUpdate for each mod
type UpdateCheck struct {
	// UpdateAvailable is true if an update is available for this mod
	UpdateAvailable bool
	// UpdateString is a string that details the update in some way to the user. Usually this will be in the form of
	// a version change (1.0.0 -> 1.0.1), or a file name change (thanos-skin-1.0.0.jar -> thanos-skin-1.0.1.jar).
	UpdateString string
	// CachedState can be used to preserve per-mod state between CheckUpdate and DoUpdate (e.g. file metadata)
	CachedState any
	// Error stores an error for this specific mod
	// Errors can also be returned from CheckUpdate directly, if the whole operation failed completely (so only 1 error is printed)
	// If an error is returned for a mod, or from CheckUpdate, DoUpdate is not called on that mod / at all
	Error error
}

// MetaDownloader specifies a downloader for a Mod using a "metadata:source" mode
// The calling code should handle caching and hash validation.
type MetaDownloader interface {
	GetFilesMetadata([]*Mod) ([]MetaDownloaderData, error)
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
