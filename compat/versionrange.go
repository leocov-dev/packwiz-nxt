// Package compat checks parsed mod jar metadata (see package jarmeta) against
// a pack target and against the other mods of the pack. It is pure: no
// network, no filesystem, no printing, no global state. Findings are
// informational and never describe a change to the pack.
package compat

import (
	"errors"
	"fmt"

	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

var (
	// ErrInvalidRange is returned when a version range expression cannot be parsed.
	ErrInvalidRange = errors.New("invalid version range")
	// ErrInvalidVersion is returned when a version cannot be compared by the
	// range scheme in use (for example a Minecraft snapshot against a semver
	// comparator).
	ErrInvalidVersion = errors.New("invalid version")
	// ErrUnsupportedLoader is returned when no range scheme exists for a loader.
	ErrUnsupportedLoader = errors.New("unsupported loader")
)

// RangeMatcher evaluates one version range expression of a single scheme.
// Adding a scheme means adding an implementation and mapping loaders to it in
// MatcherFor.
type RangeMatcher interface {
	// Satisfies reports whether version lies inside rangeExpr. Errors wrap
	// ErrInvalidRange or ErrInvalidVersion, in which case the bool is false
	// and carries no meaning.
	Satisfies(rangeExpr, version string) (bool, error)
}

// MatcherFor returns the range scheme used by a loader's metadata: Fabric
// style predicates for Fabric and Quilt, Maven intervals for Forge and
// NeoForge.
func MatcherFor(loader jarmeta.Loader) (RangeMatcher, error) {
	switch loader {
	case jarmeta.LoaderFabric, jarmeta.LoaderQuilt:
		return FabricMatcher{}, nil
	case jarmeta.LoaderForge, jarmeta.LoaderNeoForge:
		return MavenMatcher{}, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupportedLoader, loader)
}

// Satisfies evaluates rangeExpr against version using the range scheme of loader.
func Satisfies(loader jarmeta.Loader, rangeExpr, version string) (bool, error) {
	m, err := MatcherFor(loader)
	if err != nil {
		return false, err
	}
	return m.Satisfies(rangeExpr, version)
}

// verdict is a three-valued evaluation result: a range check that cannot be
// evaluated must never turn into a false error.
type verdict int

const (
	verdictUnknown verdict = iota
	verdictYes
	verdictNo
)

// evalAny is the OR of the ranges: yes if any matches, no if all fail to
// match, unknown if none matches and at least one could not be evaluated.
// An empty list is yes (no range declared).
func evalAny(m RangeMatcher, ranges []string, version string) (verdict, error) {
	if len(ranges) == 0 {
		return verdictYes, nil
	}
	var firstErr error
	for _, r := range ranges {
		ok, err := m.Satisfies(r, version)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if ok {
			return verdictYes, nil
		}
	}
	if firstErr != nil {
		return verdictUnknown, firstErr
	}
	return verdictNo, nil
}

// evalAll is the AND of the ranges: no if any fails to match, unknown if
// none fails and at least one could not be evaluated. An empty list is yes.
func evalAll(m RangeMatcher, ranges []string, version string) (verdict, error) {
	var firstErr error
	for _, r := range ranges {
		ok, err := m.Satisfies(r, version)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !ok {
			return verdictNo, nil
		}
	}
	if firstErr != nil {
		return verdictUnknown, firstErr
	}
	return verdictYes, nil
}

// evalDependencyRanges combines the any-of and all-of ranges of a dependency.
func evalDependencyRanges(m RangeMatcher, dep jarmeta.Dependency, version string) (verdict, error) {
	a, errA := evalAny(m, dep.VersionRanges, version)
	b, errB := evalAll(m, dep.VersionRangesAll, version)
	switch {
	case a == verdictNo || b == verdictNo:
		return verdictNo, nil
	case a == verdictUnknown:
		return verdictUnknown, errA
	case b == verdictUnknown:
		return verdictUnknown, errB
	}
	return verdictYes, nil
}
