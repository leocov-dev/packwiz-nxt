package cmdcurseforge

import (
	"fmt"
	"path/filepath"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// detectCmd represents the detect command
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect .jar files in the mods folder (experimental)",
	Args:  cobra.NoArgs,
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

		// The directory containing the actual mod files, e.g. "mods" (as opposed to
		// meta-folder-base, which is where packwiz writes the .toml metadata for a mod;
		// the two are conventionally the same folder name, so we reuse meta-folder here).
		modType := viper.GetString("meta-folder")
		if modType == "" {
			modType = "mods"
		}
		modsDir := filepath.Join(viper.GetString("meta-folder-base"), modType)

		result, err := sources.CurseforgeDetectMods(modsDir)
		if err != nil {
			shared.Exitln(err)
		}
		if result == nil {
			// The fingerprint lookup failed and was already reported; nothing more to do.
			return
		}

		fmt.Printf("Successfully matched %d files\n", result.MatchedCount)
		if len(result.PartialMatches) > 0 {
			fmt.Println("The following fingerprints were partial and I don't know what to do!!!")
			for _, v := range result.PartialMatches {
				fmt.Printf("%s (%d)", v.Path, v.Fingerprint)
			}
		}
		if len(result.UnmatchedFiles) > 0 {
			fmt.Printf("Failed to match the following %d files:\n", len(result.UnmatchedFiles))
			for _, v := range result.UnmatchedFiles {
				fmt.Printf("%s (%d)\n", v.Path, v.Fingerprint)
			}
		}

		for _, mod := range result.Mods {
			pack.SetMod(mod)
		}
		fmt.Println("Detection complete!")

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitln(err)
		}
	},
}

func init() {
	curseforgeCmd.AddCommand(detectCmd)
}
