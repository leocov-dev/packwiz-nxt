package cmdsettings

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"os"
	"strings"
)

var acceptableVersionsCommand = &cobra.Command{
	Use:     "acceptable-versions",
	Short:   "Manage your pack's acceptable Minecraft versions. This must be a comma seperated list of Minecraft versions, e.g. 1.16.3,1.16.4,1.16.5",
	Aliases: []string{"av"},
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
		var currentVersions []string
		// Check if they have no options whatsoever
		if modpack.Options == nil {
			// Initialize the options
			modpack.Options = make(map[string]interface{})
		}
		// Check if the acceptable-game-versions is nil, which would mean their pack.toml doesn't have it set yet
		acceptableGameVersions := modpack.GetAcceptableGameVersions()
		if acceptableGameVersions != nil {
			// Convert the interface{} to a string slice
			for _, v := range acceptableGameVersions {
				currentVersions = append(currentVersions, v)
			}
		}
		// Check our flags to see if we're adding or removing
		if flagAdd {
			acceptableVersion := args[0]
			// Check if the version is already in the list
			if slices.Contains(currentVersions, acceptableVersion) {
				shared.Exitf("Version %s is already in your acceptable versions list!\n", acceptableVersion)
			}
			// Add the version to the list and re-sort it
			currentVersions = append(currentVersions, acceptableVersion)
			// Set the new list
			modpack.SetAcceptableGameVersions(currentVersions)
			// Save the pack
			packWriter := fileio.NewPackWriter()
			err = packWriter.Write(&modpack)
			if err != nil {
				shared.Exitf("Error writing pack: %s\n", err)
			}
			// Print success message
			prettyList := strings.Join(currentVersions, ", ")
			prettyList += ", " + modpack.Versions["minecraft"]
			fmt.Printf("Added %s to acceptable versions list, now %s\n", acceptableVersion, prettyList)
		} else if flagRemove {
			acceptableVersion := args[0]
			// Check if the version is in the list
			if !slices.Contains(currentVersions, acceptableVersion) {
				shared.Exitf("Version %s is not in your acceptable versions list!\n", acceptableVersion)
			}
			// Remove the version from the list
			i := slices.Index(currentVersions, acceptableVersion)
			currentVersions = slices.Delete(currentVersions, i, i+1)
			// Set the new list
			modpack.SetAcceptableGameVersions(currentVersions)
			// Save the pack
			packWriter := fileio.NewPackWriter()
			err = packWriter.Write(&modpack)
			if err != nil {
				shared.Exitf("Error writing pack: %s\n", err)
			}
			// Print success message
			prettyList := strings.Join(currentVersions, ", ")
			prettyList += ", " + modpack.Versions["minecraft"]
			fmt.Printf("Removed %s from acceptable versions list, now %s\n", acceptableVersion, prettyList)
		} else {
			// Overwriting
			acceptableVersions := args[0]
			acceptableVersionsList := strings.Split(acceptableVersions, ",")
			modpack.SetAcceptableGameVersions(acceptableVersionsList)
			packWriter := fileio.NewPackWriter()
			err = packWriter.Write(&modpack)
			if err != nil {
				shared.Exitf("Error writing pack: %s\n", err)
			}
			// Print success message
			prettyList := strings.Join(acceptableVersionsList, ", ")
			prettyList += ", " + modpack.Versions["minecraft"]
			fmt.Printf("Set acceptable versions to %s\n", prettyList)
		}
	},
}

var flagAdd bool
var flagRemove bool

func init() {
	settingsCmd.AddCommand(acceptableVersionsCommand)

	// Add and remove flags for adding or removing specific versions
	acceptableVersionsCommand.Flags().BoolVarP(&flagAdd, "add", "a", false, "Add a version to the list")
	acceptableVersionsCommand.Flags().BoolVarP(&flagRemove, "remove", "r", false, "Remove a version from the list")
}
