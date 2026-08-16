package sources

import (
	"errors"
	"fmt"
	"golang.org/x/exp/slices"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func ModrinthNewMod(
	project *modrinthApi.Project,
	version *modrinthApi.Version,
	modType string,
	compatibleLoaders []string,
	optionalFilenameMatch string,
) (*core.Mod, error) {

	var err error

	primaryFile := GetModrinthVersionPrimaryFile(version, optionalFilenameMatch)

	mod, err := createModrinthMod(project, version, primaryFile, compatibleLoaders, modType)
	if err != nil {
		return nil, err
	}

	return mod, nil
}

type ModrinthDepMetadataStore struct {
	ProjectInfo *modrinthApi.Project
	VersionInfo *modrinthApi.Version
	FileInfo    *modrinthApi.File
}

func ModrinthFindMissingDependencies(
	version *modrinthApi.Version,
	pack core.Pack,
	optionalDatapackFolder string,
) ([]*core.Mod, error) {
	// TODO: could get installed version IDs, and compare to install the newest - i.e. preferring pinned versions over getting absolute latest?
	installedProjects := mrGetInstalledProjectIDs(pack.GetModsList())
	isQuilt := slices.Contains(pack.GetCompatibleLoaders(), "quilt")
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return nil, err
	}

	var depMetadata []ModrinthDepMetadataStore
	var depProjectIDPendingQueue []string
	var depVersionIDPendingQueue []string

	for _, dep := range version.Dependencies {
		// TODO: recommend optional dependencies?
		if dep.DependencyType != nil && *dep.DependencyType == "required" {
			if dep.VersionID != nil {
				depVersionIDPendingQueue = append(depVersionIDPendingQueue, *dep.VersionID)
			} else {
				if dep.ProjectID != nil {
					depProjectIDPendingQueue = append(depProjectIDPendingQueue, mrMapDepOverride(*dep.ProjectID, isQuilt, mcVersion))
				}
			}
		}
	}

	if len(depProjectIDPendingQueue)+len(depVersionIDPendingQueue) > 0 {
		fmt.Println("Finding dependencies...")

		// prepareNext folds in the two bits of provider-specific bookkeeping that happen at
		// the top of each resolution cycle: resolving any queued version IDs into project
		// IDs (Modrinth dependencies may be expressed as either), then deduping the merged
		// project ID batch against IDs already installed in the pack or already collected as
		// a dependency this run.
		prepareNext := func(newProjectIDs []string) ([]string, error) {
			pending := append([]string{}, newProjectIDs...)

			if len(depVersionIDPendingQueue) > 0 {
				depVersions, err := GetModrinthClient().Versions.GetMultiple(depVersionIDPendingQueue)
				if err != nil {
					return nil, fmt.Errorf("error retrieving dependency data: %w", err)
				}
				for _, v := range depVersions {
					pending = append(pending, mrMapDepOverride(*v.ProjectID, isQuilt, mcVersion))
				}
				depVersionIDPendingQueue = depVersionIDPendingQueue[:0]
			}

			// Remove installed project IDs from dep queue
			i := 0
			for _, id := range pending {
				contains := slices.Contains(installedProjects, id)
				for _, dep := range depMetadata {
					if *dep.ProjectInfo.ID == id {
						contains = true
						break
					}
				}
				if !contains {
					pending[i] = id
					i++
				}
			}
			pending = pending[:i]

			// Clean up duplicates from dep queue (from deps on both QFAPI + FAPI)
			slices.Sort(pending)
			pending = slices.Compact(pending)

			return pending, nil
		}

		// fetchAndExpand batch-fetches project data for the pending project IDs, resolves
		// the latest compatible version of each, and returns the further dependency project
		// IDs discovered along the way (any dependency version IDs are queued directly via
		// depVersionIDPendingQueue, to be resolved by prepareNext on the next cycle).
		fetchAndExpand := func(pending []string) ([]string, error) {
			depProjects, err := GetModrinthClient().Projects.GetMultiple(pending)
			if err != nil {
				return nil, fmt.Errorf("error retrieving dependency data: %w", err)
			}

			var next []string
			for _, project := range depProjects {
				if project.ID == nil {
					return nil, errors.New("failed to get dependency data: invalid response")
				}
				// Get latest version - could reuse version lookup data but it's not as easy (particularly since the version won't necessarily be the latest)
				latestVersion, err := ModrinthGetLatestVersion(*project.ID, *project.Title, pack, optionalDatapackFolder)
				if err != nil {
					return nil, fmt.Errorf("failed to get latest version of dependency %v: %w", *project.Title, err)
				}

				for _, dep := range version.Dependencies {
					// TODO: recommend optional dependencies?
					if dep.DependencyType != nil && *dep.DependencyType == "required" {
						if dep.ProjectID != nil {
							next = append(next, mrMapDepOverride(*dep.ProjectID, isQuilt, mcVersion))
						}
						if dep.VersionID != nil {
							depVersionIDPendingQueue = append(depVersionIDPendingQueue, *dep.VersionID)
						}
					}
				}

				file := GetModrinthVersionPrimaryFile(latestVersion, "")

				depMetadata = append(depMetadata, ModrinthDepMetadataStore{
					ProjectInfo: project,
					VersionInfo: latestVersion,
					FileInfo:    file,
				})
			}

			return next, nil
		}

		if err := runDependencyResolution(depProjectIDPendingQueue, DefaultMaxDependencyCycles, prepareNext, fetchAndExpand); err != nil {
			return nil, err
		}
	}

	mods, err := createModrinthDependencies(pack.GetCompatibleLoaders(), depMetadata)
	if err != nil {
		return nil, err
	}

	return mods, nil
}

func GetModrinthVersionPrimaryFile(
	version *modrinthApi.Version,
	optionalFilenameMatch string,
) *modrinthApi.File {
	var file = version.Files[0]
	// Prefer the primary file
	for _, v := range version.Files {
		if (*v.Primary) || (optionalFilenameMatch != "" && optionalFilenameMatch == *v.Filename) {
			file = v
		}
	}

	return file
}

func createModrinthMod(
	project *modrinthApi.Project,
	version *modrinthApi.Version,
	file *modrinthApi.File,
	compatibleLoaders []string,
	customMetaFolder string,
) (*core.Mod, error) {
	updateMap := make(core.ModUpdate)

	var err error
	metaFolder := customMetaFolder
	if metaFolder == "" {
		metaFolder, err = mrGetProjectTypeFolder(*project.ProjectType, version.Loaders, compatibleLoaders)
		if err != nil {
			return nil, err
		}
	}

	updateMap["modrinth"], err = mrUpdateData{
		ProjectID:        *project.ID,
		InstalledVersion: *version.ID,
	}.ToMap()
	if err != nil {
		return nil, err
	}

	side := mrGetSide(project)
	if side == core.EmptySide {
		return nil, errors.New("version doesn't have a side that's supported. Server: " + *project.ServerSide + " Client: " + *project.ClientSide)
	}

	algorithm, hash := mrGetBestHash(file)
	if algorithm == "" {
		return nil, errors.New("file doesn't have a hash")
	}

	download := core.ModDownload{
		URL:        *file.URL,
		HashFormat: algorithm,
		Hash:       hash,
	}

	mod := core.NewMod(
		getModrinthProjectSlug(project),
		*project.Title,
		*file.Filename,
		side,
		metaFolder,
		"",
		false,
		false,
		updateMap,
		download,
		nil,
	)

	return mod, nil
}

func getModrinthProjectSlug(project *modrinthApi.Project) string {
	if project.Slug != nil {
		return *project.Slug
	}
	return core.SlugifyName(*project.Title)
}

func createModrinthDependencies(
	compatibleLoaders []string,
	depMetadata []ModrinthDepMetadataStore,
) ([]*core.Mod, error) {
	mods := make([]*core.Mod, 0)

	for _, v := range depMetadata {
		mod, err := createModrinthMod(v.ProjectInfo, v.VersionInfo, v.FileInfo, compatibleLoaders, "")
		if err != nil {
			return nil, err
		}

		mods = append(mods, mod)
	}

	return mods, nil
}
