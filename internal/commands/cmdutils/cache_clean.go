package cmdutils

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
)

// cacheCleanCmd represents the cache-clean command
var cacheCleanCmd = &cobra.Command{
	Use:   "cache-clean",
	Short: "Remove download cache entries whose backing file is missing or corrupted",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cacheIndex, err := fileio.OpenCacheIndex()
		if err != nil {
			shared.Exitf("Error opening download cache: %s\n", err)
		}

		removed := cacheIndex.PruneOrphaned()

		err = cacheIndex.Save()
		if err != nil {
			shared.Exitf("Error saving download cache: %s\n", err)
		}

		fmt.Printf("Removed %d orphaned cache entr%s\n", removed, plural(removed))
	},
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func init() {
	utilsCmd.AddCommand(cacheCleanCmd)
}
