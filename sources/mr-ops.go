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
	additionalDependencies []ModrinthDepMetadataStore,
	optionalFilenameMatch string,
) ([]*core.Mod, error) {

	var mods []*core.Mod
	var err error

	if len(additionalDependencies) > 0 {
		mods, err = CreateModrinthDependencies(compatibleLoaders, additionalDependencies)
		if err != nil {
			return nil, err
		}
	} else {
		mods = make([]*core.Mod, 0)
	}

	primaryFile := GetModrinthVersionPrimaryFile(version, optionalFilenameMatch)

	mod, err := CreateModrinthMod(project, version, primaryFile, compatibleLoaders, modType)
	if err != nil {
		return nil, err
	}

	mods = append(mods, mod)

	return mods, nil
}

const mrMaxCycles = 20

type ModrinthDepMetadataStore struct {
	ProjectInfo *modrinthApi.Project
	VersionInfo *modrinthApi.Version
	FileInfo    *modrinthApi.File
}

func ModrinthFindMissingDependencies(
	version *modrinthApi.Version,
	pack core.Pack,
	optionalDatapackFolder string,
) ([]ModrinthDepMetadataStore, error) {
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

		cycles := 0
		for len(depProjectIDPendingQueue)+len(depVersionIDPendingQueue) > 0 && cycles < mrMaxCycles {
			// Look up version IDs
			if len(depVersionIDPendingQueue) > 0 {
				depVersions, err := GetModrinthClient().Versions.GetMultiple(depVersionIDPendingQueue)
				if err == nil {
					for _, v := range depVersions {
						// Add project ID to queue
						depProjectIDPendingQueue = append(depProjectIDPendingQueue, mrMapDepOverride(*v.ProjectID, isQuilt, mcVersion))
					}
				} else {
					fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
				}
				depVersionIDPendingQueue = depVersionIDPendingQueue[:0]
			}

			// Remove installed project IDs from dep queue
			i := 0
			for _, id := range depProjectIDPendingQueue {
				contains := slices.Contains(installedProjects, id)
				for _, dep := range depMetadata {
					if *dep.ProjectInfo.ID == id {
						contains = true
						break
					}
				}
				if !contains {
					depProjectIDPendingQueue[i] = id
					i++
				}
			}
			depProjectIDPendingQueue = depProjectIDPendingQueue[:i]

			// Clean up duplicates from dep queue (from deps on both QFAPI + FAPI)
			slices.Sort(depProjectIDPendingQueue)
			depProjectIDPendingQueue = slices.Compact(depProjectIDPendingQueue)

			if len(depProjectIDPendingQueue) == 0 {
				break
			}
			depProjects, err := GetModrinthClient().Projects.GetMultiple(depProjectIDPendingQueue)
			if err != nil {
				fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
			}
			depProjectIDPendingQueue = depProjectIDPendingQueue[:0]

			for _, project := range depProjects {
				if project.ID == nil {
					return nil, errors.New("failed to get dependency data: invalid response")
				}
				// Get latest version - could reuse version lookup data but it's not as easy (particularly since the version won't necessarily be the latest)
				latestVersion, err := ModrinthGetLatestVersion(*project.ID, *project.Title, pack, optionalDatapackFolder)
				if err != nil {
					fmt.Printf("Failed to get latest version of dependency %v: %v\n", *project.Title, err)
					continue
				}

				for _, dep := range version.Dependencies {
					// TODO: recommend optional dependencies?
					if dep.DependencyType != nil && *dep.DependencyType == "required" {
						if dep.ProjectID != nil {
							depProjectIDPendingQueue = append(depProjectIDPendingQueue, mrMapDepOverride(*dep.ProjectID, isQuilt, mcVersion))
						}
						if dep.VersionID != nil {
							depVersionIDPendingQueue = append(depVersionIDPendingQueue, *dep.VersionID)
						}
					}
				}

				var file = latestVersion.Files[0]
				// Prefer the primary file
				for _, v := range latestVersion.Files {
					if *v.Primary {
						file = v
					}
				}

				depMetadata = append(depMetadata, ModrinthDepMetadataStore{
					ProjectInfo: project,
					VersionInfo: latestVersion,
					FileInfo:    file,
				})
			}

			cycles++
		}
		if cycles >= mrMaxCycles {
			return nil, errors.New("dependencies recurse too deeply, try increasing mrMaxCycles")
		}
	}

	if len(depMetadata) > 0 {
		fmt.Println("Dependencies found:")
		for _, v := range depMetadata {
			fmt.Println(*v.ProjectInfo.Title)
		}
		return depMetadata, nil
	}

	return nil, nil
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

func CreateModrinthMod(
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
		GetModrinthProjectSlug(project),
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

func GetModrinthProjectSlug(project *modrinthApi.Project) string {
	if project.Slug != nil {
		return *project.Slug
	}
	return core.SlugifyName(*project.Title)
}

func CreateModrinthDependencies(
	compatibleLoaders []string,
	depMetadata []ModrinthDepMetadataStore,
) ([]*core.Mod, error) {
	mods := make([]*core.Mod, 0)

	for _, v := range depMetadata {
		mod, err := CreateModrinthMod(v.ProjectInfo, v.VersionInfo, v.FileInfo, compatibleLoaders, "")
		if err != nil {
			return nil, err
		}

		mods = append(mods, mod)
	}

	return mods, nil
}
