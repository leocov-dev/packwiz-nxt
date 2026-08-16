package sources

import (
	"errors"
	"fmt"
	"golang.org/x/exp/slices"

	"github.com/leocov-dev/packwiz-nxt/core"
)

type CfInstallableDep struct {
	CfModInfo
	FileInfo CfModFileInfo
}

func CurseforgeFindMissingDependencies(
	pack core.Pack,
	fileInfoData CfModFileInfo,
	primaryMCVersion string,
) ([]*core.Mod, error) {
	var depsInstallable []CfInstallableDep

	isQuilt := slices.Contains(pack.GetCompatibleLoaders(), "quilt")

	depIDPendingQueue := buildDependencyQueue(fileInfoData, primaryMCVersion, isQuilt)

	mcVersions, err := pack.GetSupportedMCVersions()
	if err != nil {
		return nil, err
	}

	if len(depIDPendingQueue) > 0 {
		GetCurseforgeClient().logger.Infof("Finding dependencies...\n")

		installedIDList, err := buildInstalledIdList(pack)
		if err != nil {
			return nil, err
		}

		// prepareNext dedupes a batch of candidate dependency IDs against IDs that are
		// already installed in the pack, or already collected as a dependency this run.
		prepareNext := func(newIDs []uint32) ([]uint32, error) {
			var out []uint32
			for _, id := range newIDs {
				if slices.Contains(installedIDList, id) {
					continue
				}
				installed := false
				for _, data := range depsInstallable {
					if id == data.ID {
						installed = true
						break
					}
				}
				if !installed {
					out = append(out, id)
				}
			}
			return out, nil
		}

		// fetchAndExpand batch-fetches mod info for the pending IDs, resolves the latest
		// compatible file for each, and returns the required-dependency IDs discovered
		// along the way (after applying the Quilt dependency overrides).
		fetchAndExpand := func(pending []uint32) ([]uint32, error) {
			depInfoData, err := GetCurseforgeClient().GetModInfoMultiple(pending)
			if err != nil {
				return nil, err
			}

			var next []uint32
			for _, currData := range depInfoData {
				depFileInfo, err := GetLatestFile(currData, mcVersions, 0, pack.GetCompatibleLoaders())
				if err != nil {
					return nil, err
				}

				for _, dep := range depFileInfo.Dependencies {
					if dep.Type == DependencyTypeRequired {
						next = append(next, MapDepOverride(dep.ModID, isQuilt, primaryMCVersion))
					}
				}

				depsInstallable = append(
					depsInstallable,
					CfInstallableDep{
						currData,
						depFileInfo,
					},
				)
			}

			return next, nil
		}

		if err := runDependencyResolution(depIDPendingQueue, DefaultMaxDependencyCycles, prepareNext, fetchAndExpand); err != nil {
			return nil, err
		}
	}

	mods, err := CreateCurseforgeDependencies(depsInstallable)
	if err != nil {
		return nil, err
	}

	return mods, nil
}

func GetLatestFile(modInfoData CfModInfo, mcVersions []string, fileID uint32, packLoaders []string) (CfModFileInfo, error) {
	if fileID == 0 {
		if len(modInfoData.LatestFiles) == 0 && len(modInfoData.GameVersionLatestFiles) == 0 {
			return CfModFileInfo{}, fmt.Errorf("addon %d has no files", modInfoData.ID)
		}

		var fileInfoData *CfModFileInfo
		fileID, fileInfoData, _ = CfFindLatestFile(modInfoData, mcVersions, packLoaders)
		if fileInfoData != nil {
			return *fileInfoData, nil
		}

		// Possible to reach this point without obtaining file info; particularly from GameVersionLatestFiles
		if fileID == 0 {
			return CfModFileInfo{}, errors.New("mod not available for the configured Minecraft version(s) (use the 'packwiz settings acceptable-versions' command to accept more) or loader")
		}
	}

	fileInfoData, err := GetCurseforgeClient().GetFileInfo(modInfoData.ID, fileID)
	if err != nil {
		return CfModFileInfo{}, err
	}
	return fileInfoData, nil
}

func buildDependencyQueue(
	fileInfoData CfModFileInfo,
	primaryMCVersion string,
	isQuilt bool,
) []uint32 {
	var depIDPendingQueue []uint32

	for _, dep := range fileInfoData.Dependencies {
		if dep.Type == DependencyTypeRequired {
			depIDPendingQueue = append(
				depIDPendingQueue,
				MapDepOverride(dep.ModID, isQuilt, primaryMCVersion),
			)
		}
	}

	return depIDPendingQueue
}

func buildInstalledIdList(pack core.Pack) ([]uint32, error) {
	var installedIDList []uint32
	for _, mod := range pack.GetModsList() {
		var updateData CfExportData
		err := mod.DecodeNamedModSourceData("curseforge", updateData)
		if err != nil {
			return nil, err
		}

		if updateData.ProjectID > 0 {
			installedIDList = append(installedIDList, updateData.ProjectID)
		}
	}

	return installedIDList, nil
}

func CreateCurseforgeDependencies(depsInstallable []CfInstallableDep) ([]*core.Mod, error) {
	var mods []*core.Mod

	for _, v := range depsInstallable {
		mod, err := CurseforgeNewMod(v.CfModInfo, v.FileInfo, false)
		if err != nil {
			return nil, err
		}
		mods = append(mods, mod)
	}

	return mods, nil
}

func CurseforgeModInfoFromID(
	modID uint32,
	fileID uint32,
	mcVersions []string,
	packLoaders []string,
) (CfModInfo, CfModFileInfo, error) {
	modInfo, err := GetCurseforgeClient().GetModInfo(modID)
	if err != nil {
		return CfModInfo{}, CfModFileInfo{}, err
	}

	fileInfo, err := GetLatestFile(modInfo, mcVersions, fileID, packLoaders)
	if err != nil {
		return CfModInfo{}, CfModFileInfo{}, err
	}

	return modInfo, fileInfo, nil
}

func CurseforgeNewMod(modInfo CfModInfo, fileInfo CfModFileInfo, optionalDisabled bool) (*core.Mod, error) {
	updateMap := make(core.ModUpdate)
	var err error

	updateMap["curseforge"], err = CfUpdateData{
		ProjectID: modInfo.ID,
		FileID:    fileInfo.ID,
	}.ToMap()
	if err != nil {
		return nil, err
	}

	hash, hashFormat := fileInfo.GetBestHash()

	var optional *core.ModOption
	if optionalDisabled {
		optional = &core.ModOption{
			Optional: true,
			Default:  false,
		}
	}

	return core.NewMod(
		modInfo.Slug,
		modInfo.Name,
		fileInfo.FileName,
		core.UniversalSide,
		GetCfModType(modInfo.GameID, modInfo.ClassID, modInfo.PrimaryCategoryID),
		"",
		false,
		false,
		updateMap,
		core.ModDownload{
			HashFormat: hashFormat,
			Hash:       hash,
			Mode:       core.ModeCF,
		},
		optional,
	), nil
}

func CurseforgeModInfoFromSlug(
	slug string,
	category string,
	fileID uint32,
	mcVersions []string,
	searchLoaderType ModloaderType,
	packLoaders []string,
) (CfModInfo, CfModFileInfo, error) {
	if category == "" {
		return CfModInfo{}, CfModFileInfo{}, errors.New("must supply a category")
	}

	categoryID, classID, err := CurseforgeCategoryLookup(category)
	if err != nil {
		return CfModInfo{}, CfModFileInfo{}, err
	}

	var filterGameVersion string
	if len(mcVersions) == 1 {
		filterGameVersion = GetCurseforgeVersion(mcVersions[0])
	}

	results, err := GetCurseforgeClient().GetSearch("", slug, classID, categoryID, filterGameVersion, searchLoaderType)
	if err != nil || len(results) == 0 {
		return CfModInfo{}, CfModFileInfo{}, err
	}

	// TODO: do we need to fuzzy search by slug as well?
	modInfo := results[0]

	fileInfo, err := GetLatestFile(modInfo, mcVersions, fileID, packLoaders)
	if err != nil {
		return CfModInfo{}, CfModFileInfo{}, err
	}

	return modInfo, fileInfo, nil
}

func CurseforgeCategoryLookup(category string) (uint32, uint32, error) {
	var categoryID, classID uint32

	categories, err := GetCurseforgeClient().GetCategories()
	if err != nil {
		return 0, 0, err
	}
	for _, v := range categories {
		if v.Slug == category {
			if v.IsClass {
				classID = v.ID
			} else {
				classID = v.ClassID
				categoryID = v.ID
			}
			break
		}
	}
	if categoryID == 0 && classID == 0 {
		return 0, 0, fmt.Errorf("failed to lookup category '%s' could not be found", category)
	}

	return categoryID, classID, nil
}
