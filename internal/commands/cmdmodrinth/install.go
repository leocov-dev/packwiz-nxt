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

		// If project/version IDs/version file name is provided in command line, use those
		var projectID, versionID, versionFilename string
		if projectIDFlag != "" {
			projectID = projectIDFlag
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
			versionFilename = versionFilenameFlag
		}

		if (len(args) == 0 || len(args[0]) == 0) && projectID == "" {
			shared.Exitln("You must specify a project; with the ID flags, or by passing a URL, slug or search term directly.")
		}

		var err error
		var version string
		var parsedSlug bool
		if projectID == "" && versionID == "" && len(args) == 1 {
			// Try interpreting the argument as a slug/project ID, or project/version/CDN URL
			parsedSlug, err = sources.ParseModrinthSlugOrUrl(args[0], &projectID, &version, &versionID, &versionFilename)
			if err != nil {
				shared.Exitf("Failed to parse URL: %v\n", err)
			}
		}

		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		fmt.Printf("Loading modpack %s\n", packFile)
		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		// Got version ID; install using this ID
		if versionID != "" {
			err = installVersionById(versionID, versionFilename, pack)
			if err != nil {
				shared.Exitf("Failed to add project: %s\n", err)
			}
			return
		}

		// Look up project ID
		if projectID != "" {
			// Modrinth transparently handles slugs/project IDs in their API; we don't have to detect which one it is.
			var project *modrinthApi.Project
			project, err = sources.GetModrinthClient().Projects.Get(projectID)
			if err == nil {
				var versionData *modrinthApi.Version
				if version == "" {
					versionData, err = sources.GetModrinthLatestVersion(*project.ID, *project.Title, *pack)
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

				err = installVersion(project, versionData, versionFilename, pack)
				if err != nil {
					shared.Exitf("Failed to add project: %s\n", err)
				}
				return
			}
		}

		// Arguments weren't a valid slug/project ID, try to search for it instead (if it was not parsed as a URL)
		if projectID == "" || parsedSlug {
			err = installViaSearch(strings.Join(args, " "), versionFilename, !parsedSlug, pack)
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

func installVersionById(versionId string, versionFilename string, pack *core.Pack) error {
	version, err := sources.GetModrinthClient().Versions.Get(versionId)
	if err != nil {
		return fmt.Errorf("failed to fetch version %s: %v", versionId, err)
	}

	project, err := sources.GetModrinthClient().Projects.Get(*version.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to fetch project %s: %v", *version.ProjectID, err)
	}

	return installVersion(project, version, versionFilename, pack)
}

func installViaSearch(query string, versionFilename string, autoAcceptFirst bool, pack *core.Pack) error {
	mcVersions, err := pack.GetSupportedMCVersions()
	if err != nil {
		return err
	}

	fmt.Println("Searching Modrinth...")

	results, err := sources.GetModrinthProjectIdsViaSearch(query, mcVersions)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return errors.New("no projects found")
	}

	if viper.GetBool("non-interactive") || (len(results) == 1 && autoAcceptFirst) {
		// Install the first project found
		project, err := sources.GetModrinthClient().Projects.Get(*results[0].ProjectID)
		if err != nil {
			return err
		}

		return installProject(project, versionFilename, pack)
	}

	// Create menu for the user to choose the correct project
	menu := wmenu.NewMenu("Choose a number:")
	menu.Option("Cancel", nil, false, nil)
	for i, v := range results {
		// Should be non-nil (Title is a required field)
		menu.Option(*v.Title, v, i == 0, nil)
	}

	menu.Action(func(menuRes []wmenu.Opt) error {
		if len(menuRes) != 1 || menuRes[0].Value == nil {
			return errors.New("project selection cancelled")
		}

		// Get the selected project
		selectedProject, ok := menuRes[0].Value.(*modrinthApi.SearchResult)
		if !ok {
			return errors.New("error converting interface from wmenu")
		}

		// Install the selected project
		project, err := sources.GetModrinthClient().Projects.Get(*selectedProject.ProjectID)
		if err != nil {
			return err
		}

		return installProject(project, versionFilename, pack)
	})

	return menu.Run()
}

func installProject(project *modrinthApi.Project, versionFilename string, pack *core.Pack) error {
	latestVersion, err := sources.GetModrinthLatestVersion(*project.ID, *project.Title, *pack)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}

	return installVersion(project, latestVersion, versionFilename, pack)
}

func installVersion(project *modrinthApi.Project, version *modrinthApi.Version, versionFilename string, pack *core.Pack) error {
	if len(version.Files) == 0 {
		return errors.New("version doesn't have any files attached")
	}

	if len(version.Dependencies) > 0 {

		mods := make([]*core.Mod, 1)
		for _, v := range pack.Mods {
			mods = append(mods, v)
		}

		missingDependencies, err := sources.GetModrinthModMissingDependencies(version, *pack, mods)
		if err != nil {
			return err
		}
		if len(missingDependencies) > 0 {
			if err = maybeInstallDependencies(missingDependencies, pack); err != nil {
				return err
			}
		}
	}

	var file = sources.GetModrinthVersionPrimaryFile(version, versionFilename)

	// TODO: handle optional/required resource pack files

	// Create the metadata file
	mod, err := sources.CreateModrinthMod(project, version, file, pack, viper.GetString("meta-folder"))
	if err != nil {
		return err
	}

	pack.SetMod(mod)

	fmt.Printf("Project \"%s\" successfully added! (%s)\n", *project.Title, *file.Filename)
	return nil
}

func maybeInstallDependencies(
	depMetadata []sources.ModrinthDepMetadataStore,
	pack *core.Pack,
) error {
	if shared.PromptYesNo("Would you like to add them? [Y/n]: ") {
		for _, v := range depMetadata {
			mod, err := sources.CreateModrinthMod(v.ProjectInfo, v.VersionInfo, v.FileInfo, pack, viper.GetString("meta-folder"))
			if err != nil {
				return err
			}

			pack.SetMod(mod)

			fmt.Printf("Dependency \"%s\" successfully added! (%s)\n", *v.ProjectInfo.Title, *v.FileInfo.Filename)
		}
	}

	return nil
}
