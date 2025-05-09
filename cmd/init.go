package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/fatih/camelcase"
	"github.com/igorsobreira/titlecase"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"os"
	"path/filepath"
	"strings"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise a packwiz modpack",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		if err := checkReinit(packFile); err != nil {
			shared.Exitln(err)
		}

		name := getPackName(cmd)

		author := getAuthorName(cmd)

		version := getPackVersion(cmd)

		mcVersion, err := getMcVersion()
		if err != nil {
			shared.Exitln(err)
		}

		modLoaderVersions, err := getModLoader(mcVersion)
		if err != nil {
			shared.Exitln(err)
		}

		pack := core.NewPack(
			name,
			author,
			version,
			"",
			mcVersion,
			modLoaderVersions,
		)

		if err := fileio.WriteAll(*pack, packDir); err != nil {
			shared.Exitln(err)
		}

		fmt.Println(viper.GetString("pack-file") + " created!")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().String("name", "", "The name of the modpack (omit to define interactively)")
	initCmd.Flags().String("author", "", "The author of the modpack (omit to define interactively)")
	initCmd.Flags().String("version", "", "The version of the modpack (omit to define interactively)")
	initCmd.Flags().String("mc-version", "", "The Minecraft version to use (omit to define interactively)")
	_ = viper.BindPFlag("init.mc-version", initCmd.Flags().Lookup("mc-version"))
	initCmd.Flags().BoolP("latest", "l", false, "Automatically select the latest version of Minecraft")
	_ = viper.BindPFlag("init.latest", initCmd.Flags().Lookup("latest"))
	initCmd.Flags().BoolP("snapshot", "s", false, "Use the latest snapshot version with --latest")
	_ = viper.BindPFlag("init.snapshot", initCmd.Flags().Lookup("snapshot"))
	initCmd.Flags().BoolP("reinit", "r", false, "Recreate the pack file if it already exists, rather than exiting")
	_ = viper.BindPFlag("init.reinit", initCmd.Flags().Lookup("reinit"))
	initCmd.Flags().String("modloader", "", "The mod loader to use (omit to define interactively)")
	_ = viper.BindPFlag("init.modloader", initCmd.Flags().Lookup("modloader"))

	// ok this is epic
	for _, loader := range core.ModLoaders {
		initCmd.Flags().String(loader.Name+"-version", "", "The "+loader.FriendlyName+" version to use (omit to define interactively)")
		_ = viper.BindPFlag("init."+loader.Name+"-version", initCmd.Flags().Lookup(loader.Name+"-version"))
		initCmd.Flags().Bool(loader.Name+"-latest", false, "Automatically select the latest version of "+loader.FriendlyName)
		_ = viper.BindPFlag("init."+loader.Name+"-latest", initCmd.Flags().Lookup(loader.Name+"-latest"))
	}
}

func initReadValue(prompt string, def string) string {
	fmt.Print(prompt)
	if viper.GetBool("non-interactive") {
		fmt.Printf("%s\n", def)
		return def
	}
	value, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		shared.Exitf("Error reading input: %s\n", err)
	}
	// Trims both CR and LF
	value = strings.TrimSpace(strings.TrimRight(value, "\r\n"))
	if len(value) > 0 {
		return value
	}
	return def
}

func checkReinit(packFile string) error {
	_, err := os.Stat(packFile)
	if err == nil && !viper.GetBool("init.reinit") {
		return errors.New("modpack metadata file already exists, use -r to override")
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error checking pack file: %s", err)
	}
	return nil
}

func getPackName(cmd *cobra.Command) string {
	name, err := cmd.Flags().GetString("name")
	if err != nil || len(name) == 0 {
		// Get current file directory name
		wd, err := os.Getwd()
		directoryName := "."
		if err == nil {
			directoryName = filepath.Base(wd)
		}
		if directoryName != "." && len(directoryName) > 0 {
			// Turn directory name into a space-seperated proper name
			name = titlecase.Title(strings.ReplaceAll(strings.ReplaceAll(strings.Join(camelcase.Split(directoryName), " "), " - ", " "), " _ ", " "))
			name = initReadValue("Modpack name ["+name+"]: ", name)
		} else {
			name = initReadValue("Modpack name: ", "")
		}
	}

	return name
}

func getAuthorName(cmd *cobra.Command) string {
	author, err := cmd.Flags().GetString("author")
	if err != nil || len(author) == 0 {
		author = initReadValue("Author: ", "")
	}

	return author
}

func getPackVersion(cmd *cobra.Command) string {
	version, err := cmd.Flags().GetString("version")
	if err != nil || len(version) == 0 {
		version = initReadValue("Version [1.0.0]: ", "1.0.0")
	}

	return version
}

func getMcVersion() (string, error) {
	mcVersions, err := shared.GetValidMCVersions()
	if err != nil {
		return "", fmt.Errorf("failed to get latest minecraft versions: %s", err)
	}

	mcVersion := viper.GetString("init.mc-version")
	if len(mcVersion) == 0 {
		var latestVersion string
		if viper.GetBool("init.snapshot") {
			latestVersion = mcVersions.Latest.Snapshot
		} else {
			latestVersion = mcVersions.Latest.Release
		}
		if viper.GetBool("init.latest") {
			mcVersion = latestVersion
		} else {
			mcVersion = initReadValue("Minecraft version ["+latestVersion+"]: ", latestVersion)
		}
	}
	mcVersions.CheckValid(mcVersion)

	return mcVersion, nil
}

func getModLoader(mcVersion string) (core.LoaderInfo, error) {
	modLoaderName := strings.ToLower(viper.GetString("init.modloader"))
	if len(modLoaderName) == 0 {
		modLoaderName = strings.ToLower(initReadValue("ModToml loader [quilt]: ", "quilt"))
	}

	loader, ok := core.ModLoaders[modLoaderName]
	modLoaderVersions := make(core.LoaderInfo)
	if modLoaderName != "none" {
		if ok {
			versions, latestVersion, err := loader.VersionListGetter(mcVersion)
			if err != nil {
				return modLoaderVersions, fmt.Errorf("error loading versions: %s", err)
			}
			componentVersion := viper.GetString("init." + loader.Name + "-version")
			if len(componentVersion) == 0 {
				if viper.GetBool("init." + loader.Name + "-latest") {
					componentVersion = latestVersion
				} else {
					componentVersion = initReadValue(loader.FriendlyName+" version ["+latestVersion+"]: ", latestVersion)
				}
			}
			v := componentVersion
			if loader.Name == "forge" || loader.Name == "neoforge" {
				v = shared.GetRawForgeVersion(componentVersion)
			}
			if !slices.Contains(versions, v) {
				return modLoaderVersions, fmt.Errorf("given %s version cannot be found", loader.FriendlyName)
			}
			modLoaderVersions[loader.Name] = v
		} else {
			fmt.Println("Given mod loader is not supported! Use \"none\" to specify no modloader, or to configure one manually.")
			fmt.Print("The following mod loaders are supported: ")
			keys := make([]string, len(core.ModLoaders))
			i := 0
			for k := range core.ModLoaders {
				keys[i] = k
				i++
			}
			shared.Exitln(strings.Join(keys, ", "))
		}
	}

	return modLoaderVersions, nil
}
