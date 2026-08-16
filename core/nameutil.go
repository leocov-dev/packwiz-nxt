package core

import (
	"regexp"
	"strings"
)

var stripParens = regexp.MustCompile(`\(.*\)`)
var stripDashSuffix = regexp.MustCompile(` - .+`)
var nonAlnumToDash = regexp.MustCompile(`[^a-z\d]`)
var collapseDashes = regexp.MustCompile(`-+`)
var trimEdgeDashes = regexp.MustCompile(`^-|-$`)

func SlugifyName(name string) string {
	lower := strings.ToLower(name)
	noBrackets := stripParens.ReplaceAllString(lower, "")
	noSuffix := stripDashSuffix.ReplaceAllString(noBrackets, "")
	limitedChars := nonAlnumToDash.ReplaceAllString(noSuffix, "-")
	noDuplicateDashes := collapseDashes.ReplaceAllString(limitedChars, "-")
	noLeadingTrailingDashes := trimEdgeDashes.ReplaceAllString(noDuplicateDashes, "")
	return noLeadingTrailingDashes
}
