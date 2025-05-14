package sources

import (
	"errors"
	"fmt"
	"golang.org/x/exp/slices"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/unascribed/FlexVer/go/flexver"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func init() {
	core.AddUpdater(CfUpdater{})
	core.MetaDownloaders["curseforge"] = CfDownloader{}
}

var snapshotVersionRegex = regexp.MustCompile(`(?:Snapshot )?(\d+)w0?(0|[1-9]\d*)([a-z])`)

var snapshotNames = [...]string{"-pre", " Pre-Release ", " Pre-release ", "-rc"}

func GetCurseforgeVersion(mcVersion string) string {
	for _, name := range snapshotNames {
		index := strings.Index(mcVersion, name)
		if index > -1 {
			return mcVersion[:index] + "-Snapshot"
		}
	}

	matches := snapshotVersionRegex.FindStringSubmatch(mcVersion)
	if matches == nil {
		return mcVersion
	}
	year, err := strconv.Atoi(matches[1])
	if err != nil {
		return mcVersion
	}
	week, err := strconv.Atoi(matches[2])
	if err != nil {
		return mcVersion
	}

	if year >= 22 && week >= 11 {
		return "1.19-Snapshot"
	} else if year == 21 && week >= 37 || year >= 22 {
		return "1.18-Snapshot"
	} else if year == 20 && week >= 45 || year == 21 && week <= 20 {
		return "1.17-Snapshot"
	} else if year == 20 && week >= 6 {
		return "1.16-Snapshot"
	} else if year == 19 && week >= 34 {
		return "1.15-Snapshot"
	} else if year == 18 && week >= 43 || year == 19 && week <= 14 {
		return "1.14-Snapshot"
	} else if year == 18 && week >= 30 && week <= 33 {
		return "1.13.1-Snapshot"
	} else if year == 17 && week >= 43 || year == 18 && week <= 22 {
		return "1.13-Snapshot"
	} else if year == 17 && week == 31 {
		return "1.12.1-Snapshot"
	} else if year == 17 && week >= 6 && week <= 18 {
		return "1.12-Snapshot"
	} else if year == 16 && week == 50 {
		return "1.11.1-Snapshot"
	} else if year == 16 && week >= 32 && week <= 44 {
		return "1.11-Snapshot"
	} else if year == 16 && week >= 20 && week <= 21 {
		return "1.10-Snapshot"
	} else if year == 16 && week >= 14 && week <= 15 {
		return "1.9.3-Snapshot"
	} else if year == 15 && week >= 31 || year == 16 && week <= 7 {
		return "1.9-Snapshot"
	} else if year == 14 && week >= 2 && week <= 34 {
		return "1.8-Snapshot"
	} else if year == 13 && week >= 47 && week <= 49 {
		return "1.7.4-Snapshot"
	} else if year == 13 && week >= 36 && week <= 43 {
		return "1.7.2-Snapshot"
	} else if year == 13 && week >= 16 && week <= 26 {
		return "1.6-Snapshot"
	} else if year == 13 && week >= 11 && week <= 12 {
		return "1.5.1-Snapshot"
	} else if year == 13 && week >= 1 && week <= 10 {
		return "1.5-Snapshot"
	} else if year == 12 && week >= 49 && week <= 50 {
		return "1.4.6-Snapshot"
	} else if year == 12 && week >= 32 && week <= 42 {
		return "1.4.2-Snapshot"
	} else if year == 12 && week >= 15 && week <= 30 {
		return "1.3.1-Snapshot"
	} else if year == 12 && week >= 3 && week <= 8 {
		return "1.2.1-Snapshot"
	} else if year == 11 && week >= 47 || year == 12 && week <= 1 {
		return "1.1-Snapshot"
	}
	return mcVersion
}

func GetCurseforgeVersions(mcVersions []string) []string {
	out := make([]string, len(mcVersions))
	for i, v := range mcVersions {
		out[i] = GetCurseforgeVersion(v)
	}
	return out
}

var urlRegexes = [...]*regexp.Regexp{
	regexp.MustCompile(`^https?://(?P<game>minecraft)\.curseforge\.com/projects/(?P<slug>[^/]+)(?:/(?:files|download)/(?P<fileID>\d+))?`),
	regexp.MustCompile(`^https?://(?:www\.|beta\.|legacy\.)?curseforge\.com/(?P<game>[^/]+)/(?P<category>[^/]+)/(?P<slug>[^/]+)(?:/(?:files|download)/(?P<fileID>\d+))?`),
	regexp.MustCompile(`^(?P<slug>[a-z][\da-z\-_]{0,127})$`),
}

func CfParseSlugOrUrl(url string) (game string, category string, slug string, fileID uint32, err error) {
	for _, r := range urlRegexes {
		matches := r.FindStringSubmatch(url)
		if matches != nil {
			if i := r.SubexpIndex("game"); i >= 0 {
				game = matches[i]
			}
			if i := r.SubexpIndex("category"); i >= 0 {
				category = matches[i]
			}
			if i := r.SubexpIndex("slug"); i >= 0 {
				slug = matches[i]
			}
			if i := r.SubexpIndex("fileID"); i >= 0 {
				if matches[i] != "" {
					var f uint64
					f, err = strconv.ParseUint(matches[i], 10, 32)
					fileID = uint32(f)
				}
			}
			return
		}
	}
	return
}

var defaultFolders = map[uint32]map[uint32]string{
	432: { // Minecraft
		5:  "plugins", // Bukkit Plugins
		12: "resourcepacks",
		6:  "mods",
		17: "saves",
	},
}

func GetCfModType(gameID, classID, categoryID uint32) string {
	if m, ok := defaultFolders[gameID]; ok {
		if modType, ok := m[classID]; ok {
			return modType
		} else if modType, ok := m[categoryID]; ok {
			return modType
		}
	}
	return "unknown"
}

func CreateModFile(modInfo ModInfo, fileInfo ModFileInfo, optionalDisabled bool) (*core.Mod, error) {
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

func GetSearchLoaderType(pack core.Pack) ModloaderType {
	dependencies := pack.Versions

	_, hasFabric := dependencies["fabric"]
	_, hasQuilt := dependencies["quilt"]
	_, hasForge := dependencies["forge"]
	_, hasNeoForge := dependencies["neoforge"]
	if hasFabric && !hasQuilt && !hasForge && !hasNeoForge {
		return ModloaderTypeFabric
	}
	if hasForge && !hasNeoForge && !hasFabric && !hasQuilt {
		return ModloaderTypeForge
	}
	// We can't filter by more than one loader: accept any and filter the response
	return ModloaderTypeAny
}

// Crude way of preferring Quilt to Fabric / NeoForge to Forge: larger types are preferred
// so NeoForge > Quilt > Fabric > Forge > Any

func FilterLoaderTypeIndex(packLoaders []string, modLoaderType ModloaderType) (ModloaderType, bool) {
	if len(packLoaders) == 0 || modLoaderType == ModloaderTypeAny {
		// No loaders are specified: allow all files
		return ModloaderTypeAny, true
	} else {
		if int(modLoaderType) < len(ModloaderIds) && slices.Contains(packLoaders, ModloaderIds[modLoaderType]) {
			// PackToml contains this loader, pass through
			return modLoaderType, true
		} else {
			// PackToml does not contain this loader, report unsupported
			return ModloaderTypeAny, false
		}
	}
}

func FilterFileInfoLoaderIndex(packLoaders []string, fileInfoData ModFileInfo) (ModloaderType, bool) {
	if len(packLoaders) == 0 {
		// No loaders are specified: allow all files
		return ModloaderTypeAny, true
	} else {
		bestLoaderId := -1
		for i, name := range ModloaderNames {
			// Check if packLoaders and the file both contain this loader type
			if slices.Contains(packLoaders, ModloaderIds[i]) && slices.Contains(fileInfoData.GameVersions, name) {
				if i > bestLoaderId {
					// First loader found, or a loader preferred over the previous one (later IDs are preferred)
					bestLoaderId = i
				}
			}
		}
		if bestLoaderId > -1 {
			// Found a supported loader
			return ModloaderType(bestLoaderId), true
		} else {
			// Failed to find a supported version
			return ModloaderTypeAny, false
		}
	}
}

// FindLatestFile looks at mod info, and finds the latest file ID (and potentially the file info for it - may be null)
func FindLatestFile(modInfoData ModInfo, mcVersions []string, packLoaders []string) (fileID uint32, fileInfoData *ModFileInfo, fileName string) {
	cfMcVersions := GetCurseforgeVersions(mcVersions)
	bestMcVer := -1
	bestLoaderType := ModloaderTypeAny

	// For snapshots, curseforge doesn't put them in GameVersionLatestFiles
	for _, v := range modInfoData.LatestFiles {
		mcVerIdx := core.HighestSliceIndex(mcVersions, v.GameVersions)
		loaderIdx, loaderValid := FilterFileInfoLoaderIndex(packLoaders, v)

		if mcVerIdx < 0 || !loaderValid {
			continue
		}
		// Compare first by Minecraft version (prefer higher indexes of mcVersions)
		compare := int32(mcVerIdx - bestMcVer)
		if compare == 0 {
			// Treat unmarked versions as neutral (i.e. same as others)
			if bestLoaderType == ModloaderTypeAny || loaderIdx == ModloaderTypeAny {
				compare = 0
			} else {
				// Prefer higher loader indexes
				compare = int32(loaderIdx) - int32(bestLoaderType)
			}
		}
		if compare == 0 {
			// Other comparisons are equal, compare by ID instead
			compare = int32(int64(v.ID) - int64(fileID))
		}
		if compare > 0 {
			fileID = v.ID
			fileInfoDataCopy := v // Fix for loop variable reference (which gets reassigned on every iteration!)
			fileInfoData = &fileInfoDataCopy
			fileName = v.FileName
			bestMcVer = mcVerIdx
			bestLoaderType = loaderIdx
		}
	}
	// TODO: manage alpha/beta/release correctly, check update channel?
	for _, v := range modInfoData.GameVersionLatestFiles {
		mcVerIdx := slices.Index(cfMcVersions, v.GameVersion)
		loaderIdx, loaderValid := FilterLoaderTypeIndex(packLoaders, v.Modloader)

		if mcVerIdx < 0 || !loaderValid {
			continue
		}
		// Compare first by Minecraft version (prefer higher indexes of mcVersions)
		compare := int32(mcVerIdx - bestMcVer)
		if compare == 0 {
			// Treat unmarked versions as neutral (i.e. same as others)
			if bestLoaderType == ModloaderTypeAny || loaderIdx == ModloaderTypeAny {
				compare = 0
			} else {
				// Prefer higher loader indexes
				compare = int32(loaderIdx) - int32(bestLoaderType)
			}
		}
		if compare == 0 {
			// Other comparisons are equal, compare by ID instead
			compare = int32(int64(v.ID) - int64(fileID))
		}
		if compare > 0 {
			fileID = v.ID
			fileInfoData = nil // (no file info in GameVersionLatestFiles)
			fileName = v.Name
			bestMcVer = mcVerIdx
			bestLoaderType = loaderIdx
		}
	}
	return
}

type CfUpdateData struct {
	ProjectID uint32 `mapstructure:"project-id"`
	FileID    uint32 `mapstructure:"file-id"`
}

func (u CfUpdateData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}

type CfUpdater struct{}

func (u CfUpdater) GetName() string {
	return "curseforge"
}

func (u CfUpdater) ParseUpdate(updateUnparsed map[string]interface{}) (interface{}, error) {
	var updateData CfUpdateData
	err := mapstructure.Decode(updateUnparsed, &updateData)
	return updateData, err
}

type cachedStateStore struct {
	ModInfo
	fileID   uint32
	fileInfo *ModFileInfo
}

func (u CfUpdater) CheckUpdate(mods []*core.Mod, pack core.Pack) ([]core.UpdateCheck, error) {
	results := make([]core.UpdateCheck, len(mods))
	modIDs := make([]uint32, len(mods))
	modInfos := make([]ModInfo, len(mods))

	mcVersions, err := pack.GetSupportedMCVersions()
	if err != nil {
		return nil, err
	}

	for i, m := range mods {
		var project CfUpdateData
		err = m.DecodeNamedModSourceData("curseforge", &project)
		if err != nil {
			results[i] = core.UpdateCheck{Error: errors.New("failed to parse update metadata")}
			continue
		}
		modIDs[i] = project.ProjectID
	}

	modInfosUnsorted, err := GetCurseforgeClient().GetModInfoMultiple(modIDs)
	if err != nil {
		return nil, err
	}
	for _, v := range modInfosUnsorted {
		for i, id := range modIDs {
			if id == v.ID {
				modInfos[i] = v
				break
			}
		}
	}

	packLoaders := pack.GetCompatibleLoaders()

	for i, m := range mods {
		var project CfUpdateData
		err = m.DecodeNamedModSourceData("curseforge", &project)
		if err != nil {
			results[i] = core.UpdateCheck{Error: errors.New("failed to parse update metadata")}
			continue
		}

		fileID, fileInfoData, fileName := FindLatestFile(modInfos[i], mcVersions, packLoaders)
		if fileID != project.FileID && fileID != 0 {
			// Update (or downgrade, if changing to an older version) available!
			results[i] = core.UpdateCheck{
				UpdateAvailable: true,
				UpdateString:    m.FileName + " -> " + fileName,
				CachedState:     cachedStateStore{modInfos[i], fileID, fileInfoData},
			}
		} else {
			// Could not find a file, too old, or up to date: no update available
			results[i] = core.UpdateCheck{UpdateAvailable: false}
			continue
		}
	}
	return results, nil
}

func (u CfUpdater) DoUpdate(mods []*core.Mod, cachedState []interface{}) error {
	// "Do" isn't really that accurate, more like "Apply", because all the work is done in CheckUpdate!
	for i, m := range mods {
		modState := cachedState[i].(cachedStateStore)

		var fileInfoData ModFileInfo
		if modState.fileInfo != nil {
			fileInfoData = *modState.fileInfo
		} else {
			var err error
			fileInfoData, err = GetCurseforgeClient().GetFileInfo(modState.ID, modState.fileID)
			if err != nil {
				return err
			}
		}

		m.FileName = fileInfoData.FileName
		m.Name = modState.Name
		hash, hashFormat := fileInfoData.GetBestHash()
		m.Download = core.ModDownload{
			HashFormat: hashFormat,
			Hash:       hash,
			Mode:       core.ModeCF,
		}

		m.Update["curseforge"]["project-id"] = modState.ID
		m.Update["curseforge"]["file-id"] = fileInfoData.ID
	}

	return nil
}

type CfExportData struct {
	ProjectID uint32 `mapstructure:"project-id"`
}

func (e CfExportData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(e, &newMap)
	return newMap, err
}

func ParseExportData(from map[string]interface{}) (CfExportData, error) {
	var exportData CfExportData
	err := mapstructure.Decode(from, &exportData)
	return exportData, err
}

type CfDownloader struct{}

func (c CfDownloader) GetFilesMetadata(mods []*core.Mod) ([]core.MetaDownloaderData, error) {
	if len(mods) == 0 {
		return []core.MetaDownloaderData{}, nil
	}

	downloaderData := make([]core.MetaDownloaderData, len(mods))
	// Need array mapping project ID -> mod index, since there may be multiple files with the same project ID
	indexMap := make(map[uint32][]int)
	projectMetadata := make([]CfUpdateData, len(mods))
	fileIDs := make([]uint32, len(mods))
	for i, v := range mods {
		var project CfUpdateData
		err := v.DecodeNamedModSourceData("curseforge", &project)
		if err != nil {
			return nil, fmt.Errorf("failed to read CurseForge update metadata from %s", v.Name)
		}
		indexMap[project.ProjectID] = append(indexMap[project.ProjectID], i)
		projectMetadata[i] = project
		fileIDs[i] = project.FileID
	}

	fileData, err := GetCurseforgeClient().GetFileInfoMultiple(fileIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get CurseForge file metadata: %w", err)
	}

	modIDsToLookup := make([]uint32, 0)
	fileNames := make(map[uint32]string)
	for _, file := range fileData {
		if _, ok := indexMap[file.ModID]; !ok {
			return nil, fmt.Errorf("unknown project ID in response: %v (file %v, name %v)", file.ModID, file.ID, file.FileName)
		}
		// Opted-out mods don't provide their download URLs
		if file.DownloadURL == "" {
			modIDsToLookup = append(modIDsToLookup, file.ModID)
			fileNames[file.ModID] = file.FileName
		} else {
			for _, v := range indexMap[file.ModID] {
				downloaderData[v] = &CfDownloadMetadata{
					url: file.DownloadURL,
				}
			}
		}
	}

	if len(modIDsToLookup) > 0 {
		modData, err := GetCurseforgeClient().GetModInfoMultiple(modIDsToLookup)
		if err != nil {
			return nil, fmt.Errorf("failed to get CurseForge project metadata: %w", err)
		}
		for _, mod := range modData {
			if _, ok := indexMap[mod.ID]; !ok {
				return nil, fmt.Errorf("unknown project ID in response: %v (for %v)", mod.ID, mod.Name)
			}
			for _, v := range indexMap[mod.ID] {
				downloaderData[v] = &CfDownloadMetadata{
					noDistribution: true, // Inverted so the default value is not this (probably doesn't matter)
					name:           mod.Name,
					websiteUrl:     mod.Links.WebsiteURL + "/files/" + strconv.FormatUint(uint64(fileIDs[v]), 10),
					fileName:       fileNames[mod.ID],
				}
			}
		}
	}

	// Ensure all files got data
	for i, v := range downloaderData {
		if v == nil {
			return nil, fmt.Errorf("did not get CurseForge metadata for %s", mods[i].Name)
		}
	}

	return downloaderData, nil
}

type CfDownloadMetadata struct {
	url            string
	noDistribution bool
	name           string
	fileName       string
	websiteUrl     string
}

func (m *CfDownloadMetadata) GetManualDownload() (bool, core.ManualDownload) {
	if !m.noDistribution {
		return false, core.ManualDownload{}
	}
	return true, core.ManualDownload{
		Name:     m.name,
		FileName: m.fileName,
		URL:      m.websiteUrl,
	}
}

func (m *CfDownloadMetadata) DownloadFile() (io.ReadCloser, error) {
	resp, err := core.GetWithUA(m.url, "application/octet-stream")
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", m.url, err)
	}
	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("failed to download %s: invalid status code %v", m.url, resp.StatusCode)
	}
	return resp.Body, nil
}

// MapDepOverride transforms manual dependency overrides (which will likely be removed when packwiz is able to determine provided mods)
func MapDepOverride(depID uint32, isQuilt bool, mcVersion string) uint32 {
	if isQuilt && depID == 306612 {
		// Transform FAPI dependencies to QFAPI/QSL dependencies when using Quilt
		return 634179
	}
	if isQuilt && depID == 308769 {
		// Transform FLK dependencies to QKL dependencies when using Quilt >=1.19.2 non-snapshot
		if flexver.Less("1.19.1", mcVersion) && flexver.Less(mcVersion, "2.0.0") {
			return 720410
		}
	}
	return depID
}
