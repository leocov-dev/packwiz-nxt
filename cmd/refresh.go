package cmd

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// refreshCmd represents the refresh command
var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the index file",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			cmdshared.Exitln(err)
		}

		index, err := fileio.LoadPackIndexFile(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}
		err = fileio.RefreshIndexFiles(&index)
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
