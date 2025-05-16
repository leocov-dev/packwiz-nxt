package cmdmodrinth

import (
	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"errors"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/dixonwille/wmenu.v4"
	"strings"
)

var projectIDFlag string
var versionIDFlag string
var versionFilenameFlag string

func init() {
	modrinthCmd.AddCommand(installCmd)

	installCmd.Flags().StringVar(&projectIDFlag, "project-id", "", "The Modrinth project ID to use")
	installCmd.Flags().StringVar(&versionIDFlag, "version-id", "", "The Modrinth version ID to use")
	installCmd.Flags().StringVar(&versionFilenameFlag, "version-filename", "", "The Modrinth version filename to use")
}

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "add [URL|slug|search]",
	Short:   "Add a project from a Modrinth URL, slug/project ID or search",
	Aliases: []string{"install", "get"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {

		var err error

		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		fmt.Printf("Loading modpack %s\n", packFile)
		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		// ---
		var projectSlug, version, versionID, optionalFilenameMatch string

		// If project/version IDs/version file name is provided in command line, use those
		if projectIDFlag != "" {
			projectSlug = projectIDFlag
			if len(args) != 0 {
				shared.Exitln("--project-id cannot be used with a separately specified URL/slug/search term")
			}
		}
		if versionIDFlag != "" {
			versionID = versionIDFlag
			if len(args) != 0 {
				shared.Exitln("--version-id cannot be used with a separately specified URL/slug/search term")
			}
		}
		if versionFilenameFlag != "" {
			optionalFilenameMatch = versionFilenameFlag
		}

		if (len(args) == 0 || len(args[0]) == 0) && projectSlug == "" {
			shared.Exitln("You must specify a project; with the ID flags, or by passing a URL, slug or search term directly.")
		}

		if projectSlug == "" && versionID == "" && len(args) == 1 {
			if parsedSlug := sources.ParseAsModrinthSlug(args[0]); parsedSlug != "" {
				projectSlug = parsedSlug
			}
			if parsedVersion := sources.ParseAsModrinthVersion(args[0]); parsedVersion != "" {
				version = parsedVersion
			}
			if parsedVersionID := sources.ParseAsModrinthVersionID(args[0]); parsedVersionID != "" {
				versionID = parsedVersionID
			}
			if parsedFilename := sources.ParseAsParseAsFilename(args[0]); parsedFilename != "" {
				optionalFilenameMatch = parsedFilename
			}
		}

		// Got version ID; install using this ID
		if versionID != "" {
			err = installVersionById(versionID, optionalFilenameMatch, pack)
			if err != nil {
				shared.Exitf("Failed to add project: %s\n", err)
			}
			return
		}

		// Look up project ID
		if projectSlug != "" {
			// Modrinth transparently handles slugs/project IDs in their API; we don't have to detect which one it is.
			var project *modrinthApi.Project
			project, err = sources.GetModrinthClient().Projects.Get(projectSlug)
			if err == nil {
				var versionData *modrinthApi.Version
				if version == "" {
					versionData, err = sources.ModrinthGetLatestVersion(*project.ID, *project.Title, *pack, viper.GetString("datapack-folder"))
					if err != nil {
						shared.Exitf("failed to get latest version: %v", err)
					}
				} else {
					versionData, err = sources.ResolveModrinthVersion(project, version)
					if err != nil {
						shared.Exitf("Failed to add project: %s\n", err)
					}
					return
				}

				err = installVersion(project, versionData, optionalFilenameMatch, pack)
				if err != nil {
					shared.Exitf("Failed to add project: %s\n", err)
				}
				return
			}
		} else if len(args) > 0 {
			// Arguments weren't a valid slug/project ID, try to search for it instead
			// (if it was not parsed as a URL)
			err = installViaSearch(strings.Join(args, " "), optionalFilenameMatch, pack)
			if err != nil {
				shared.Exitf("Failed to add project: %s\n", err)
			}
		} else {
			shared.Exitf("Failed to add project: %s\n", err)
		}

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitf("Failed to write pack file: %s\n", err)
		}
		fmt.Printf("Pack file written to %s\n", viper.GetString("pack-file"))
	},
}

func installVersionById(versionId string, optionalFilenameMatch string, pack *core.Pack) error {
	project, version, err := sources.ModrinthProjectFromVersionID(versionId)
	if err != nil {
		return fmt.Errorf("failed to fetch project for versionId %s: %v", versionId, err)
	}

	return installVersion(project, version, optionalFilenameMatch, pack)
}

func installViaSearch(query string, optionalFilenameMatch string, pack *core.Pack) error {
	mcVersions, err := pack.GetSupportedMCVersions()
	if err != nil {
		return err
	}

	fmt.Println("Searching Modrinth...")

	projects, err := sources.ModrinthSearchForProjects(query, mcVersions)
	if err != nil {
		return err
	}

	if viper.GetBool("non-interactive") || (len(projects) == 1 && optionalFilenameMatch != "") {
		// Install the first project found
		return installProject(projects[0], optionalFilenameMatch, pack)
	}

	// Create a menu for the user to choose the correct project
	menu := wmenu.NewMenu("Choose a number:")
	menu.Option("Cancel", nil, false, nil)
	for i, v := range projects {
		// Should be non-nil (Title is a required field)
		menu.Option(*v.Title, v, i == 0, nil)
	}

	menu.Action(func(menuRes []wmenu.Opt) error {
		if len(menuRes) != 1 || menuRes[0].Value == nil {
			return errors.New("project selection cancelled")
		}

		// Get the selected project
		selectedProject, ok := menuRes[0].Value.(*modrinthApi.Project)
		if !ok {
			return errors.New("error converting interface from wmenu")
		}

		return installProject(selectedProject, optionalFilenameMatch, pack)
	})

	return menu.Run()
}

func installProject(project *modrinthApi.Project, optionalFilenameMatch string, pack *core.Pack) error {
	latestVersion, err := sources.ModrinthGetLatestVersion(*project.ID, *project.Title, *pack, viper.GetString("datapack-folder"))
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}

	return installVersion(project, latestVersion, optionalFilenameMatch, pack)
}

func installVersion(project *modrinthApi.Project, version *modrinthApi.Version, optionalFilenameMatch string, pack *core.Pack) error {
	if len(version.Files) == 0 {
		return errors.New("version doesn't have any files attached")
	}

	var missingDependencies []*core.Mod
	if len(version.Dependencies) > 0 {

		missingDependencies, err := sources.ModrinthFindMissingDependencies(version, *pack, viper.GetString("datapack-folder"))
		if err != nil {
			return err
		}

		if len(missingDependencies) > 0 {
			fmt.Println("Dependencies found:")
			for _, v := range missingDependencies {
				fmt.Println(v.Slug)
			}

			if !shared.PromptYesNo("Would you like to add them? [Y/n]: ") {
				// if NO is chosen then we'll nil the slice to prevent installing
				missingDependencies = nil
			}
		}

	}

	mainMod, err := sources.ModrinthNewMod(project, version, viper.GetString("meta-folder"), pack.GetCompatibleLoaders(), optionalFilenameMatch)
	if err != nil {
		return err
	}

	newMods := append(missingDependencies, mainMod)

	if len(newMods) == 0 {
		return errors.New("no mods were installed")
	}

	for _, mod := range newMods {
		pack.SetMod(mod)
	}

	fmt.Printf("Project \"%s\" successfully added! (%s)\n", *project.Title, newMods[len(newMods)-1].Slug)
	return nil
}
