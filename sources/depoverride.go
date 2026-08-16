package sources

import (
	"golang.org/x/exp/slices"

	"github.com/unascribed/FlexVer/go/flexver"
)

// depOverrideRule describes a hardcoded dependency substitution: some projects declare a
// dependency on a Fabric library even when a Quilt-native replacement is available, so
// packwiz swaps in the Quilt project when the pack targets Quilt (and, if versionGate is
// set, only within a specific Minecraft version range). This will likely be removed once
// packwiz is able to determine provided mods itself.
type depOverrideRule struct {
	// name is for reference/debugging only.
	name string

	cfProjectID uint32
	cfQuiltID   uint32

	// mrIDs lists every Modrinth project ID/slug that identifies the dependency (Modrinth
	// dependencies may be expressed as either).
	mrIDs     []string
	mrQuiltID string

	// versionGate, if non-nil, must return true for the override to apply. A nil gate means
	// the override always applies (once isQuilt is true).
	versionGate func(mcVersion string) bool
}

// quiltFlkVersionGate matches the Minecraft version range where Quilt Kotlin Library is the
// intended replacement for Fabric Language Kotlin (Quilt >=1.19.2 non-snapshot).
func quiltFlkVersionGate(mcVersion string) bool {
	return flexver.Less("1.19.1", mcVersion) && flexver.Less(mcVersion, "2.0.0")
}

// depOverrideRules is the single shared table of dependency overrides applied by both the
// CurseForge (MapDepOverride) and Modrinth (mrMapDepOverride) providers - only the ID
// literals differ per provider, the substitution rule and version-gating logic are identical.
var depOverrideRules = []depOverrideRule{
	{
		name:        "Fabric API",
		cfProjectID: 306612,
		cfQuiltID:   634179,
		mrIDs:       []string{"P7dR8mSH", "fabric-api"},
		mrQuiltID:   "qvIfYCYJ",
	},
	{
		name:        "Fabric Language Kotlin",
		cfProjectID: 308769,
		cfQuiltID:   720410,
		mrIDs:       []string{"Ha28R6CL", "fabric-language-kotlin"},
		mrQuiltID:   "lwVhp9o5",
		versionGate: quiltFlkVersionGate,
	},
}

// findDepOverrideRule returns the override rule matching the given CurseForge project ID, if
// any.
func findCfDepOverrideRule(cfProjectID uint32) (depOverrideRule, bool) {
	for _, rule := range depOverrideRules {
		if rule.cfProjectID == cfProjectID {
			return rule, true
		}
	}
	return depOverrideRule{}, false
}

// findMrDepOverrideRule returns the override rule matching the given Modrinth project
// ID/slug, if any.
func findMrDepOverrideRule(mrID string) (depOverrideRule, bool) {
	for _, rule := range depOverrideRules {
		if slices.Contains(rule.mrIDs, mrID) {
			return rule, true
		}
	}
	return depOverrideRule{}, false
}
