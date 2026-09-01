package cmdutils

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
)

var maxSizeBytes int64

// cacheCleanCmd represents the cache-clean command
var cacheCleanCmd = &cobra.Command{
	Use:   "cache-clean",
	Short: "Remove download cache entries whose backing file is missing or corrupted, and optionally evict least-recently-used entries over a size budget",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cacheIndex, err := fileio.OpenCacheIndex()
		if err != nil {
			shared.Exitf("Error opening download cache: %s\n", err)
		}

		removedOrphaned := cacheIndex.PruneOrphaned()

		var removedLRU int
		var freedBytes int64
		if maxSizeBytes > 0 {
			removedLRU, freedBytes, err = cacheIndex.EvictLRU(maxSizeBytes)
			if err != nil {
				shared.Exitf("Error evicting cache entries: %s\n", err)
			}
		}

		err = cacheIndex.Save()
		if err != nil {
			shared.Exitf("Error saving download cache: %s\n", err)
		}

		fmt.Printf("Removed %d orphaned cache entr%s\n", removedOrphaned, plural(removedOrphaned))
		if maxSizeBytes > 0 {
			fmt.Printf("Evicted %d least-recently-used cache entr%s (%d bytes freed)\n", removedLRU, plural(removedLRU), freedBytes)
		}
	},
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func init() {
	cacheCleanCmd.Flags().Int64Var(&maxSizeBytes, "max-size-bytes", 0,
		"if set (>0), also evict least-recently-used cache entries until the cache is at or under this size in bytes")
	utilsCmd.AddCommand(cacheCleanCmd)
}
