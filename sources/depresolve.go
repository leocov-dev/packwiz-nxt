package sources

import "fmt"

// DefaultMaxDependencyCycles is the shared cycle limit used by both the CurseForge and
// Modrinth "find missing dependencies" resolvers, as a safety net against pathologically
// deep (or circular) dependency graphs.
const DefaultMaxDependencyCycles = 20

// runDependencyResolution drives the breadth-first "resolve missing dependencies" loop
// shared by the CurseForge and Modrinth providers: starting from an initial set of
// dependency IDs, it repeatedly narrows the pending set with prepareNext (deduping against
// already-installed/already-collected IDs, and doing any provider-specific bookkeeping such
// as resolving intermediate ID types), then hands the pending batch to fetchAndExpand to
// fetch data for those IDs and discover further dependency IDs to chase. It stops once the
// pending set is empty, or returns an error once maxCycles batches have been processed
// without the set emptying.
//
// Provider-specific concerns - how IDs are batch-fetched, how dependency-override tables are
// applied, and (for Modrinth) how version IDs are disambiguated from project IDs - stay in
// the prepareNext/fetchAndExpand closures supplied by each provider; this function only owns
// the queue/cycle-limit/dedup control flow.
func runDependencyResolution[ID comparable](
	initial []ID,
	maxCycles int,
	prepareNext func(newIDs []ID) ([]ID, error),
	fetchAndExpand func(pending []ID) (newIDs []ID, err error),
) error {
	pending, err := prepareNext(initial)
	if err != nil {
		return err
	}

	for cycles := 0; len(pending) > 0; cycles++ {
		if cycles >= maxCycles {
			return fmt.Errorf("dependencies recurse too deeply! Try increasing the max dependency resolution cycles (limit: %d)", maxCycles)
		}

		newIDs, err := fetchAndExpand(pending)
		if err != nil {
			return err
		}

		pending, err = prepareNext(newIDs)
		if err != nil {
			return err
		}
	}

	return nil
}
