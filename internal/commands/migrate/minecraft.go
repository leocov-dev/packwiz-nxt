package migrate

import (
	"fmt"
	packCmd "github.com/leocov-dev/packwiz-nxt/cmd"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
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
				cmdshared.Exitln("No pack.toml file found, run 'packwiz init' to create one!")
			}
			cmdshared.Exitf("Error loading pack: %s\n", err)
		}
		currentVersion, err := modpack.GetMCVersion()
		if err != nil {
			cmdshared.Exitf("Error getting Minecraft version from pack: %s\n", err)
		}
		wantedMCVersion := args[0]
		if wantedMCVersion == currentVersion {
			fmt.Printf("Minecraft version is already %s!\n", wantedMCVersion)
			os.Exit(0)
		}
		mcVersions, err := cmdshared.GetValidMCVersions()
		if err != nil {
			cmdshared.Exitf("Error getting Minecraft versions: %s\n", err)
		}
		mcVersions.CheckValid(wantedMCVersion)
		// Set the version in the pack
		modpack.Versions["minecraft"] = wantedMCVersion
		// Write the pack to disk
		packWriter := fileio.NewPackWriter()
		err = packWriter.Write(&modpack)
		if err != nil {
			cmdshared.Exitf("Error writing pack.toml: %s\n", err)
		}
		fmt.Printf("Successfully updated Minecraft version to %s\n", wantedMCVersion)
		// Prompt the user if they want to update the loader too while they're at it.
		if cmdshared.PromptYesNo("Would you like to update your loader version to the latest version for this Minecraft version? [Y/n] ") {
			// We'll run the loader command to update to latest
			loaderCommand.Run(loaderCommand, []string{"latest"})
		}
		// Prompt the user to update their mods too.
		if cmdshared.PromptYesNo("Would you like to update your mods to the latest versions for this Minecraft version? [Y/n] ") {
			// Run the update command
			viper.Set("update.all", true)
			packCmd.UpdateCmd.Run(packCmd.UpdateCmd, []string{})
		}
	},
}

func init() {
	migrateCmd.AddCommand(minecraftCommand)
}
