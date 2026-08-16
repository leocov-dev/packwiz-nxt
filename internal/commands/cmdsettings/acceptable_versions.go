package cmdsettings

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		// Check if they have no options whatsoever
		if modpack.Options == nil {
			// Initialize the options
			modpack.Options = make(map[string]interface{})
		}

		// Compute which mutation to apply, and the corresponding success message prefix
		var msgPrefix string
		switch {
		case flagAdd:
			acceptableVersion := args[0]
			if err := modpack.AddAcceptableVersion(acceptableVersion); err != nil {
				shared.Exitf("Version %s is already in your acceptable versions list!\n", acceptableVersion)
			}
			msgPrefix = fmt.Sprintf("Added %s to acceptable versions list, now", acceptableVersion)
		case flagRemove:
			acceptableVersion := args[0]
			if err := modpack.RemoveAcceptableVersion(acceptableVersion); err != nil {
				shared.Exitf("Version %s is not in your acceptable versions list!\n", acceptableVersion)
			}
			msgPrefix = fmt.Sprintf("Removed %s from acceptable versions list, now", acceptableVersion)
		default:
			// Overwriting
			acceptableVersionsList := strings.Split(args[0], ",")
			modpack.SetAcceptableGameVersions(acceptableVersionsList)
			msgPrefix = "Set acceptable versions to"
		}

		// Save the pack
		packWriter := fileio.NewPackWriter()
		if err := packWriter.Write(&modpack); err != nil {
			shared.Exitf("Error writing pack: %s\n", err)
		}

		// Print success message
		finalVersions, err := modpack.GetAcceptableGameVersions()
		if err != nil {
			shared.Exitf("Error reading acceptable versions: %s\n", err)
		}
		prettyList := strings.Join(finalVersions, ", ")
		prettyList += ", " + modpack.Versions["minecraft"]
		fmt.Printf("%s %s\n", msgPrefix, prettyList)
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
