package compat

import (
	"cmp"
	"fmt"
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
	// found lists the versions of the providers that were present.
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
			out = append(out, missingFinding(e, dep))
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
	return append(extra, Finding{
		Severity: SeverityError, Code: code, Slug: e.Slug,
		Message: fmt.Sprintf("%s: one of %s is required", what, strings.Join(alts, ", ")),
		Detail:  map[string]string{"alternatives": strings.Join(alts, " | ")},
	})
}

func missingFinding(e *resolved, dep jarmeta.Dependency) Finding {
	return Finding{
		Severity: SeverityError, Code: CodeMissingDependency, Slug: e.Slug,
		Message: fmt.Sprintf("required mod %q (%s) is not in the pack", dep.ID, describeRanges(dep)),
		Detail:  map[string]string{"dependency": dep.ID, "required": describeRanges(dep)},
	}
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

// checkDuplicateIDs reports mod ids declared by more than one entry. One
// finding per involved entry, so each mod can link to its own problem.
func checkDuplicateIDs(c *checkContext) []Finding {
	byID := map[string][]*resolved{}
	for i := range c.entries {
		e := &c.entries[i]
		seen := map[string]bool{}
		for _, m := range e.meta.Mods {
			if m.ID != "" && !seen[m.ID] {
				seen[m.ID] = true
				byID[m.ID] = append(byID[m.ID], e)
			}
		}
	}
	ids := make([]string, 0, len(byID))
	for id, es := range byID {
		if len(es) > 1 {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	var out []Finding
	for _, id := range ids {
		es := byID[id]
		slugs := make([]string, len(es))
		for i, e := range es {
			slugs[i] = e.Slug
		}
		slices.SortFunc(es, func(a, b *resolved) int { return cmp.Compare(a.Slug, b.Slug) })
		slices.Sort(slugs)
		for _, e := range es {
			out = append(out, Finding{
				Severity: SeverityError, Code: CodeDuplicateModID, Slug: e.Slug,
				Message: fmt.Sprintf("mod id %q is provided by multiple mods: %s", id, strings.Join(slugs, ", ")),
				Detail:  map[string]string{"mod_id": id, "slugs": strings.Join(slugs, ",")},
			})
		}
	}
	return out
}
