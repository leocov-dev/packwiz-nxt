package cmd

import (
	"fmt"
	"github.com/leocov-dev/fork.packwiz/core"
	"github.com/leocov-dev/fork.packwiz/fileio"
	"github.com/leocov-dev/fork.packwiz/internal/cmdshared"
	"github.com/spf13/cobra"
)

// refreshCmd represents the refresh command
var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the index file",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		pack, err := core.LoadPack()
		if err != nil {
			cmdshared.Exitln(err)
		}

		index, err := pack.LoadIndex()
		if err != nil {
			cmdshared.Exitln(err)
		}
		err = index.Refresh()
		if err != nil {
			cmdshared.Exitln(err)
		}

		repr := index.ToWritable()
		writer := fileio.NewIndexWriter()
		format, hash, err := writer.Write(&repr)
		if err != nil {
			cmdshared.Exitln(err)
		}

		pack.RefreshIndexHash(format, hash)

		packWriter := fileio.NewPackWriter()
		err = packWriter.Write(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}
		fmt.Println("Index refreshed!")
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
