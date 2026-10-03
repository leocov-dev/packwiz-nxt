package fileio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
)

const (
	// DefaultRemoteMaxFileSize caps the size of any single pack.toml, index.toml
	// or mod metadata file fetched by LoadRemotePack.
	DefaultRemoteMaxFileSize int64 = 4 << 20
	// DefaultRemoteMaxMetaFiles caps how many mod metadata files LoadRemotePack
	// will fetch for one pack.
	DefaultRemoteMaxMetaFiles = 5000

	remoteFetchConcurrency = 8
)

// RemoteLoadOptions tunes LoadRemotePack. The zero value is valid.
type RemoteLoadOptions struct {
	// Client performs the requests. Nil uses a client with core.DefaultHTTPTimeout.
	Client *http.Client
	// MaxFileSize is the per-file size cap in bytes. Zero uses DefaultRemoteMaxFileSize.
	MaxFileSize int64
	// MaxMetaFiles caps the number of mod metadata files. Zero uses DefaultRemoteMaxMetaFiles.
	MaxMetaFiles int
}

// RemotePack is a packwiz pack read from a live pack.toml URL.
type RemotePack struct {
	Pack *core.Pack
	// Warnings are non-fatal problems: skipped or unreadable mod files, hash
	// mismatches, pack format migrations.
	Warnings []string
	// SkippedFiles are index paths that are not mod metadata (configs, scripts,
	// overrides). They are listed but never downloaded.
	SkippedFiles []string
}

// LoadRemotePack reads a pack.toml from packURL, then the index it references,
// then every mod metadata file the index lists. The index and mod files must be
// served from the same scheme and host as the pack.toml.
//
// A pack.toml or index.toml that cannot be read is an error. A mod metadata file
// that cannot be read is skipped and reported in RemotePack.Warnings.
func LoadRemotePack(ctx context.Context, packURL string, opts RemoteLoadOptions) (*RemotePack, error) {
	f := newRemoteFetcher(opts)

	base, err := url.Parse(packURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("pack url must be an absolute http(s) url: %q", packURL)
	}

	result := &RemotePack{}

	raw, err := f.fetch(ctx, base)
	if err != nil {
		return nil, fmt.Errorf("fetch pack.toml: %w", err)
	}
	var packMeta core.PackToml
	if err := toml.Unmarshal(raw, &packMeta); err != nil {
		return nil, fmt.Errorf("parse pack.toml: %w", err)
	}
	_, warnings, err := core.ValidatePack(&packMeta)
	if err != nil {
		return nil, fmt.Errorf("validate pack.toml: %w", err)
	}
	result.Warnings = append(result.Warnings, warnings...)

	if packMeta.Index.File == "" {
		return nil, fmt.Errorf("pack.toml has no index file")
	}
	indexURL, err := f.resolve(base, packMeta.Index.File)
	if err != nil {
		return nil, fmt.Errorf("index file: %w", err)
	}
	rawIndex, err := f.fetch(ctx, indexURL)
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	if w := checkHash(packMeta.Index.HashFormat, packMeta.Index.Hash, rawIndex); w != "" {
		result.Warnings = append(result.Warnings, "index.toml "+w)
	}

	var index core.IndexTomlRepresentation
	if err := toml.Unmarshal(rawIndex, &index); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	if index.DefaultModHashFormat == "" {
		index.DefaultModHashFormat = core.DefaultHashFormat
	}

	entries, skipped := selectMetaEntries(index)
	result.SkippedFiles = skipped
	if max := f.maxMetaFiles; len(entries) > max {
		return nil, fmt.Errorf("index lists %d mod files, limit is %d", len(entries), max)
	}

	mods, modWarnings := f.fetchMods(ctx, indexURL, entries)
	result.Warnings = append(result.Warnings, modWarnings...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result.Pack = core.FromPackAndModsMeta(packMeta, mods.metas)
	for slug, e := range mods.entries {
		if m, ok := result.Pack.Mods[slug]; ok {
			m.Alias = e.Alias
			m.Preserve = e.Preserve
			m.HashFormat = e.HashFormat
		}
	}
	return result, nil
}

// selectMetaEntries splits index entries into mod metadata files (kept, sorted
// by path, de-duplicated across aliases) and everything else (returned as paths).
func selectMetaEntries(index core.IndexTomlRepresentation) (meta []core.IndexFile, skipped []string) {
	seen := map[string]bool{}
	for _, e := range index.Files {
		p := path.Clean(e.File)
		if seen[p] {
			continue
		}
		seen[p] = true
		e.File = p
		if e.MetaFile || strings.HasSuffix(p, core.MetaExtension) {
			if e.HashFormat == "" {
				e.HashFormat = index.DefaultModHashFormat
			}
			meta = append(meta, e)
		} else {
			skipped = append(skipped, p)
		}
	}
	sort.Slice(meta, func(i, j int) bool { return meta[i].File < meta[j].File })
	sort.Strings(skipped)
	return meta, skipped
}

type fetchedMods struct {
	metas   []*core.ModToml
	entries map[string]core.IndexFile // by slug
}

func (f *remoteFetcher) fetchMods(
	ctx context.Context,
	indexURL *url.URL,
	entries []core.IndexFile,
) (fetchedMods, []string) {
	type outcome struct {
		mod     *core.ModToml
		skipped string // set when the file was not usable
		note    string // set when the file was usable but suspect
	}
	outcomes := make([]outcome, len(entries))

	sem := make(chan struct{}, remoteFetchConcurrency)
	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, e core.IndexFile) {
			defer wg.Done()
			defer func() { <-sem }()
			mod, skipped, note := f.fetchMod(ctx, indexURL, e)
			outcomes[i] = outcome{mod, skipped, note}
		}(i, e)
	}
	wg.Wait()

	result := fetchedMods{entries: map[string]core.IndexFile{}}
	var warnings []string
	for i, o := range outcomes {
		if o.skipped != "" {
			warnings = append(warnings, o.skipped)
			continue
		}
		if o.note != "" {
			warnings = append(warnings, o.note)
		}
		if renamed := uniqueSlug(o.mod, result.entries); renamed {
			warnings = append(warnings, fmt.Sprintf("%s: slug in use, renamed to %q", entries[i].File, o.mod.GetSlug()))
		}
		slug := o.mod.GetSlug()
		result.metas = append(result.metas, o.mod)
		result.entries[slug] = entries[i]
	}
	return result, warnings
}

// uniqueSlug renames mod when its slug is already in taken: first by appending
// its folder (mods/foo + resourcepacks/foo -> foo-resourcepacks), then a counter.
func uniqueSlug(mod *core.ModToml, taken map[string]core.IndexFile) (renamed bool) {
	orig := mod.GetSlug()
	if _, used := taken[orig]; !used {
		return false
	}
	candidate := orig + "-" + mod.GetMetaFolder()
	for n := 2; ; n++ {
		if _, used := taken[candidate]; !used {
			mod.SetSlug(candidate)
			return true
		}
		candidate = fmt.Sprintf("%s-%s-%d", orig, mod.GetMetaFolder(), n)
	}
}

func (f *remoteFetcher) fetchMod(ctx context.Context, indexURL *url.URL, e core.IndexFile) (mod *core.ModToml, skipped, note string) {
	u, err := f.resolve(indexURL, e.File)
	if err != nil {
		return nil, fmt.Sprintf("%s: skipped, %v", e.File, err), ""
	}
	raw, err := f.fetch(ctx, u)
	if err != nil {
		return nil, fmt.Sprintf("%s: skipped, %v", e.File, err), ""
	}
	var parsed core.ModToml
	if err := toml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Sprintf("%s: skipped, parse: %v", e.File, err), ""
	}
	// ReflectUpdateData is deliberately not called: it needs a registered
	// updater per source, and the raw Update map is all an importer keeps.
	parsed.SetMetaPath(e.File)
	if parsed.GetSlug() == "" {
		return nil, fmt.Sprintf("%s: skipped, no slug", e.File), ""
	}
	if w := checkHash(e.HashFormat, e.Hash, raw); w != "" {
		// not fatal: hosts commonly serve a stale index
		note = fmt.Sprintf("%s: %s", e.File, w)
	}
	return &parsed, "", note
}

// checkHash returns a warning if want is set and does not match the hash of data.
func checkHash(format, want string, data []byte) string {
	if want == "" {
		return ""
	}
	if format == "" {
		format = core.DefaultHashFormat
	}
	h, err := core.GetHashImpl(format)
	if err != nil {
		return fmt.Sprintf("hash not verified: %v", err)
	}
	h.Write(data)
	if !strings.EqualFold(h.String(), want) {
		return "hash does not match the index"
	}
	return ""
}

type remoteFetcher struct {
	client       *http.Client
	maxFileSize  int64
	maxMetaFiles int
}

func newRemoteFetcher(opts RemoteLoadOptions) *remoteFetcher {
	f := &remoteFetcher{
		client:       opts.Client,
		maxFileSize:  opts.MaxFileSize,
		maxMetaFiles: opts.MaxMetaFiles,
	}
	if f.client == nil {
		f.client = &http.Client{Timeout: core.DefaultHTTPTimeout}
	}
	if f.maxFileSize <= 0 {
		f.maxFileSize = DefaultRemoteMaxFileSize
	}
	if f.maxMetaFiles <= 0 {
		f.maxMetaFiles = DefaultRemoteMaxMetaFiles
	}
	return f
}

// resolve resolves ref against base and requires the result to stay on base's
// scheme and host, so a pack cannot point the importer at other servers.
func (f *remoteFetcher) resolve(base *url.URL, ref string) (*url.URL, error) {
	var r *url.URL
	if strings.Contains(ref, "://") {
		parsed, err := url.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("bad url %q: %w", ref, err)
		}
		r = parsed
	} else {
		r = &url.URL{Path: ref}
	}
	u := base.ResolveReference(r)
	if u.Scheme != base.Scheme || u.Host != base.Host {
		return nil, fmt.Errorf("%q is not on the same host as the pack", ref)
	}
	return u, nil
}

func (f *remoteFetcher) fetch(ctx context.Context, u *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", core.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u.Path, resp.Status)
	}

	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(resp.Body, f.maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if n > f.maxFileSize {
		return nil, fmt.Errorf("GET %s: larger than %d bytes", u.Path, f.maxFileSize)
	}
	return buf.Bytes(), nil
}
