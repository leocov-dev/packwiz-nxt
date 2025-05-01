package migrate

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"os"
)

var loaderCommand = &cobra.Command{
	Use:   "loader [version|latest|recommended]",
	Short: "Migrate your modloader version to a newer version.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modpack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			// Check if it's a no such file or directory error
			if os.IsNotExist(err) {
				cmdshared.Exitln("No pack.toml file found, run 'packwiz init' to create one!")
			}
			cmdshared.Exitf("Error loading pack: %s\n", err)
		}
		var currentLoaders = modpack.GetLoaders()
		// Do some sanity checks on the current loader slice
		if len(currentLoaders) == 0 {
			cmdshared.Exitln("No loader is currently set in your pack.toml!")
		} else if len(currentLoaders) > 1 {
			cmdshared.Exitln("You have multiple loaders set in your pack.toml, this is not supported!")
		}
		// Get the Minecraft version for the pack
		mcVersion, err := modpack.GetMCVersion()
		if err != nil {
			cmdshared.Exitf("Error getting Minecraft version: %s\n", err)
		}

		packWriter := fileio.NewPackWriter()
		if args[0] == "latest" {
			fmt.Println("Updating to latest loader version")
			// We'll be updating to the latest loader version
			for _, loader := range currentLoaders {
				_, latest, gottenLoader := getVersionsForLoader(loader, mcVersion)
				if !updatePackToVersion(latest, modpack, gottenLoader) {
					continue
				}
				// Write the pack to disk
				err = packWriter.Write(&modpack)
				if err != nil {
					fmt.Printf("Error writing pack.toml: %s\n", err)
					continue
				}
			}
		} else if args[0] == "recommended" {
			// TODO: Figure out a way to get the recommended version, this is Forge only
			// Ensure we're on Forge
			if !slices.Contains(currentLoaders, "forge") {
				cmdshared.Exitln("The recommended loader version is only available on Forge!")
			}
			// We'll be updating to the recommended loader version
			recommendedVer := core.GetForgeRecommended(mcVersion)
			if recommendedVer == "" {
				cmdshared.Exitln("Error getting recommended Forge version!")
			}
			if ok := updatePackToVersion(recommendedVer, modpack, core.ModLoaders["forge"]); !ok {
				os.Exit(1)
			}
			// Write the pack to disk
			packWriter := fileio.NewPackWriter()
			err = packWriter.Write(&modpack)
			if err != nil {
				cmdshared.Exitf("Error writing pack.toml: %s", err)
			}
		} else {
			fmt.Println("Updating to explicit loader version")
			// This one is easy :D
			versions, _, loader := getVersionsForLoader(currentLoaders[0], mcVersion)
			// Check if the loader happens to be Forge/NeoForge, since there's two version formats
			if loader.Name == "forge" || loader.Name == "neoforge" {
				wantedVersion := cmdshared.GetRawForgeVersion(args[0])
				validateVersion(versions, wantedVersion, loader)
				_ = updatePackToVersion(wantedVersion, modpack, loader)
			} else if loader.Name == "liteloader" {
				// These are weird and just have a MC version
				fmt.Println("LiteLoader only has 1 version per Minecraft version so we're unable to update!")
				os.Exit(0)
			} else {
				// We're on Fabric or quilt
				validateVersion(versions, args[0], loader)
				if ok := updatePackToVersion(args[0], modpack, loader); !ok {
					os.Exit(1)
				}
			}
			// Write the pack to disk
			packWriter := fileio.NewPackWriter()
			err = packWriter.Write(&modpack)
			if err != nil {
				cmdshared.Exitf("Error writing pack.toml: %s\n", err)
			}
		}
	},
}

func init() {
	migrateCmd.AddCommand(loaderCommand)
}

func getVersionsForLoader(loader, mcVersion string) ([]string, string, core.ModLoaderComponent) {
	gottenLoader, ok := core.ModLoaders[loader]
	if !ok {
		cmdshared.Exitf("Unknown loader %s\n", loader)
	}
	versions, latestVersion, err := gottenLoader.VersionListGetter(mcVersion)
	if err != nil {
		cmdshared.Exitf("Error getting version list for %s: %s\n", gottenLoader.FriendlyName, err)
	}
	return versions, latestVersion, gottenLoader
}

func validateVersion(versions []string, version string, gottenLoader core.ModLoaderComponent) {
	if !slices.Contains(versions, version) {
		cmdshared.Exitf("Version %s is not a valid version for %s\n", version, gottenLoader.FriendlyName)
	}
}

func updatePackToVersion(version string, modpack core.Pack, loader core.ModLoaderComponent) bool {
	// Check if the version is already set
	if version == modpack.Versions[loader.Name] {
		fmt.Printf("%s is already on version %s!\n", loader.FriendlyName, version)
		return false
	}
	// Set the latest version
	modpack.Versions[loader.Name] = version
	fmt.Printf("Updated %s to version %s\n", loader.FriendlyName, version)
	return true
}
