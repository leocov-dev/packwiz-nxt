package compat

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// FabricMatcher evaluates Fabric and Quilt version predicates:
//
//   - "*" or empty: any version
//   - "1.2.3": exact (missing components count as zero)
//   - ">=", "<=", ">", "<", "=": comparators
//   - "^1.2": at least 1.2, below the next major
//   - "~1.2": at least 1.2, below the next minor ("~1" is below the next major)
//   - "1.2.x", "1.2.*", "1.x": component wildcards
//   - space separated terms are ANDed; "||" separates alternatives
//
// Versions are compared as SemVer, so "1.21-pre1" sorts below "1.21". A
// version that is not SemVer (a snapshot such as "24w14a") can only be
// compared for exact string equality; any other predicate yields
// ErrInvalidVersion. Build metadata ("+build.1") is ignored. This is not
// FlexVer, which Fabric uses only for display ordering of non-semver mods.
type FabricMatcher struct{}

// Satisfies implements RangeMatcher.
func (FabricMatcher) Satisfies(rangeExpr, version string) (bool, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return false, fmt.Errorf("%w: empty version", ErrInvalidVersion)
	}
	var firstErr error
	for _, alt := range strings.Split(rangeExpr, "||") {
		ok, err := fabricConjunction(alt, version)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if ok {
			return true, nil
		}
	}
	return false, firstErr
}

// fabricTerms splits a conjunction into terms, joining a bare operator with
// the token that follows it (">= 1.2").
func fabricTerms(expr string) []string {
	var terms []string
	for _, f := range strings.Fields(expr) {
		if n := len(terms); n > 0 && isBareOperator(terms[n-1]) {
			terms[n-1] += f
			continue
		}
		terms = append(terms, f)
	}
	return terms
}

func isBareOperator(s string) bool {
	switch s {
	case ">=", "<=", ">", "<", "=", "^", "~":
		return true
	}
	return false
}

func fabricConjunction(expr, version string) (bool, error) {
	terms := fabricTerms(expr)
	if len(terms) == 0 {
		return true, nil
	}
	var firstErr error
	result := true
	for _, t := range terms {
		ok, err := fabricTerm(t, version)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !ok {
			result = false
		}
	}
	if !result {
		return false, nil
	}
	return true, firstErr
}

func fabricTerm(term, version string) (bool, error) {
	if term == "*" {
		return true, nil
	}
	op, rest := splitOperator(term)
	if rest == "" || rest == "-" {
		return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
	}

	if op == "" || op == "=" {
		if wild, ok, err := wildcardPrefix(rest); err != nil {
			return false, err
		} else if ok {
			return matchWildcard(term, wild, version)
		}
	}

	v, verr := parseSemver(version)

	switch op {
	case "", "=":
		b, berr := parseBound(rest)
		if berr != nil {
			return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
		}
		if verr != nil || b.v == nil {
			// Not both SemVer: only literal equality is decidable.
			return version == rest, nil
		}
		return v.Compare(b.v) == 0, nil
	case "^", "~":
		b, berr := parseBound(rest)
		if berr != nil || b.v == nil {
			return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
		}
		if verr != nil {
			return false, fmt.Errorf("%w: %q", ErrInvalidVersion, version)
		}
		upper := upperBound(op, b)
		return v.Compare(lowerBound(b)) >= 0 && v.Compare(upper) < 0, nil
	case ">=", "<=", ">", "<":
		b, berr := parseBound(rest)
		if berr != nil || b.v == nil {
			return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
		}
		if verr != nil {
			return false, fmt.Errorf("%w: %q", ErrInvalidVersion, version)
		}
		lo := lowerBound(b)
		c := v.Compare(lo)
		switch op {
		case ">=":
			return c >= 0, nil
		case ">":
			if b.openPre {
				return c >= 0, nil
			}
			return c > 0, nil
		case "<=":
			return c <= 0, nil
		default:
			return c < 0, nil
		}
	}
	return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
}

func splitOperator(term string) (op, rest string) {
	for _, o := range []string{">=", "<=", ">", "<", "=", "^", "~"} {
		if strings.HasPrefix(term, o) {
			return o, term[len(o):]
		}
	}
	return "", term
}

// bound is a parsed version operand. v is nil when the text is not SemVer.
type bound struct {
	v *semver.Version
	// parts is the number of numeric components written ("1.2" is 2).
	parts int
	// openPre is true for a trailing "-" ("1.20-"), meaning "including all
	// prereleases of 1.20".
	openPre bool
}

func parseBound(s string) (bound, error) {
	var b bound
	if strings.HasSuffix(s, "-") {
		b.openPre = true
		s = strings.TrimSuffix(s, "-")
	}
	if s == "" {
		return b, ErrInvalidRange
	}
	core := s
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		core = core[:i]
	}
	b.parts = strings.Count(core, ".") + 1
	v, err := parseSemver(s)
	if err != nil {
		return b, nil // non-semver operand: v stays nil
	}
	b.v = v
	return b, nil
}

func parseSemver(s string) (*semver.Version, error) {
	return semver.NewVersion(s)
}

// lowerBound returns the version to compare against; an open-prerelease
// bound is lowered to the smallest prerelease of that version.
func lowerBound(b bound) *semver.Version {
	if !b.openPre {
		return b.v
	}
	pre, err := b.v.SetPrerelease("0")
	if err != nil {
		return b.v
	}
	return &pre
}

func upperBound(op string, b bound) *semver.Version {
	major, minor := b.v.Major(), b.v.Minor()
	if op == "^" || b.parts == 1 {
		return semver.New(major+1, 0, 0, "", "")
	}
	return semver.New(major, minor+1, 0, "", "")
}

// wildcardPrefix reports whether s is a wildcard like "1.2.x" and returns its
// leading numeric components.
func wildcardPrefix(s string) ([]string, bool, error) {
	parts := strings.Split(s, ".")
	for i, p := range parts {
		if p == "x" || p == "X" || p == "*" {
			for _, q := range parts[i+1:] {
				if q != "x" && q != "X" && q != "*" {
					return nil, false, fmt.Errorf("%w: %q", ErrInvalidRange, s)
				}
			}
			return parts[:i], true, nil
		}
	}
	return nil, false, nil
}

func matchWildcard(term string, prefix []string, version string) (bool, error) {
	if len(prefix) == 0 {
		return true, nil
	}
	if len(prefix) > 2 {
		return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
	}
	want := make([]uint64, len(prefix))
	for i, p := range prefix {
		var n uint64
		if p == "" {
			return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false, fmt.Errorf("%w: %q", ErrInvalidRange, term)
			}
			n = n*10 + uint64(c-'0')
		}
		want[i] = n
	}
	v, err := parseSemver(version)
	if err != nil {
		return false, fmt.Errorf("%w: %q", ErrInvalidVersion, version)
	}
	got := []uint64{v.Major(), v.Minor()}
	for i := range want {
		if got[i] != want[i] {
			return false, nil
		}
	}
	return true, nil
}
