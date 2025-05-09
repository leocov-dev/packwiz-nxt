package cmd

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/spf13/cobra"
)

func pinMod(args []string, pinned bool) {
	fmt.Println("Loading modpack...")

	packFile, packDir, err := shared.GetPackPaths()
	if err != nil {
		shared.Exitln(err)
	}

	pack, err := fileio.LoadAll(packFile)
	if err != nil {
		shared.Exitln(err)
	}

	pack.Mods[args[0]].Pin = pinned

	err = fileio.WritePackAndIndex(*pack, packDir)
	if err != nil {
		shared.Exitln(err)
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
