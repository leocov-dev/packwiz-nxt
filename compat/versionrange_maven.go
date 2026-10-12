package compat

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MavenMatcher evaluates Maven version ranges as used by Forge and NeoForge:
//
//   - "[1.0,2.0)", "(1.0,]", "[1.0,)", "(,1.0]": intervals; either bound may be omitted
//   - "[1.0]": exact
//   - "[1.0,2.0),[3.0,)": comma separated union of intervals
//   - empty or "*": any version
//   - a bare version such as "1.0": a soft requirement, treated as satisfied
//     by any version (Forge does not enforce it)
//
// Versions are compared with a numeric-aware Maven-like ordering: numeric
// components compare as numbers, missing components count as zero, and
// qualifiers order alpha < beta < milestone < rc < snapshot < release < sp
// (so "1.0-beta" < "1.0"). core.CompareVersions is not used because it
// ignores qualifiers.
type MavenMatcher struct{}

// Satisfies implements RangeMatcher.
func (MavenMatcher) Satisfies(rangeExpr, version string) (bool, error) {
	expr := strings.TrimSpace(rangeExpr)
	if expr == "" || expr == "*" {
		return true, nil
	}
	if expr[0] != '[' && expr[0] != '(' {
		if strings.ContainsAny(expr, "[]()") {
			return false, fmt.Errorf("%w: %q", ErrInvalidRange, rangeExpr)
		}
		return true, nil // soft requirement
	}
	intervals, err := parseMavenRange(expr)
	if err != nil {
		return false, fmt.Errorf("%w: %q: %w", ErrInvalidRange, rangeExpr, err)
	}
	version = strings.TrimSpace(version)
	vt, ok := tokenizeVersion(version)
	if !ok {
		return false, fmt.Errorf("%w: %q", ErrInvalidVersion, version)
	}
	for _, iv := range intervals {
		if iv.contains(vt) {
			return true, nil
		}
	}
	return false, nil
}

type mavenInterval struct {
	lo, hi       []versionToken
	hasLo, hasHi bool
	loIncl       bool
	hiIncl       bool
}

func (iv mavenInterval) contains(v []versionToken) bool {
	if iv.hasLo {
		c := compareTokens(v, iv.lo)
		if c < 0 || (c == 0 && !iv.loIncl) {
			return false
		}
	}
	if iv.hasHi {
		c := compareTokens(v, iv.hi)
		if c > 0 || (c == 0 && !iv.hiIncl) {
			return false
		}
	}
	return true
}

func parseMavenRange(expr string) ([]mavenInterval, error) {
	var out []mavenInterval
	rest := strings.TrimSpace(expr)
	for rest != "" {
		end := strings.IndexAny(rest, "])")
		if end < 0 || (rest[0] != '[' && rest[0] != '(') {
			return nil, errors.New("malformed interval")
		}
		iv, err := parseInterval(rest[:end+1])
		if err != nil {
			return nil, err
		}
		out = append(out, iv)
		rest = strings.TrimSpace(rest[end+1:])
		if rest == "" {
			break
		}
		if rest[0] != ',' {
			return nil, errors.New("expected ',' between intervals")
		}
		rest = strings.TrimSpace(rest[1:])
		if rest == "" {
			return nil, errors.New("trailing ','")
		}
	}
	return out, nil
}

func parseInterval(s string) (mavenInterval, error) {
	var iv mavenInterval
	iv.loIncl = s[0] == '['
	iv.hiIncl = s[len(s)-1] == ']'
	body := s[1 : len(s)-1]
	lo, hi, isRange := strings.Cut(body, ",")
	lo, hi = strings.TrimSpace(lo), strings.TrimSpace(hi)
	if !isRange {
		if !iv.loIncl || !iv.hiIncl || lo == "" {
			return iv, errors.New("single version needs [ ]")
		}
		t, ok := tokenizeVersion(lo)
		if !ok {
			return iv, fmt.Errorf("bad version %q", lo)
		}
		iv.lo, iv.hi, iv.hasLo, iv.hasHi = t, t, true, true
		return iv, nil
	}
	if strings.Contains(hi, ",") {
		return iv, errors.New("too many bounds")
	}
	if lo != "" {
		t, ok := tokenizeVersion(lo)
		if !ok {
			return iv, fmt.Errorf("bad version %q", lo)
		}
		iv.lo, iv.hasLo = t, true
	}
	if hi != "" {
		t, ok := tokenizeVersion(hi)
		if !ok {
			return iv, fmt.Errorf("bad version %q", hi)
		}
		iv.hi, iv.hasHi = t, true
	}
	return iv, nil
}

// versionToken is one numeric or qualifier component of a version.
type versionToken struct {
	numeric bool
	num     uint64
	qual    string
}

func isASCIIDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isASCIILetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

// tokenizeVersion splits a version on '.', '-', '_', '+' and on digit/letter
// transitions. It rejects empty input and characters that cannot appear in a
// version.
func tokenizeVersion(s string) ([]versionToken, bool) {
	if s == "" {
		return nil, false
	}
	var toks []versionToken
	var cur []rune
	curNum := false
	flush := func() bool {
		if len(cur) == 0 {
			return true
		}
		if curNum {
			n, err := strconv.ParseUint(string(cur), 10, 64)
			if err != nil {
				return false
			}
			toks = append(toks, versionToken{numeric: true, num: n})
		} else {
			toks = append(toks, versionToken{qual: strings.ToLower(string(cur))})
		}
		cur = cur[:0]
		return true
	}
	for _, r := range s {
		switch {
		case r == '.' || r == '-' || r == '_' || r == '+':
			if !flush() {
				return nil, false
			}
		case isASCIIDigit(r):
			if len(cur) > 0 && !curNum && !flush() {
				return nil, false
			}
			curNum = true
			cur = append(cur, r)
		case isASCIILetter(r):
			if len(cur) > 0 && curNum && !flush() {
				return nil, false
			}
			curNum = false
			cur = append(cur, r)
		default:
			return nil, false
		}
	}
	if !flush() || len(toks) == 0 {
		return nil, false
	}
	return toks, true
}

// Qualifier ranks; unknown qualifiers sort after sp, lexically among themselves.
const (
	rankAlpha = iota + 1
	rankBeta
	rankMilestone
	rankRC
	rankSnapshot
	rankRelease
	rankSP
	rankUnknown
)

func qualifierRank(q string) int {
	switch q {
	case "alpha", "a":
		return rankAlpha
	case "beta", "b":
		return rankBeta
	case "milestone", "m":
		return rankMilestone
	case "rc", "cr":
		return rankRC
	case "snapshot":
		return rankSnapshot
	case "", "ga", "final", "release":
		return rankRelease
	case "sp":
		return rankSP
	}
	return rankUnknown
}

func compareTokens(a, b []versionToken) int {
	n := max(len(a), len(b))
	for i := 0; i < n; i++ {
		x, y := versionToken{numeric: true}, versionToken{numeric: true}
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if c := compareToken(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func compareToken(x, y versionToken) int {
	switch {
	case x.numeric && y.numeric:
		return cmpOrdered(x.num, y.num)
	case !x.numeric && !y.numeric:
		rx, ry := qualifierRank(x.qual), qualifierRank(y.qual)
		if rx != ry {
			return cmpOrdered(uint64(rx), uint64(ry))
		}
		return strings.Compare(x.qual, y.qual)
	case x.numeric:
		// number vs qualifier: zero behaves like the release marker.
		if x.num == 0 {
			return cmpOrdered(uint64(rankRelease), uint64(qualifierRank(y.qual)))
		}
		return 1
	default:
		return -compareToken(y, x)
	}
}

func cmpOrdered(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// excludesOnlyAtUpperBound reports whether the Maven range excludes version
// solely because it is the exclusive upper bound of an interval whose lower
// bound it meets, such as "[1.21,1.21.1)" and "1.21.1". Authors write such
// ranges to mean "the 1.21 line" and the mods run on that version, so callers
// treat the mismatch as doubtful rather than certain.
func excludesOnlyAtUpperBound(rangeExpr, version string) bool {
	expr := strings.TrimSpace(rangeExpr)
	if expr == "" || (expr[0] != '[' && expr[0] != '(') {
		return false
	}
	intervals, err := parseMavenRange(expr)
	if err != nil {
		return false
	}
	vt, ok := tokenizeVersion(strings.TrimSpace(version))
	if !ok {
		return false
	}
	for _, iv := range intervals {
		if iv.hasHi && !iv.hiIncl && compareTokens(vt, iv.hi) == 0 &&
			(!iv.hasLo || compareTokens(vt, iv.lo) >= 0) {
			return true
		}
	}
	return false
}
