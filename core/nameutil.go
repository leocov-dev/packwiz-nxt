package core

import (
	"regexp"
	"strings"
)

var slugifyRegex1 = regexp.MustCompile(`\(.*\)`)
var slugifyRegex2 = regexp.MustCompile(` - .+`)
var slugifyRegex3 = regexp.MustCompile(`[^a-z\d]`)
var slugifyRegex4 = regexp.MustCompile(`-+`)
var slugifyRegex5 = regexp.MustCompile(`^-|-$`)

func SlugifyName(name string) string {
	lower := strings.ToLower(name)
	noBrackets := slugifyRegex1.ReplaceAllString(lower, "")
	noSuffix := slugifyRegex2.ReplaceAllString(noBrackets, "")
	limitedChars := slugifyRegex3.ReplaceAllString(noSuffix, "-")
	noDuplicateDashes := slugifyRegex4.ReplaceAllString(limitedChars, "-")
	noLeadingTrailingDashes := slugifyRegex5.ReplaceAllString(noDuplicateDashes, "")
	return noLeadingTrailingDashes
}
