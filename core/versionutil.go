package core

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/unascribed/FlexVer/go/flexver"
)

type ModLoaderComponent struct {
	Name              string
	FriendlyName      string
	VersionListGetter func(mcVersion string) ([]string, string, error)
}

var ModLoaders = map[string]ModLoaderComponent{
	"fabric": {
		Name:         "fabric",
		FriendlyName: "Fabric loader",
		VersionListGetter: func(mcVersion string) ([]string, string, error) {
			return GetLoaderCache().GetVersions(mcVersion, "fabric")
		},
	},
	"forge": {
		Name:         "forge",
		FriendlyName: "Forge",
		VersionListGetter: func(mcVersion string) ([]string, string, error) {
			return GetLoaderCache().GetVersions(mcVersion, "forge")
		},
	},
	"liteloader": {
		Name:         "liteloader",
		FriendlyName: "LiteLoader",
		VersionListGetter: func(mcVersion string) ([]string, string, error) {
			return GetLoaderCache().GetVersions(mcVersion, "liteloader")
		},
	},
	"quilt": {
		Name:         "quilt",
		FriendlyName: "Quilt loader",
		VersionListGetter: func(mcVersion string) ([]string, string, error) {
			return GetLoaderCache().GetVersions(mcVersion, "quilt")
		},
	},
	"neoforge": {
		Name:         "neoforge",
		FriendlyName: "NeoForge",
		VersionListGetter: func(mcVersion string) ([]string, string, error) {
			return GetLoaderCache().GetVersions(mcVersion, "neoforge")
		},
	},
}

func ComponentToFriendlyName(component string) string {
	if component == "minecraft" {
		return "Minecraft"
	}
	loader, ok := ModLoaders[component]
	if ok {
		return loader.FriendlyName
	} else {
		return component
	}
}

// HighestSliceIndex returns the highest index of the given values in the slice (-1 if no value is found in the slice)
func HighestSliceIndex(slice []string, values []string) int {
	highest := -1
	for _, val := range values {
		for i, v := range slice {
			if v == val && i > highest {
				highest = i
			}
		}
	}
	return highest
}

type ForgeRecommended struct {
	Homepage string            `json:"homepage"`
	Versions map[string]string `json:"promos"`
}

// GetForgeRecommended gets the recommended version of Forge for the given Minecraft version
func GetForgeRecommended(mcVersion string) string {
	res, err := GetWithUA("https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json", "application/json")
	if err != nil {
		return ""
	}
	dec := json.NewDecoder(res.Body)
	out := ForgeRecommended{}
	err = dec.Decode(&out)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	// Get mcVersion-recommended, if it doesn't exist then get mcVersion-latest
	// If neither exist, return empty string
	recommendedString := fmt.Sprintf("%s-recommended", mcVersion)
	if out.Versions[recommendedString] != "" {
		return out.Versions[recommendedString]
	}
	latestString := fmt.Sprintf("%s-latest", mcVersion)
	if out.Versions[latestString] != "" {
		return out.Versions[latestString]
	}
	return ""
}

func SortAndDedupeVersions(versions []string) {
	flexver.VersionSlice(versions).Sort()
	// Deduplicate the sorted array
	if len(versions) > 0 {
		j := 0
		for i := 1; i < len(versions); i++ {
			if versions[i] != versions[j] {
				j++
				versions[j] = versions[i]
			}
		}
		versions = versions[:j+1]
	}
}

// VersionMap keys are minecraft versions and value is list of valid loader
// versions for that minecraft version
type VersionMap map[string][]string

type LoaderVersionCache struct {
	Fabric     []string
	Forge      VersionMap
	Liteloader []string
	Quilt      []string
	Neoforge   VersionMap
}

var defaultLoaderCache LoaderVersionCache

func GetLoaderCache() *LoaderVersionCache {
	return &defaultLoaderCache
}

func (l *LoaderVersionCache) IsEmpty() bool {
	return len(l.Fabric) == 0 || len(l.Forge) == 0 || len(l.Liteloader) == 0 || len(l.Quilt) == 0 || len(l.Neoforge) == 0
}

func (l *LoaderVersionCache) RefreshCache() error {
	if fabricVersions, err := fetchFabricVersions(); err != nil {
		return err
	} else {
		l.Fabric = fabricVersions
	}

	if forgeVersions, err := fetchForgeVersions(); err != nil {
		return err
	} else {
		l.Forge = forgeVersions
	}

	if liteloaderVersions, err := fetchLiteloaderVersions(); err != nil {
		return err
	} else {
		l.Liteloader = liteloaderVersions
	}

	if quiltVersions, err := fetchQuiltVersions(); err != nil {
		return err
	} else {
		l.Quilt = quiltVersions
	}

	if neoforgeVersions, err := fetchNeoforgeVersions(); err != nil {
		return err
	} else {
		l.Neoforge = neoforgeVersions
	}

	return nil
}

func (l *LoaderVersionCache) GetVersions(mcVersion string, loader string) ([]string, string, error) {
	if l.IsEmpty() {
		if err := l.RefreshCache(); err != nil {
			return nil, "", err
		}
	}

	var versions []string

	if loader == "fabric" {
		versions = l.Fabric
	} else if loader == "forge" {
		versions = l.Forge[mcVersion]
	} else if loader == "liteloader" {
		versions = l.Liteloader
	} else if loader == "quilt" {
		versions = l.Quilt
	} else if loader == "neoforge" {
		versions = l.Neoforge[mcVersion]
	}

	if len(versions) == 0 {
		return nil, "", fmt.Errorf("unknown loader %s", loader)
	}

	return versions, versions[0], nil
}

func fetchFabricVersions() ([]string, error) {
	versions, err := fetchMavenList(
		"https://maven.fabricmc.net/net/fabricmc/fabric-loader/maven-metadata.xml",
		func(version string) string {
			// Skip versions containing "+"
			if strings.Contains(version, "+") {
				return ""
			}
			return version
		},
	)

	return SortDescending(versions), err
}

func fetchForgeVersions() (VersionMap, error) {
	versionMap, err := fetchMavenMap(
		"https://maven.minecraftforge.net/net/minecraftforge/forge/maven-metadata.xml",
		func(version string) (string, string) {
			parts := strings.Split(version, "-")

			return parts[0], parts[1]
		},
	)

	for mcVersion, loaderVersions := range versionMap {
		versionMap[mcVersion] = SortDescending(loaderVersions)
	}

	return versionMap, err
}

func fetchLiteloaderVersions() ([]string, error) {
	versions, err := fetchMavenList(
		"https://repo.mumfrey.com/content/repositories/snapshots/com/mumfrey/liteloader/maven-metadata.xml",
		func(version string) string {
			// versions are in the format <version>-SNAPSHOT
			return strings.Split(version, "-")[0]
		},
	)
	return SortDescending(versions), err
}

func fetchQuiltVersions() ([]string, error) {
	versions, err := fetchMavenList(
		"https://maven.quiltmc.org/repository/release/org/quiltmc/quilt-loader/maven-metadata.xml",
		func(version string) string {
			return version
		},
	)

	return SortDescending(versions), err
}

func fetchNeoforgeVersions() (VersionMap, error) {
	versions, err := fetchMavenMap(
		"https://maven.neoforged.net/releases/net/neoforged/forge/maven-metadata.xml",
		func(version string) (string, string) {
			parts := strings.Split(version, "-")
			if len(parts) < 2 {
				return "", ""
			}

			return parts[0], parts[1]
		},
	)
	if err != nil {
		return nil, err
	}

	moreVersions, err := fetchMavenMap(
		"https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml",
		func(version string) (string, string) {
			parts := strings.Split(version, ".")

			if len(parts) < 2 {
				return "", ""
			}

			return "1." + parts[0] + "." + parts[1], version
		},
	)
	if err != nil {
		return nil, err
	}

	for mcVersion, loaderVersions := range moreVersions {
		if _, exists := versions[mcVersion]; !exists {
			versions[mcVersion] = make([]string, 0)
		}
		versions[mcVersion] = append(versions[mcVersion], loaderVersions...)
	}

	for mcVersion, loaderVersions := range versions {
		versions[mcVersion] = SortDescending(loaderVersions)
	}

	return versions, nil
}

// ----
type mavenXmlMetadata struct {
	Versioning struct {
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
	} `xml:"versioning"`
}

func fetchMavenList(url string, versionCb func(version string) string) ([]string, error) {
	resp, err := GetWithUA(url, "application/xml")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var metadata mavenXmlMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, err
	}

	var filteredVersions []string
	// we want in reverse order
	for i := len(metadata.Versioning.Versions.Version) - 1; i >= 0; i-- {
		version := metadata.Versioning.Versions.Version[i]

		processedVersion := versionCb(version)
		if processedVersion == "" {
			continue
		}

		filteredVersions = append(filteredVersions, processedVersion)
	}

	return filteredVersions, nil
}

func fetchMavenMap(url string, keyValueCb func(version string) (string, string)) (VersionMap, error) {
	resp, err := GetWithUA(url, "application/xml")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var metadata mavenXmlMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, err
	}

	versionMap := make(VersionMap)

	for _, version := range metadata.Versioning.Versions.Version {
		if version == "" {
			continue
		}

		minecraftVersion, loaderVersion := keyValueCb(version)

		if minecraftVersion == "" || loaderVersion == "" {
			continue
		}

		if _, exists := versionMap[minecraftVersion]; !exists {
			versionMap[minecraftVersion] = make([]string, 0)
		}
		versionMap[minecraftVersion] = append(versionMap[minecraftVersion], loaderVersion)

	}

	return versionMap, nil
}
