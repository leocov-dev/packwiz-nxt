package sources

// compareChain evaluates the given comparators in order and returns the first non-zero
// result (positive meaning "candidate preferred", negative meaning "current best preferred"),
// or 0 if every comparator reports a tie.
//
// This captures the "keep the best candidate so far, using a multi-key priority comparison"
// shape shared by the CurseForge (sources/cf-updater.go) and Modrinth (sources/mr-api.go)
// "find latest file/version" algorithms: both compare candidates first by Minecraft version,
// then by loader preference, then by a provider-specific tiebreaker. The loader-preference
// tables themselves differ structurally between providers (CurseForge uses an ordinal index
// scheme, Modrinth a string/compat-group scheme) and are intentionally NOT unified here -
// only the "chain of comparators, first non-zero wins" control flow is shared, via
// comparators supplied as closures by each provider.
func compareChain(comparators ...func() int32) int32 {
	for _, cmp := range comparators {
		if c := cmp(); c != 0 {
			return c
		}
	}
	return 0
}
