package compat

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

// Check compares the metadata of every entry against the pack target and
// against the other entries. It is pure and deterministic: findings are
// sorted by severity (errors first), slug, code and message. It never
// returns nil.
//
// Rules worth knowing:
//   - An empty Target.Loader skips all metadata checks.
//   - Quilt packs accept Fabric metadata when a jar has no Quilt metadata.
//     Loader version checks are skipped for such jars (Quilt and Fabric
//     loader versions are different numbering schemes).
//   - Only mandatory "depends" relations are reported. Optional, recommends
//     and suggests relations are ignored.
//   - A Forge/NeoForge dependency whose Side is CLIENT is ignored for entries
//     with side server, and SERVER for entries with side client.
//   - Dependencies on the platform ids (minecraft, java, fabricloader,
//     quilt_loader, forge, neoforge) are not matched against entries;
//     minecraft and the loader are checked against the Target instead.
//   - Entries that carry no usable metadata cannot provide mod ids, so a
//     dependency on a library shipped without metadata is reported missing.
//   - A version that is empty, a build placeholder or not comparable is
//     "unknown": no version finding is produced for it.
func Check(target Target, entries []Entry) []Finding {
	c := newContext(target, entries)
	for i := range c.entries {
		for _, chk := range entryChecks() {
			c.add(chk(c, &c.entries[i])...)
		}
	}
	for _, chk := range packChecks() {
		c.add(chk(c)...)
	}
	sortFindings(c.findings)
	return c.findings
}

// entryCheck inspects one resolved entry.
type entryCheck func(c *checkContext, e *resolved) []Finding

// packCheck inspects the pack as a whole.
type packCheck func(c *checkContext) []Finding

func entryChecks() []entryCheck {
	return []entryCheck{
		checkMinecraftVersion,
		checkLoaderVersion,
		checkDependencies,
		checkIncompatibilities,
		checkEnvironment,
	}
}

func packChecks() []packCheck {
	return []packCheck{checkDuplicateIDs}
}

// resolved is an entry with the LoaderMeta selected for the pack loader.
type resolved struct {
	Entry
	meta jarmeta.LoaderMeta
	// matcher evaluates the ranges written in meta.
	matcher RangeMatcher
	// viaFabric is true when a Fabric jar is used on a Quilt pack.
	viaFabric bool
}

// provider is one entry answering to a mod id.
type provider struct {
	entry   *resolved
	version string
	// versionKnown is false for empty or placeholder versions.
	versionKnown bool
}

type checkContext struct {
	target    Target
	entries   []resolved
	providers map[string][]provider
	findings  []Finding
}

func (c *checkContext) add(fs ...Finding) {
	c.findings = append(c.findings, fs...)
}

func newContext(target Target, entries []Entry) *checkContext {
	c := &checkContext{
		target:    target,
		providers: map[string][]provider{},
		findings:  []Finding{},
	}
	c.entries = make([]resolved, 0, len(entries))
	for _, en := range entries {
		r, fs := resolve(target, en)
		c.add(fs...)
		if r != nil {
			c.entries = append(c.entries, *r)
		}
	}
	for i := range c.entries {
		c.indexProviders(&c.entries[i])
	}
	return c
}

func (c *checkContext) indexProviders(r *resolved) {
	seen := map[string]bool{}
	for _, m := range r.meta.Mods {
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		c.providers[m.ID] = append(c.providers[m.ID], provider{
			entry: r, version: m.Version, versionKnown: usableVersion(m.Version, m.VersionIsPlaceholder),
		})
	}
	ver := usableVersion(r.meta.Version, r.meta.VersionIsPlaceholder)
	for _, id := range r.meta.Provides {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		c.providers[id] = append(c.providers[id], provider{entry: r, version: r.meta.Version, versionKnown: ver})
	}
}

func usableVersion(v string, placeholder bool) bool {
	return v != "" && !placeholder && !strings.Contains(v, "${")
}

// resolve selects the metadata matching the pack loader and reports the
// per-entry findings that stop an entry from being checked further.
func resolve(target Target, en Entry) (*resolved, []Finding) {
	var out []Finding
	if en.ParseErr != nil && !errors.Is(en.ParseErr, jarmeta.ErrNoMetadata) {
		out = append(out, Finding{
			Severity: SeverityWarning, Code: CodeParseError, Slug: en.Slug,
			Message: "mod jar could not be fully read: " + en.ParseErr.Error(),
			Detail:  map[string]string{"error": en.ParseErr.Error()},
		})
	}
	if len(en.Metas) == 0 {
		if en.ParseErr == nil || errors.Is(en.ParseErr, jarmeta.ErrNoMetadata) {
			out = append(out, Finding{
				Severity: SeverityInfo, Code: CodeNoMetadata, Slug: en.Slug,
				Message: "mod jar has no loader metadata (common for libraries)",
			})
		}
		return nil, out
	}

	if target.Loader == "" {
		return nil, out // no pack loader to judge against
	}
	meta, viaFabric, ok := selectMeta(target.Loader, en.Metas)
	if !ok {
		loaders := metaLoaders(en.Metas)
		out = append(out, Finding{
			Severity: SeverityError, Code: CodeWrongLoader, Slug: en.Slug,
			Message: fmt.Sprintf("mod has no %s metadata (jar supports: %s)", target.Loader, strings.Join(loaders, ", ")),
			Detail:  map[string]string{"pack_loader": string(target.Loader), "jar_loaders": strings.Join(loaders, ",")},
		})
		return nil, out
	}
	matcher, err := MatcherFor(meta.Loader)
	if err != nil {
		return nil, out
	}
	return &resolved{Entry: en, meta: meta, matcher: matcher, viaFabric: viaFabric}, out
}

func selectMeta(loader jarmeta.Loader, metas []jarmeta.LoaderMeta) (jarmeta.LoaderMeta, bool, bool) {
	for _, m := range metas {
		if m.Loader == loader {
			return m, false, true
		}
	}
	if loader == jarmeta.LoaderQuilt {
		for _, m := range metas {
			if m.Loader == jarmeta.LoaderFabric {
				return m, true, true
			}
		}
	}
	return jarmeta.LoaderMeta{}, false, false
}

func metaLoaders(metas []jarmeta.LoaderMeta) []string {
	var out []string
	for _, m := range metas {
		if s := string(m.Loader); !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

// sideApplies reports whether a Forge/NeoForge per-dependency side concerns
// an entry installed on the given pack side.
func sideApplies(dep jarmeta.Dependency, packSide core.ModSide) bool {
	switch dep.Side {
	case jarmeta.SideClient:
		return packSide != core.ServerSide
	case jarmeta.SideServer:
		return packSide != core.ClientSide
	}
	return true
}
