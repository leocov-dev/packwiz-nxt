package cmd

import (
	"fmt"
	"github.com/leocov-dev/fork.packwiz/core"
	"github.com/leocov-dev/fork.packwiz/fileio"
	"github.com/leocov-dev/fork.packwiz/internal/cmdshared"
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
		pack, err := core.LoadPack()
		if err != nil {
			cmdshared.Exitln(err)
		}
		build, err := cmd.Flags().GetBool("build")
		if err == nil && build {
			viper.Set("no-internal-hashes", false)
		} else if viper.GetBool("no-internal-hashes") {
			fmt.Println("Note: no-internal-hashes mode is set, no hashes will be saved. Use --build to override this for distribution.")
		}
		index, err := pack.LoadIndex()
		if err != nil {
			cmdshared.Exitln(err)
		}
		err = index.Refresh()
		if err != nil {
			cmdshared.Exitln(err)
		}
		err = index.Write()
		if err != nil {
			cmdshared.Exitln(err)
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			cmdshared.Exitln(err)
		}
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

	refreshCmd.Flags().Bool("build", false, "Only has an effect in no-internal-hashes mode: generates internal hashes for distribution with packwiz-installer")
}
