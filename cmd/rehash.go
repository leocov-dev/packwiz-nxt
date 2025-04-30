package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/leocov-dev/fork.packwiz/core"
	"github.com/leocov-dev/fork.packwiz/internal/cmdshared"
)

// rehashCmd represents the rehash command
var rehashCmd = &cobra.Command{
	Use:   "rehash [hash format]",
	Short: "Migrate all hashes to a specific format",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		// Load pack
		pack, err := core.LoadPack()
		if err != nil {
			cmdshared.Exitln(err)
		}

		// Load index
		index, err := pack.LoadIndex()
		if err != nil {
			cmdshared.Exitln(err)
		}

		// Load mods
		mods, err := index.LoadAllMods()
		if err != nil {
			cmdshared.Exitln(err)
		}

		if !slices.Contains([]string{"sha1", "sha512", "sha256"}, args[0]) {
			cmdshared.Exitf("Hash format '%s' is not supported\n", args[0])
		}

		session, err := core.CreateDownloadSession(mods, []string{args[0]})
		if err != nil {
			cmdshared.Exitf("Error retrieving external files: %v\n", err)
		}

		cmdshared.ListManualDownloads(session)

		for dl := range session.StartDownloads() {
			if dl.Error != nil {
				fmt.Printf("Error retrieving %s: %v\n", dl.Mod.Name, dl.Error)
			} else {
				dl.Mod.Download.HashFormat = args[0]
				dl.Mod.Download.Hash = dl.Hashes[args[0]]
				_, _, err := dl.Mod.Write()
				if err != nil {
					cmdshared.Exitf("Error saving mod %s: %v\n", dl.Mod.Name, err)
				}
			}
			// TODO pass the hash to index instead of recomputing from scratch
		}

		err = session.SaveIndex()
		if err != nil {
			cmdshared.Exitf("Error saving cache index: %v\n", err)
		}

		err = index.Refresh()
		if err != nil {
			cmdshared.Exitf("Error refreshing index: %v\n", err)
		}

		err = index.Write()
		if err != nil {
			cmdshared.Exitf("Error writing index: %v\n", err)
		}

		err = pack.UpdateIndexHash()
		if err != nil {
			cmdshared.Exitf("Error updating index hash: %v\n", err)
		}

		err = pack.Write()
		if err != nil {
			cmdshared.Exitf("Error writing pack: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(rehashCmd)
}
