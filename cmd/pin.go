package cmd

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func pinMod(args []string, pinned bool) {
	fmt.Println("Loading modpack...")
	pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
	if err != nil {
		cmdshared.Exitln(err)
	}
	index, err := fileio.LoadPackIndexFile(&pack)
	if err != nil {
		cmdshared.Exitln(err)
	}
	modPath, ok := index.FindMod(args[0])
	if !ok {
		cmdshared.Exitln("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
	}
	modData, err := core.LoadMod(modPath)
	if err != nil {
		cmdshared.Exitln(err)
	}
	modData.Pin = pinned

	modWriter := fileio.NewModWriter()
	format, hash, err := modWriter.Write(&modData)
	if err != nil {
		cmdshared.Exitln(err)
	}

	err = index.RefreshFileWithHash(modPath, format, hash, true)
	if err != nil {
		cmdshared.Exitln(err)
	}

	repr := index.ToWritable()
	writer := fileio.NewIndexWriter()
	format, hash, err = writer.Write(&repr)
	if err != nil {
		cmdshared.Exitln(err)
	}

	pack.RefreshIndexHash(format, hash)

	packWriter := fileio.NewPackWriter()
	err = packWriter.Write(&pack)
	if err != nil {
		cmdshared.Exitln(err)
	}

	message := "pinned"
	if !pinned {
		message = "unpinned"
	}
	fmt.Printf("%s %s successfully!\n", args[0], message)
}

// pinCmd represents the pin command
var pinCmd = &cobra.Command{
	Use:     "pin",
	Short:   "Pin a file so it does not get updated automatically",
	Aliases: []string{"hold"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pinMod(args, true)
	},
}

// unpinCmd represents the unpin command
var unpinCmd = &cobra.Command{
	Use:     "unpin",
	Short:   "Unpin a file so it receives updates",
	Aliases: []string{"unhold"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pinMod(args, false)
	},
}

func init() {
	rootCmd.AddCommand(pinCmd)
	rootCmd.AddCommand(unpinCmd)
}
