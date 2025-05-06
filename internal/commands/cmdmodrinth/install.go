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
	"path/filepath"
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

		pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			shared.Exitln(err)
		}

		index, err := fileio.LoadPackIndexFile(&pack)
		if err != nil {
			shared.Exitln(err)
		}

		// Got version ID; install using this ID
		if versionID != "" {
			err = installVersionById(versionID, versionFilename, pack, &index)
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
					versionData, err = sources.GetModrinthLatestVersion(*project.ID, *project.Title, pack)
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

				err = installVersion(project, versionData, versionFilename, pack, &index)
				if err != nil {
					shared.Exitf("Failed to add project: %s\n", err)
				}
				return
			}
		}

		// Arguments weren't a valid slug/project ID, try to search for it instead (if it was not parsed as a URL)
		if projectID == "" || parsedSlug {
			err = installViaSearch(strings.Join(args, " "), versionFilename, !parsedSlug, pack, &index)
			if err != nil {
				shared.Exitf("Failed to add project: %s\n", err)
			}
		} else {
			shared.Exitf("Failed to add project: %s\n", err)
		}
	},
}

func installVersionById(versionId string, versionFilename string, pack core.PackToml, index *core.IndexFS) error {
	version, err := sources.GetModrinthClient().Versions.Get(versionId)
	if err != nil {
		return fmt.Errorf("failed to fetch version %s: %v", versionId, err)
	}

	project, err := sources.GetModrinthClient().Projects.Get(*version.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to fetch project %s: %v", *version.ProjectID, err)
	}

	return installVersion(project, version, versionFilename, pack, index)
}

func installViaSearch(query string, versionFilename string, autoAcceptFirst bool, pack core.PackToml, index *core.IndexFS) error {
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

		return installProject(project, versionFilename, pack, index)
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

		return installProject(project, versionFilename, pack, index)
	})

	return menu.Run()
}

func installProject(project *modrinthApi.Project, versionFilename string, pack core.PackToml, index *core.IndexFS) error {
	latestVersion, err := sources.GetModrinthLatestVersion(*project.ID, *project.Title, pack)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}

	return installVersion(project, latestVersion, versionFilename, pack, index)
}

func installVersion(project *modrinthApi.Project, version *modrinthApi.Version, versionFilename string, pack core.PackToml, index *core.IndexFS) error {
	if len(version.Files) == 0 {
		return errors.New("version doesn't have any files attached")
	}

	if len(version.Dependencies) > 0 {
		mods, err := fileio.LoadAllMods(index)
		if err != nil {
			return err
		}

		missingDependencies, err := sources.GetModrinthModMissingDependencies(version, pack, mods)
		if err != nil {
			return err
		}
		if len(missingDependencies) > 0 {
			if err = maybeInstallDependencies(missingDependencies, pack, index); err != nil {
				return err
			}
		}
	}

	var file = sources.GetModrinthVersionPrimaryFile(version, versionFilename)

	// TODO: handle optional/required resource pack files

	// Create the metadata file
	modMeta, err := createFileMeta(project, version, file, pack)
	if err != nil {
		return err
	}
	err = writeModFile(modMeta, index, project)
	if err != nil {
		return err
	}

	repr := index.ToWritable()
	writer := fileio.NewIndexWriter()
	err = writer.Write(&repr)
	if err != nil {
		return err
	}

	pack.RefreshIndexHash(*index)

	packWriter := fileio.NewPackWriter()
	err = packWriter.Write(&pack)
	if err != nil {
		return err
	}

	fmt.Printf("Project \"%s\" successfully added! (%s)\n", *project.Title, *file.Filename)
	return nil
}

func maybeInstallDependencies(
	depMetadata []sources.ModrinthDepMetadataStore,
	pack core.PackToml,
	index *core.IndexFS,
) error {
	if shared.PromptYesNo("Would you like to add them? [Y/n]: ") {
		for _, v := range depMetadata {
			modMeta, err := createFileMeta(v.ProjectInfo, v.VersionInfo, v.FileInfo, pack)
			if err != nil {
				return err
			}

			err = writeModFile(modMeta, index, v.ProjectInfo)
			if err != nil {
				return err
			}

			fmt.Printf("Dependency \"%s\" successfully added! (%s)\n", *v.ProjectInfo.Title, *v.FileInfo.Filename)
		}
	}

	return nil
}

func createFileMeta(
	project *modrinthApi.Project,
	version *modrinthApi.Version,
	file *modrinthApi.File,
	pack core.PackToml,
) (core.ModToml, error) {

	modMeta, err := sources.CreateModrinthMod(
		project, version, file, pack, viper.GetString("meta-folder"))
	if err != nil {
		return core.ModToml{}, err
	}
	return modMeta, err
}

func writeModFile(modMeta core.ModToml, index *core.IndexFS, project *modrinthApi.Project) error {
	path := modMeta.SetMetaPath(
		filepath.Join(
			viper.GetString("meta-folder-base"),
			modMeta.GetMetaFolder(),
			sources.GetModrinthProjectSlug(project)+core.MetaExtension,
		),
	)

	// If the file already exists, this will overwrite it!!!
	// TODO: Should this be improved?
	// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
	// VCS anyway.

	modWriter := fileio.NewModWriter()
	format, hash, err := modWriter.Write(&modMeta)
	if err != nil {
		return err
	}

	return index.UpdateFileHashGiven(path, format, hash, true)
}
