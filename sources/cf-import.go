package sources

import (
	"fmt"
	"path/filepath"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/commands/cmdcurseforge/packinterop"
	"github.com/spf13/viper"
)

// CurseforgeImportPack creates or updates the pack in packDir from CurseForge
// pack metadata (a parsed manifest.json/minecraftinstance.json), resolving
// project/file info for every referenced mod via the CurseForge API and
// writing a .pw.toml for each. It returns the resulting pack (not yet
// written to disk) along with the absolute paths of files covered by the
// resolved mod metadata, so callers can avoid re-copying them as overrides.
func CurseforgeImportPack(packImport packinterop.ImportPackMetadata, packFile string, packDir string) (*core.Pack, []string, error) {
	pack, err := fileio.LoadAll(packFile)
	if err != nil {
		fmt.Println("Failed to load existing pack, creating a new one...")

		pack = core.NewPack(
			packImport.Name(),
			packImport.PackAuthor(),
			packImport.PackVersion(),
			"",
			packImport.Versions()["minecraft"],
			packImport.Versions(),
		)
	} else {
		for component, version := range packImport.Versions() {
			packVersion, ok := pack.Versions[component]
			if !ok {
				fmt.Println("Set " + core.ComponentToFriendlyName(component) + " version to " + version)
			} else if packVersion != version {
				fmt.Println("Set " + core.ComponentToFriendlyName(component) + " version to " + version + " (previously " + packVersion + ")")
			}
			pack.Versions[component] = version
		}
	}

	modsList := packImport.Mods()
	modIDs := make([]uint32, len(modsList))
	for i, v := range modsList {
		modIDs[i] = v.ProjectID
	}

	fmt.Println("Querying Curse API for dependency info...")

	modInfos, err := GetCurseforgeClient().GetModInfoMultiple(modIDs)
	if err != nil {
		return pack, nil, fmt.Errorf("Failed to obtain project information: %w", err)
	}

	modInfosMap := make(map[uint32]CfModInfo)
	for _, v := range modInfos {
		modInfosMap[v.ID] = v
	}

	// TODO: multithreading????

	modFileInfosMap := make(map[uint32]CfModFileInfo)
	referencedModPaths := make([]string, 0, len(modsList))
	successes := 0
	remainingFileIDs := make([]uint32, 0, len(modsList))

	// 1st pass: query mod metadata for every CurseForge file
	for _, v := range modsList {
		modInfoValue, ok := modInfosMap[v.ProjectID]
		if !ok {
			fmt.Printf("Failed to obtain information for project/file IDs %d/%d\n", v.ProjectID, v.FileID)
			continue
		}

		found := false
		var fileInfo CfModFileInfo
		for _, fileInfo = range modInfoValue.LatestFiles {
			if fileInfo.ID == v.FileID {
				found = true
				break
			}
		}
		if found {
			modFileInfosMap[v.FileID] = fileInfo
		} else {
			remainingFileIDs = append(remainingFileIDs, v.FileID)
		}
	}

	// 2nd pass: query files that weren't in the previous results
	fmt.Println("Querying Curse API for file info...")

	modFileInfos, err := GetCurseforgeClient().GetFileInfoMultiple(remainingFileIDs)
	if err != nil {
		return pack, nil, fmt.Errorf("Failed to obtain project file information: %w", err)
	}

	for _, v := range modFileInfos {
		modFileInfosMap[v.ID] = v
	}

	// 3rd pass: create mod files for every file
	for _, v := range modsList {
		modInfoValue, ok := modInfosMap[v.ProjectID]
		if !ok {
			fmt.Printf("Failed to obtain project information for project/file IDs %d/%d\n", v.ProjectID, v.FileID)
			continue
		}

		modFileInfoValue, ok := modFileInfosMap[v.FileID]
		if !ok {
			fmt.Printf("Failed to obtain project file information for project/file IDs %d/%d\n", v.ProjectID, v.FileID)
			continue
		}

		mod, err := CurseforgeNewMod(modInfoValue, modFileInfoValue, v.OptionalDisabled)
		if err != nil {
			return pack, nil, fmt.Errorf("Failed to save project \"%s\": %w", modInfoValue.Name, err)
		}

		pack.SetMod(mod)

		modFilePath := curseforgeMetaPathForFile(modInfoValue.GameID, modInfoValue.ClassID, modInfoValue.PrimaryCategoryID, modInfoValue.Slug)
		ref, err := filepath.Abs(filepath.Join(filepath.Dir(modFilePath), modFileInfoValue.FileName))
		if err == nil {
			referencedModPaths = append(referencedModPaths, ref)
		}

		fmt.Printf("Imported dependency \"%s\" successfully!\n", modInfoValue.Name)
		successes++
	}

	fmt.Printf("Successfully imported %d/%d dependencies!\n", successes, len(modsList))

	return pack, referencedModPaths, nil
}

// curseforgeMetaPathForFile returns the .pw.toml metadata file path that
// would be used for a mod, matching the layout used when adding CurseForge
// mods elsewhere (see the `cf add` command).
func curseforgeMetaPathForFile(gameID uint32, classID uint32, categoryID uint32, slug string) string {
	metaFolder := viper.GetString("meta-folder")
	if metaFolder == "" {
		metaFolder = GetCfModType(gameID, classID, categoryID)
	}
	return filepath.Join(viper.GetString("meta-folder-base"), metaFolder, slug+core.MetaExtension)
}
