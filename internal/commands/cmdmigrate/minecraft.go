package cmdmigrate

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var minecraftCommand = &cobra.Command{
	Use:     "minecraft [version]",
	Short:   "Migrate your Minecraft version to a newer version.",
	Aliases: []string{"mc"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modpack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			// Check if it's a no such file or directory error
			if os.IsNotExist(err) {
				shared.Exitln("No pack.toml file found, run 'packwiz init' to create one!")
			}
			shared.Exitf("Error loading pack: %s\n", err)
		}
		currentVersion, err := modpack.GetMCVersion()
		if err != nil {
			shared.Exitf("Error getting Minecraft version from pack: %s\n", err)
		}
		wantedMCVersion := args[0]
		if wantedMCVersion == currentVersion {
			fmt.Printf("Minecraft version is already %s!\n", wantedMCVersion)
			os.Exit(0)
		}
		mcVersions, err := core.GetMinecraftVersions()
		if err != nil {
			shared.Exitf("Error getting Minecraft versions: %s\n", err)
		}

		if !mcVersions.CheckValid(wantedMCVersion) {
			shared.Exitf("the Minecraft version %s is not valid\n", wantedMCVersion)
		}

		// Set the version in the pack
		modpack.Versions["minecraft"] = wantedMCVersion
		// Write the pack to disk
		packWriter := fileio.NewPackWriter()
		err = packWriter.Write(&modpack)
		if err != nil {
			shared.Exitf("Error writing pack.toml: %s\n", err)
		}
		fmt.Printf("Successfully updated Minecraft version to %s\n", wantedMCVersion)
		// Prompt the user if they want to update the loader too while they're at it.
		if shared.PromptYesNo("Would you like to update your loader version to the latest version for this Minecraft version? [Y/n] ") {
			// Update the loader directly, rather than going through the loader command's Run
			updateLoaderToLatest(modpack)
		}
		// Prompt the user to update their mods too.
		if shared.PromptYesNo("Would you like to update your mods to the latest versions for this Minecraft version? [Y/n] ") {
			// Update all mods directly, rather than going through the update command's Run
			packFile, packDir, err := shared.GetPackPaths()
			if err != nil {
				shared.Exitln(err)
			}
			fmt.Println("Loading modpack...")
			fullPack, err := fileio.LoadAll(packFile)
			if err != nil {
				shared.Exitln(err)
			}
			fmt.Println("Checking for updates...")
			if err := core.UpdateAllMods(*fullPack); err != nil {
				shared.Exitln(err)
			}
			if err := fileio.WriteAll(*fullPack, packDir); err != nil {
				shared.Exitln(err)
			}
			fmt.Println("Files updated!")
		}
	},
}

func init() {
	migrateCmd.AddCommand(minecraftCommand)
}
