package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
)

// rehashCmd represents the rehash command
var rehashCmd = &cobra.Command{
	Use:   "rehash [hash format]",
	Short: "Migrate all hashes to a specific format",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		packPath, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		// Load pack
		pack, err := fileio.LoadAll(packPath)
		if err != nil {
			shared.Exitln(err)
		}

		if !slices.Contains([]string{"sha1", "sha512", "sha256"}, args[0]) {
			shared.Exitf("Hash format '%s' is not supported\n", args[0])
		}

		session, err := fileio.CreateDownloadSession(pack.GetModsList(), []string{args[0]})
		if err != nil {
			shared.Exitf("Error retrieving external files: %v\n", err)
		}

		shared.ListManualDownloads(session)

		for dl := range session.StartDownloads() {
			if dl.Error != nil {
				fmt.Printf("Error retrieving %s: %v\n", dl.Mod.Name, dl.Error)
			} else {
				dl.Mod.Download.HashFormat = args[0]
				dl.Mod.Download.Hash = dl.Hashes[args[0]]
			}
		}

		err = session.SaveIndex()
		if err != nil {
			shared.Exitf("Error saving cache index: %v\n", err)
		}

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitf("Error writing pack: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(rehashCmd)
}
