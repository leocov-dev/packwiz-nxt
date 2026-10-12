package compat

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

type depState int

const (
	depOK depState = iota
	depMissing
	depBadVersion
)

// depOutcome is the result of resolving one dependency (or one group member).
type depOutcome struct {
	state depState
	dep   jarmeta.Dependency
	// found lists the versions of the providers that were present, ordered
	// by provider slug.
	found []string
}

// resolveDependency looks up a single dependency among the pack providers.
// Unknown versions and unevaluable ranges count as satisfied; unevaluable
// ranges are returned as info findings.
func (c *checkContext) resolveDependency(e *resolved, dep jarmeta.Dependency) (depOutcome, []Finding) {
	out := depOutcome{dep: dep}
	provs := c.providers[dep.ID]
	if len(provs) == 0 {
		out.state = depMissing
		if c.fabricAPIModule(e, dep.ID) {
			out.state = depOK // bundled in the Fabric API; not verifiable
		}
		return out, nil
	}
	var extra []Finding
	for _, p := range provs {
		if !p.versionKnown {
			out.state = depOK
			return out, nil
		}
		v, err := evalAgainst(e, dep, p.version)
		if err != nil {
			extra = append(extra, unevaluable(e, dep, err))
			out.state = depOK
			return out, extra
		}
		if v != verdictNo {
			out.state = depOK
			return out, extra
		}
		out.found = append(out.found, p.version)
	}
	out.state = depBadVersion
	return out, extra
}

// Fabric API ids. Its modules ship as nested jars inside the API mod, so they
// are never providers of their own.
const (
	idFabricAPI = "fabric-api"
	idQuiltFAPI = "quilted_fabric_api"
)

// fabricModuleID matches the id of a Fabric API module: "fabric-api-base" or
// "fabric-<name>-v<N>" (such as "fabric-resource-loader-v0"). Unrelated mods
// whose id merely starts with "fabric-" (fabric-language-kotlin) do not match.
var fabricModuleID = regexp.MustCompile(`^fabric-api-base$|^fabric-.+-v\d+$`)

// fabricAPIModule reports whether id is a module of a Fabric API that is
// present in the pack, so the dependency is satisfied by a nested jar.
func (c *checkContext) fabricAPIModule(e *resolved, id string) bool {
	if e.meta.Loader != jarmeta.LoaderFabric && e.meta.Loader != jarmeta.LoaderQuilt {
		return false
	}
	if !fabricModuleID.MatchString(id) && !strings.HasPrefix(id, "quilted_fabric") {
		return false
	}
	return len(c.providers[idFabricAPI]) > 0 || len(c.providers[idQuiltFAPI]) > 0
}

// missingNote explains why a missing dependency may not really be missing:
// it may sit in a nested jar or in a jar that could not be read. Empty when
// neither applies.
func (c *checkContext) missingNote() string {
	var notes []string
	if len(c.nestedSlugs) > 0 {
		notes = append(notes, "may be bundled inside "+strings.Join(c.nestedSlugs, ", "))
	}
	if len(c.unreadable) > 0 {
		notes = append(notes, "unreadable jar present: "+strings.Join(c.unreadable, ", "))
	}
	return strings.Join(notes, "; ")
}

// ignorable reports dependencies that are not checked against pack entries.
func ignorable(e *resolved, dep jarmeta.Dependency) bool {
	return isPlatformID(dep.ID) || !sideApplies(dep, e.Side)
}

// checkDependencies reports mandatory dependencies that are missing or whose
// version is out of range. A Quilt alternative group is satisfied when any
// member is; otherwise one finding lists all alternatives.
func checkDependencies(c *checkContext, e *resolved) []Finding {
	var out []Finding
	for _, dep := range e.meta.Dependencies {
		if dep.Kind != jarmeta.KindDepends || !dep.Mandatory {
			continue
		}
		if len(dep.AnyOf) > 0 {
			out = append(out, c.checkGroup(e, dep)...)
			continue
		}
		if dep.ID == "" || ignorable(e, dep) {
			continue
		}
		res, extra := c.resolveDependency(e, dep)
		out = append(out, extra...)
		switch res.state {
		case depMissing:
			out = append(out, c.missingFinding(e, dep))
		case depBadVersion:
			out = append(out, versionFinding(e, res))
		}
	}
	return out
}

func (c *checkContext) checkGroup(e *resolved, group jarmeta.Dependency) []Finding {
	var extra []Finding
	var outcomes []depOutcome
	for _, m := range group.AnyOf {
		if m.ID == "" || ignorable(e, m) {
			return nil // a member we do not verify counts as satisfying the group
		}
		res, fs := c.resolveDependency(e, m)
		extra = append(extra, fs...)
		if res.state == depOK {
			return extra
		}
		outcomes = append(outcomes, res)
	}
	if len(outcomes) == 0 {
		return extra
	}
	alts := make([]string, 0, len(outcomes))
	anyPresent := false
	for _, o := range outcomes {
		alt := o.dep.ID
		if o.dep.ID != "" && describeRanges(o.dep) != "any" {
			alt += " " + describeRanges(o.dep)
		}
		alts = append(alts, alt)
		if o.state == depBadVersion {
			anyPresent = true
		}
	}
	code, what := CodeMissingDependency, "none of the alternative dependencies is present"
	if anyPresent {
		code, what = CodeDependencyVersion, "no alternative dependency has a matching version"
	}
	f := Finding{
		Severity: SeverityError, Code: code, Slug: e.Slug,
		Message: fmt.Sprintf("%s: one of %s is required", what, strings.Join(alts, ", ")),
		Detail:  map[string]string{"alternatives": strings.Join(alts, " | ")},
	}
	if code == CodeMissingDependency {
		f = c.softenMissing(f)
	}
	return append(extra, f)
}

func (c *checkContext) missingFinding(e *resolved, dep jarmeta.Dependency) Finding {
	f := Finding{
		Severity: SeverityError, Code: CodeMissingDependency, Slug: e.Slug,
		Message: fmt.Sprintf("required mod %q (%s) is not in the pack", dep.ID, describeRanges(dep)),
		Detail:  map[string]string{"dependency": dep.ID, "required": describeRanges(dep)},
	}
	return c.softenMissing(f)
}

// softenMissing downgrades a missing-dependency finding to a warning with an
// explanatory note when the provider may be hidden from the check.
func (c *checkContext) softenMissing(f Finding) Finding {
	if note := c.missingNote(); note != "" {
		f.Severity = SeverityWarning
		f.Message += " (" + note + ")"
		f.Detail["note"] = note
	}
	return f
}

func versionFinding(e *resolved, res depOutcome) Finding {
	found := strings.Join(res.found, ", ")
	return Finding{
		Severity: SeverityError, Code: CodeDependencyVersion, Slug: e.Slug,
		Message: fmt.Sprintf("required mod %q needs version %s but the pack has %s", res.dep.ID, describeRanges(res.dep), found),
		Detail:  map[string]string{"dependency": res.dep.ID, "required": describeRanges(res.dep), "found": found},
	}
}

// checkIncompatibilities reports other entries matched by a breaks (error)
// or conflicts (warning) declaration. A declaration without a range matches
// any version; an unknown version never matches. Alternative groups match if
// any member does.
func checkIncompatibilities(c *checkContext, e *resolved) []Finding {
	var out []Finding
	for _, dep := range e.meta.Dependencies {
		if dep.Kind != jarmeta.KindBreaks && dep.Kind != jarmeta.KindConflicts {
			continue
		}
		members := dep.AnyOf
		if len(members) == 0 {
			members = []jarmeta.Dependency{dep}
		}
		for _, m := range members {
			if m.ID == "" || isPlatformID(m.ID) || !sideApplies(m, e.Side) {
				continue
			}
			if f, ok := c.incompatibility(e, m); ok {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

func (c *checkContext) incompatibility(e *resolved, dep jarmeta.Dependency) (Finding, bool) {
	for _, p := range c.providers[dep.ID] {
		if p.entry == e {
			continue
		}
		if len(dep.VersionRanges) > 0 || len(dep.VersionRangesAll) > 0 {
			if !p.versionKnown {
				continue
			}
			if v, err := evalAgainst(e, dep, p.version); err != nil || v != verdictYes {
				continue
			}
		}
		sev, code, verb := SeverityError, CodeBreaks, "breaks"
		if dep.Kind == jarmeta.KindConflicts {
			sev, code, verb = SeverityWarning, CodeConflicts, "conflicts with"
		}
		return Finding{
			Severity: sev, Code: code, Slug: e.Slug,
			Message: fmt.Sprintf("mod %s %q (%s) from %q", verb, dep.ID, describeRanges(dep), p.entry.Slug),
			Detail: map[string]string{
				"mod_id": dep.ID, "declared": describeRanges(dep),
				"other_slug": p.entry.Slug, "other_version": p.version,
			},
		}, true
	}
	return Finding{}, false
}

// checkDuplicateIDs reports mod ids (declared or provided) offered by more
// than one distinct entry. One finding per involved slug, so each mod can
// link to its own problem. The same slug listed twice is one mod.
func checkDuplicateIDs(c *checkContext) []Finding {
	ids := make([]string, 0, len(c.providers))
	for id := range c.providers {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var out []Finding
	for _, id := range ids {
		slugs := make([]string, 0, len(c.providers[id]))
		for _, p := range c.providers[id] {
			if !p.bundled {
				slugs = append(slugs, p.entry.Slug)
			}
		}
		slices.Sort(slugs)
		slugs = slices.Compact(slugs)
		if len(slugs) < 2 {
			continue
		}
		for _, slug := range slugs {
			out = append(out, Finding{
				Severity: SeverityError, Code: CodeDuplicateModID, Slug: slug,
				Message: fmt.Sprintf("mod id %q is provided by multiple mods: %s", id, strings.Join(slugs, ", ")),
				Detail:  map[string]string{"mod_id": id, "slugs": strings.Join(slugs, ",")},
			})
		}
	}
	return out
}
