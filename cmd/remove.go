package cmd

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
	"github.com/spf13/viper"
	"os"

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
		pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			cmdshared.Exitln(err)
		}
		index, err := fileio.LoadPackIndexFile(&pack)
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

		repr := index.ToWritable()
		writer := fileio.NewIndexWriter()
		err = writer.Write(&repr)
		if err != nil {
			cmdshared.Exitln(err)
		}

		pack.RefreshIndexHash(index)

		packWriter := fileio.NewPackWriter()
		err = packWriter.Write(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}

		fmt.Printf("%s removed successfully!\n", args[0])
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
