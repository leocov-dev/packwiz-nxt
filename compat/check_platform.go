package compat

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

// forgeJavaFML is the Forge language provider whose loaderVersion range is
// a range over the Forge version.
const forgeJavaFML = "javafml"

// isPlatformID reports ids that name the game, the runtime or a loader
// rather than a mod in the pack.
func isPlatformID(id string) bool {
	switch id {
	case jarmeta.IDMinecraft, jarmeta.IDJava, jarmeta.IDFabricLoader,
		jarmeta.IDQuiltLoader, jarmeta.IDForge, jarmeta.IDNeoForge:
		return true
	}
	return false
}

// evalAgainst evaluates a dependency's ranges against version. Errors caused
// by the version (not the range) are swallowed into "unknown".
func evalAgainst(r *resolved, dep jarmeta.Dependency, version string) (verdict, error) {
	v, err := evalDependencyRanges(r.matcher, dep, version)
	if err != nil && !errors.Is(err, ErrInvalidRange) {
		return verdictUnknown, nil
	}
	return v, err
}

func unevaluable(r *resolved, dep jarmeta.Dependency, err error) Finding {
	return Finding{
		Severity: SeverityInfo, Code: CodeUnevaluableRange, Slug: r.Slug,
		Message: fmt.Sprintf("version range for %q could not be evaluated: %v", dep.ID, err),
		Detail:  map[string]string{"id": dep.ID, "error": err.Error()},
	}
}

// platformDeps returns the mandatory "depends" relations on id.
func platformDeps(r *resolved, id string) []jarmeta.Dependency {
	var out []jarmeta.Dependency
	for _, d := range r.meta.Dependencies {
		if d.ID == id && d.Kind == jarmeta.KindDepends && d.Mandatory {
			out = append(out, d)
		}
	}
	return out
}

// checkMinecraftVersion reports a jar whose minecraft requirement excludes
// the pack Minecraft version. Skipped when the pack version is empty or not
// comparable, and for Maven ranges (Forge, NeoForge) when the pack version is
// a snapshot, pre-release or release candidate: Maven ordering has no sound
// answer for them.
func checkMinecraftVersion(c *checkContext, e *resolved) []Finding {
	mc := c.target.MinecraftVersion
	if mc == "" {
		return nil
	}
	if _, maven := e.matcher.(MavenMatcher); maven && !isMavenComparable(mc) {
		return nil
	}
	var out []Finding
	for _, dep := range platformDeps(e, jarmeta.IDMinecraft) {
		v, err := evalAgainst(e, dep, mc)
		switch {
		case err != nil:
			out = append(out, unevaluable(e, dep, err))
		case v == verdictNo:
			return append(out, Finding{
				Severity: SeverityError, Code: CodeMCVersionMismatch, Slug: e.Slug,
				Message: fmt.Sprintf("mod requires Minecraft %s but the pack uses %s", describeRanges(dep), mc),
				Detail:  map[string]string{"pack_version": mc, "required": describeRanges(dep)},
			})
		}
	}
	return out
}

// checkLoaderVersion reports a jar whose requirement on the loader excludes
// the pack loader version. Per metadata loader, the constraints are the
// dependency id on the loader and, for Forge mods using the "javafml"
// language provider, the file-level loaderVersion range. For other language
// providers (kotlinforforge, lowcodefml...) and for NeoForge the file-level
// range names the language provider version, not the loader version, and is
// ignored.
func checkLoaderVersion(c *checkContext, e *resolved) []Finding {
	lv := c.target.LoaderVersion
	if lv == "" || e.viaFabric || e.meta.Loader != c.target.Loader {
		return nil
	}
	deps := platformDeps(e, loaderDepID(e.meta.Loader))
	if e.meta.Loader == jarmeta.LoaderForge && e.meta.ModLoader == forgeJavaFML && e.meta.LoaderVersionRange != "" {
		deps = append(deps, jarmeta.Dependency{
			ID: jarmeta.IDForge, Kind: jarmeta.KindDepends, Mandatory: true,
			VersionRanges: []string{e.meta.LoaderVersionRange},
		})
	}
	var out []Finding
	for _, dep := range deps {
		v, err := evalAgainst(e, dep, lv)
		switch {
		case err != nil:
			out = append(out, unevaluable(e, dep, err))
		case v == verdictNo:
			return append(out, Finding{
				Severity: SeverityError, Code: CodeLoaderVersionMismatch, Slug: e.Slug,
				Message: fmt.Sprintf("mod requires %s %s but the pack uses %s", e.meta.Loader, describeRanges(dep), lv),
				Detail:  map[string]string{"loader": string(e.meta.Loader), "pack_version": lv, "required": describeRanges(dep)},
			})
		}
	}
	return out
}

var mcSnapshot = regexp.MustCompile(`^\d+w\d+[a-z]$`)

// isMavenComparable reports whether a Minecraft version can be ordered by
// Maven rules. Snapshots ("24w14a") and pre-releases ("1.21-pre1",
// "1.21-rc1") cannot: Maven would rank them above or inconsistently with
// the release they precede.
func isMavenComparable(mc string) bool {
	lower := strings.ToLower(mc)
	if mcSnapshot.MatchString(lower) {
		return false
	}
	for _, q := range []string{"-pre", "-rc", "-snapshot"} {
		if strings.Contains(lower, q) {
			return false
		}
	}
	return true
}

func loaderDepID(l jarmeta.Loader) string {
	switch l {
	case jarmeta.LoaderFabric:
		return jarmeta.IDFabricLoader
	case jarmeta.LoaderQuilt:
		return jarmeta.IDQuiltLoader
	case jarmeta.LoaderForge:
		return jarmeta.IDForge
	case jarmeta.LoaderNeoForge:
		return jarmeta.IDNeoForge
	}
	return ""
}

// describeRanges renders the declared ranges of a dependency for messages.
func describeRanges(dep jarmeta.Dependency) string {
	s := ""
	for i, r := range dep.VersionRanges {
		if i > 0 {
			s += " or "
		}
		s += r
	}
	for _, r := range dep.VersionRangesAll {
		if s != "" {
			s += " and "
		}
		s += r
	}
	if s == "" {
		return "any"
	}
	return s
}
