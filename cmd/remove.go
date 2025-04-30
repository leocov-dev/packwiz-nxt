package cmd

import (
	"fmt"
	"github.com/leocov-dev/fork.packwiz/internal/cmdshared"
	"os"

	"github.com/leocov-dev/fork.packwiz/core"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:     "remove",
	Short:   "Remove an external file from the modpack; equivalent to manually removing the file and running packwiz refresh",
	Aliases: []string{"delete", "uninstall", "rm"},
	Args:    cobra.ExactArgs(1),
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
		resolvedMod, ok := index.FindMod(args[0])
		if !ok {
			cmdshared.Exitln("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
		}
		err = os.Remove(resolvedMod)
		if err != nil {
			cmdshared.Exitln(err)
		}
		fmt.Println("Removing file from index...")
		err = index.RemoveFile(resolvedMod)
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
		err = pack.Write()
		if err != nil {
			cmdshared.Exitln(err)
		}

		fmt.Printf("%s removed successfully!\n", args[0])
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
