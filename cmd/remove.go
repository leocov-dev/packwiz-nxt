package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:     "remove",
	Short:   "Remove an external file from the modpack; equivalent to manually removing the file and running packwiz refresh",
	Aliases: []string{"delete", "uninstall", "rm"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")

		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		targetMod := args[0]

		removedMod, ok := pack.Mods[targetMod]
		if !ok {
			shared.Exitln("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
		}

		fmt.Println("Removing file from index...")
		err = os.Remove(filepath.Join(packDir, removedMod.GetRelMetaPath()))
		if err != nil {
			shared.Exitln("Failed to delete mod meta file")
		}

		delete(pack.Mods, targetMod)

		err = fileio.WritePackAndIndex(*pack, packDir)
		if err != nil {
			shared.Exitln(err)
		}

		fmt.Printf("%s removed successfully!\n", args[0])
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
