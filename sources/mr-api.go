package sources

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"github.com/unascribed/FlexVer/go/flexver"
	"golang.org/x/exp/slices"

	"github.com/leocov-dev/packwiz-nxt/core"
)

var mrDefaultClient = modrinthApi.NewClient(&http.Client{})

func init() {
	mrDefaultClient.UserAgent = core.UserAgent
}

func GetModrinthClient() *modrinthApi.Client {
	return mrDefaultClient
}

func ModrinthProjectFromVersionID(versionId string) (*modrinthApi.Project, *modrinthApi.Version, error) {
	version, err := GetModrinthClient().Versions.Get(versionId)
	if err != nil {
		return nil, nil, err
	}
	project, err := GetModrinthClient().Projects.Get(*version.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return project, version, nil
}

func ModrinthSearchForProjects(query string, versions []string) ([]*modrinthApi.Project, error) {
	facets := make([]string, 0)
	for _, v := range versions {
		facets = append(facets, "versions:"+v)
	}

	res, err := GetModrinthClient().Projects.Search(&modrinthApi.SearchOptions{
		Limit:  5,
		Index:  "relevance",
		Facets: [][]string{facets},
		Query:  query,
	})
	if err != nil {
		return nil, err
	}
	if len(res.Hits) == 0 {
		return nil, errors.New("no projects found")
	}

	projects := make([]*modrinthApi.Project, 0)

	for _, result := range res.Hits {
		project, err := GetModrinthClient().Projects.Get(*result.ProjectID)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// "Loaders" that are supported regardless of the configured mod loaders
var defaultMRLoaders = []string{
	// TODO: check if Canvas/Iris/Optifine are installed? suggest installing them?
	"canvas",
	"iris",
	"optifine",
	"vanilla",   // Core shaders
	"minecraft", // Resource packs
}

var withDatapackPathMRLoaders = []string{
	"canvas",
	"iris",
	"optifine",
	"vanilla",   // Core shaders
	"minecraft", // Resource packs
	// TODO: check if a datapack loader is installed; suggest installing one?
	"datapack", // Datapacks (requires a datapack loader)
}

var mrLoaderFolders = map[string]string{
	"quilt":      "mods",
	"fabric":     "mods",
	"forge":      "mods",
	"neoforge":   "mods",
	"liteloader": "mods",
	"modloader":  "mods",
	"rift":       "mods",
	"bukkit":     "plugins",
	"spigot":     "plugins",
	"paper":      "plugins",
	"purpur":     "plugins",
	"sponge":     "plugins",
	"bungeecord": "plugins",
	"waterfall":  "plugins",
	"velocity":   "plugins",
	"canvas":     "resourcepacks",
	"iris":       "shaderpacks",
	"optifine":   "shaderpacks",
	"vanilla":    "resourcepacks",
}

// Preference list for loader types, for comparing files where the version is the same - more preferred is lower
var mrLoaderPreferenceList = []string{
	// Prefer quilt versions over fabric versions
	"quilt",
	"fabric",
	// Prefer neoforge versions over forge versions
	"neoforge",
	"forge",
	"liteloader",
	"modloader",
	"rift",
	// Prefer mods to plugins
	"sponge",
	// Prefer newer Bukkit forks
	"purpur",
	"paper",
	"spigot",
	"bukkit",
	"velocity",
	// Prefer newer BungeeCord forks
	"waterfall",
	"bungeecord",
	// Prefer Canvas shaders to Iris shaders to Optifine shaders to core shaders
	"canvas",
	"iris",
	"optifine",
	"vanilla",
	// Prefer mods to datapacks
	"datapack",
	// Prefer mods to resource packs?! Idk this is just here for completeness
	"minecraft",
}

// Groups of loaders that should be treated the same as the key, if both versions support the key
// i.e. the key is a more "generic" loader; support for it implies support for the whole group
// e.g. [quilt, fabric] should compare equal to [fabric] (but less than [quilt] as Quilt support doesn't imply Fabric support)
// This is useful when authors forget to add Quilt/Purpur etc. to all versions
// TODO: make abstracted from source backend
var mrLoaderCompatGroups = map[string][]string{
	"fabric":     {"quilt"},
	"forge":      {"neoforge"},
	"bukkit":     {"purpur", "paper", "spigot"},
	"bungeecord": {"waterfall"},
}

func mrGetProjectTypeFolder(projectType string, fileLoaders []string, packLoaders []string) (string, error) {
	if projectType == "modpack" {
		return "", errors.New("this command should not be used to add Modrinth modpacks, and importing of Modrinth modpacks is not yet supported")
	} else if projectType == "resourcepack" {
		return "resourcepacks", nil
	} else if projectType == "shader" {
		bestLoaderIdx := math.MaxInt
		for _, v := range fileLoaders {
			idx := slices.Index(mrLoaderPreferenceList, v)
			if idx != -1 && idx < bestLoaderIdx {
				bestLoaderIdx = idx
			}
		}
		if bestLoaderIdx > -1 && bestLoaderIdx < math.MaxInt {
			return mrLoaderFolders[mrLoaderPreferenceList[bestLoaderIdx]], nil
		}
		return "shaderpacks", nil
	} else if projectType == "mod" {
		// Look up pack loaders in the list of loaders (note this is currently filtered to quilt/fabric/neoforge/forge)
		bestLoaderIdx := math.MaxInt
		for _, v := range fileLoaders {
			if slices.Contains(packLoaders, v) {
				idx := slices.Index(mrLoaderPreferenceList, v)
				if idx != -1 && idx < bestLoaderIdx {
					bestLoaderIdx = idx
				}
			}
		}
		if bestLoaderIdx > -1 && bestLoaderIdx < math.MaxInt {
			return mrLoaderFolders[mrLoaderPreferenceList[bestLoaderIdx]], nil
		}

		// Datapack loader is "datapack"
		if slices.Contains(fileLoaders, "datapack") {
			//if viper.GetString("datapack-folder") != "" {
			//	return viper.GetString("datapack-folder"), nil
			//} else {
			//	return "", errors.New("set the datapack-folder option to use datapacks")
			//}
			return "", errors.New("datapacks are not supported yet")
		}
		// Default to "mods" for mod type
		return "mods", nil
	} else {
		return "", fmt.Errorf("unknown project type %s", projectType)
	}
}

var mrUrlRegexes = [...]*regexp.Regexp{
	// Slug/version number regex from https://github.com/modrinth/labrinth/blob/1679a3f844497d756d0cf272c5374a5236eabd42/src/util/validate.rs#L8
	regexp.MustCompile("^https?://(www.)?modrinth\\.com/(?P<urlCategory>[^/]+)/(?P<slug>[a-zA-Z0-9!@$()`.+,_\"-]{3,64})(?:/version/(?P<version>[a-zA-Z0-9!@$()`.+,_\"-]{1,32}))?"),
	// Version/project IDs are more restrictive: [a-zA-Z0-9]+ (base62)
	regexp.MustCompile("^https?://cdn\\.modrinth\\.com/data/(?P<slug>[a-zA-Z0-9]+)/versions/(?P<versionID>[a-zA-Z0-9]+)/(?P<filename>[^/]+)$"),
	regexp.MustCompile("^(?P<slug>[a-zA-Z0-9!@$()`.+,_\"-]{3,64})$"),
}

const mrSlugRegexIdx = 2

var mrUrlCategories = []string{
	"mod", "plugin", "datapack", "shader", "resourcepack", "modpack",
}

func ParseModrinthSlugOrUrl(input string, slug *string, version *string, versionID *string, filename *string) (parsedSlug bool, err error) {
	for regexIdx, r := range mrUrlRegexes {
		matches := r.FindStringSubmatch(input)
		if matches != nil {
			if i := r.SubexpIndex("urlCategory"); i >= 0 {
				if !slices.Contains(mrUrlCategories, matches[i]) {
					err = errors.New("unknown project type: " + matches[i])
					return
				}
			}
			if i := r.SubexpIndex("slug"); i >= 0 {
				*slug = matches[i]
			}
			if i := r.SubexpIndex("version"); i >= 0 {
				*version = matches[i]
			}
			if i := r.SubexpIndex("versionID"); i >= 0 {
				*versionID = matches[i]
			}
			if i := r.SubexpIndex("filename"); i >= 0 {
				var parsed string
				parsed, err = url.PathUnescape(matches[i])
				if err != nil {
					return
				}
				*filename = parsed
			}
			parsedSlug = regexIdx == mrSlugRegexIdx
			return
		}
	}
	return
}

func ParseAsModrinthSlug(input string) string {
	for _, r := range mrUrlRegexes {
		matches := r.FindStringSubmatch(input)
		if matches != nil {
			if i := r.SubexpIndex("urlCategory"); i >= 0 {
				if !slices.Contains(mrUrlCategories, matches[i]) {
					return ""
				}
			}
			if i := r.SubexpIndex("slug"); i >= 0 {
				return matches[i]
			}
		}
	}
	return ""
}

func ParseAsModrinthVersion(input string) string {
	for _, r := range mrUrlRegexes {
		matches := r.FindStringSubmatch(input)
		if matches != nil {
			if i := r.SubexpIndex("urlCategory"); i >= 0 {
				if !slices.Contains(mrUrlCategories, matches[i]) {
					return ""
				}
			}
			if i := r.SubexpIndex("version"); i >= 0 {
				return matches[i]
			}
		}
	}
	return ""
}

func ParseAsModrinthVersionID(input string) string {
	for _, r := range mrUrlRegexes {
		matches := r.FindStringSubmatch(input)
		if matches != nil {
			if i := r.SubexpIndex("urlCategory"); i >= 0 {
				if !slices.Contains(mrUrlCategories, matches[i]) {
					return ""
				}
			}
			if i := r.SubexpIndex("versionID"); i >= 0 {
				return matches[i]
			}
		}
	}
	return ""
}

func ParseAsParseAsFilename(input string) string {
	for _, r := range mrUrlRegexes {
		matches := r.FindStringSubmatch(input)
		if matches != nil {
			if i := r.SubexpIndex("urlCategory"); i >= 0 {
				if !slices.Contains(mrUrlCategories, matches[i]) {
					return ""
				}
			}
			if i := r.SubexpIndex("filename"); i >= 0 {
				parsed, err := url.PathUnescape(matches[i])
				if err != nil {
					return ""
				}
				return parsed
			}
		}
	}
	return ""
}

func mrCompareLoaderLists(a []string, b []string) int32 {
	var compat []string
	for k, v := range mrLoaderCompatGroups {
		if slices.Contains(a, k) && slices.Contains(b, k) {
			// Prerequisite loader is in both lists; add compat group
			compat = append(compat, v...)
		}
	}
	// Prefer loaders; principally Quilt over Fabric, mods over datapacks (Modrinth backend handles filtering)
	minIdxA := math.MaxInt
	for _, v := range a {
		if slices.Contains(compat, v) {
			// Ignore loaders in compat groups for comparison
			continue
		}
		idx := slices.Index(mrLoaderPreferenceList, v)
		if idx != -1 && idx < minIdxA {
			minIdxA = idx
		}
	}
	minIdxB := math.MaxInt
	for _, v := range b {
		if slices.Contains(compat, v) {
			// Ignore loaders in compat groups for comparison
			continue
		}
		idx := slices.Index(mrLoaderPreferenceList, v)
		if idx < minIdxA {
			return 1 // B has more preferable loaders
		}
		if idx != -1 && idx < minIdxB {
			minIdxB = idx
		}
	}
	if minIdxA < minIdxB {
		return -1 // A has more preferable loaders
	}
	return 0
}

func mrFindLatestVersion(versions []*modrinthApi.Version, gameVersions []string, useFlexVer bool) *modrinthApi.Version {
	latestValidVersion := versions[0]
	bestGameVersion := core.HighestSliceIndex(gameVersions, versions[0].GameVersions)
	for _, v := range versions[1:] {
		gameVersionIdx := core.HighestSliceIndex(gameVersions, v.GameVersions)

		var compare int32
		if useFlexVer {
			// Use FlexVer to compare versions
			compare = flexver.Compare(*v.VersionNumber, *latestValidVersion.VersionNumber)
		}

		if compare == 0 {
			// Prefer later specified game versions (main version specified last)
			compare = int32(gameVersionIdx - bestGameVersion)
		}
		if compare == 0 {
			compare = mrCompareLoaderLists(latestValidVersion.Loaders, v.Loaders)
		}
		if compare == 0 {
			// Other comparisons are equal, compare date instead
			if v.DatePublished.After(*latestValidVersion.DatePublished) {
				compare = 1
			}
		}
		if compare > 0 {
			latestValidVersion = v
			bestGameVersion = gameVersionIdx
		}
	}

	return latestValidVersion
}

func ModrinthGetLatestVersion(projectID string, name string, pack core.Pack, optionalDatapackFolder string) (*modrinthApi.Version, error) {
	gameVersions, err := pack.GetSupportedMCVersions()
	if err != nil {
		return nil, err
	}
	var loaders []string
	if optionalDatapackFolder != "" {
		loaders = append(pack.GetCompatibleLoaders(), withDatapackPathMRLoaders...)
	} else {
		loaders = append(pack.GetCompatibleLoaders(), defaultMRLoaders...)
	}

	result, err := GetModrinthClient().Versions.ListVersions(projectID, modrinthApi.ListVersionsOptions{
		GameVersions: gameVersions,
		Loaders:      loaders,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest version: %w", err)
	}
	if len(result) == 0 {
		// TODO: retry with datapack specified, to determine what the issue is? or just request all and filter afterwards
		return nil, errors.New("no valid versions found")
	}

	// TODO: option to always compare using flexver?
	// TODO: ask user which one to use?
	flexverLatest := mrFindLatestVersion(result, gameVersions, true)
	releaseDateLatest := mrFindLatestVersion(result, gameVersions, false)
	if flexverLatest != releaseDateLatest && releaseDateLatest.VersionNumber != nil && flexverLatest.VersionNumber != nil {
		fmt.Printf("Warning: Modrinth versions for %s inconsistent between latest version number and newest release date (%s vs %s)\n", name, *flexverLatest.VersionNumber, *releaseDateLatest.VersionNumber)
	}

	if releaseDateLatest.ID == nil {
		return nil, errors.New("mod not available for the configured Minecraft version(s) (use the 'packwiz settings acceptable-versions' command to accept more) or loader")
	}

	return releaseDateLatest, nil
}

func mrGetSide(mod *modrinthApi.Project) core.ModSide {
	server := mrShouldDownloadOnSide(*mod.ServerSide)
	client := mrShouldDownloadOnSide(*mod.ClientSide)

	if server && client {
		return core.UniversalSide
	} else if server {
		return core.ServerSide
	} else if client {
		return core.ClientSide
	} else {
		return core.EmptySide
	}
}

func mrShouldDownloadOnSide(side string) bool {
	return side == "required" || side == "optional"
}

func mrGetBestHash(v *modrinthApi.File) (string, string) {
	// Try preferred hashes first; SHA1 is required for Modrinth pack exporting, but
	// so is SHA512, so we can't win with the current one-hash format
	val, exists := v.Hashes["sha512"]
	if exists {
		return "sha512", val
	}
	val, exists = v.Hashes["sha256"]
	if exists {
		return "sha256", val
	}
	val, exists = v.Hashes["sha1"]
	if exists {
		return "sha1", val
	}
	val, exists = v.Hashes["murmur2"] // (not defined in Modrinth pack spec, use with caution)
	if exists {
		return "murmur2", val
	}

	//none of the preferred hashes are present, just get the first one
	for key, val := range v.Hashes {
		return key, val
	}

	//No hashes were present
	return "", ""
}

func mrGetInstalledProjectIDs(mods []*core.Mod) []string {
	var installedProjects []string

	for _, mod := range mods {
		var updateData mrUpdateData
		err := mod.DecodeNamedModSourceData("modrinth", updateData)
		if err == nil {
			if len(updateData.ProjectID) > 0 {
				installedProjects = append(installedProjects, updateData.ProjectID)
			}
		}
	}

	return installedProjects
}

func ResolveModrinthVersion(project *modrinthApi.Project, version string) (*modrinthApi.Version, error) {
	// If it exists in the version list, it is already a version ID (and doesn't need querying further)
	if slices.Contains(project.Versions, version) {
		versionData, err := GetModrinthClient().Versions.Get(version)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch version %s: %v", version, err)
		}
		return versionData, nil
	}

	// Look up all versions
	// TODO: PR a version number filter to Modrinth?
	versionsList, err := GetModrinthClient().Versions.ListVersions(*project.ID, modrinthApi.ListVersionsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version list for %s: %v", *project.ID, err)
	}
	// Traverse in reverse order: Modrinth knossos always gives the oldest file precedence over having the version number path
	for i := len(versionsList) - 1; i >= 0; i-- {
		if *versionsList[i].VersionNumber == version {
			return versionsList[i], nil
		}
	}
	return nil, fmt.Errorf("unable to find version %s", version)
}

// mrMapDepOverride transforms manual dependency overrides (which will likely be removed when packwiz is able to determine provided mods)
func mrMapDepOverride(depID string, isQuilt bool, mcVersion string) string {
	if isQuilt && (depID == "P7dR8mSH" || depID == "fabric-api") {
		// Transform FAPI dependencies to QFAPI/QSL dependencies when using Quilt
		return "qvIfYCYJ"
	}
	if isQuilt && (depID == "Ha28R6CL" || depID == "fabric-language-kotlin") {
		// Transform FLK dependencies to QKL dependencies when using Quilt >=1.19.2 non-snapshot
		if flexver.Less("1.19.1", mcVersion) && flexver.Less(mcVersion, "2.0.0") {
			return "lwVhp9o5"
		}
	}
	return depID
}
