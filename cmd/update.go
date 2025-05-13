package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
)

// UpdateCmd represents the update command
var UpdateCmd = &cobra.Command{
	Use:     "update [name]",
	Short:   "Update an external file (or all external files) in the modpack",
	Aliases: []string{"upgrade"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: --check flag?
		// TODO: specify multiple files to update at once?

		fmt.Println("Loading modpack...")

		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		var singleUpdatedName string
		if viper.GetBool("update.all") {
			fmt.Println("Checking for updates...")
			if err := core.UpdateAllMods(*pack); err != nil {
				shared.Exitln(err)
			}
		} else {
			if len(args) < 1 || len(args[0]) == 0 {
				shared.Exitln("Must specify a valid file, or use the --all flag!")
			}

			mod, ok := pack.Mods[args[0]]
			if !ok {
				shared.Exitln("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
			}

			if mod.Pin {
				shared.Exitln("Version is pinned; run the unpin command to allow updating")
			}

			if err := core.UpdateSingleMod(*pack, mod); err != nil {
				shared.Exitln(err)
			}

		}

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitln(err)
		}

		if viper.GetBool("update.all") {
			fmt.Println("Files updated!")
		} else {
			fmt.Printf("\"%s\" updated!\n", singleUpdatedName)
		}
	},
}

func init() {
	rootCmd.AddCommand(UpdateCmd)

	UpdateCmd.Flags().BoolP("all", "a", false, "Update all external files")
	_ = viper.BindPFlag("update.all", UpdateCmd.Flags().Lookup("all"))
}
