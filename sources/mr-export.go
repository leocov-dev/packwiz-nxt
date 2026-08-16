package sources

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"golang.org/x/exp/slices"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
)

// modrinthWhitelistedHosts are the domains Modrinth allows a pack manifest
// entry to reference directly by URL.
var modrinthWhitelistedHosts = []string{
	"cdn.modrinth.com",
	"github.com",
	"raw.githubusercontent.com",
	"gitlab.com",
}

// CanBeIncludedDirectly reports whether a mod can be referenced directly by
// URL in an exported Modrinth pack manifest, rather than being embedded as a
// file in the exported archive's overrides. When restrictDomains is true,
// only mods hosted on Modrinth's whitelisted domains can be included
// directly.
func CanBeIncludedDirectly(mod *core.Mod, restrictDomains bool) bool {
	if mod.Download.Mode == core.ModeURL || mod.Download.Mode == "" {
		if !restrictDomains {
			return true
		}

		modUrl, err := url.Parse(mod.Download.URL)
		if err == nil {
			if slices.Contains(modrinthWhitelistedHosts, modUrl.Host) {
				return true
			}
		}
	}
	return false
}

// BuildModrinthManifest builds the Modrinth pack manifest (the contents of
// modrinth.index.json) for the given pack, driving the provided download
// session to completion.
//
// For each completed download that CanBeIncludedDirectly, a manifest file
// entry is added (with client/server env derived from the mod's Side and
// Option). Downloads that cannot be included directly are instead handed to
// embed, along with the override subdirectory ("overrides", "client-overrides"
// or "server-overrides") they belong in, so the caller can store them as a
// file in the exported archive.
func BuildModrinthManifest(
	ctx context.Context,
	pack core.Pack,
	session fileio.DownloadSession,
	restrictDomains bool,
	embed func(dl fileio.CompletedDownload, dir string),
) (ModrinthPack, error) {
	manifestFiles := make([]ModrinthPackFile, 0)

	for dl := range session.StartDownloads(ctx) {
		if CanBeIncludedDirectly(dl.Mod, restrictDomains) {
			if dl.Error != nil {
				fmt.Printf("Download of %s (%s) failed: %v\n", dl.Mod.Name, dl.Mod.FileName, dl.Error)
				continue
			}
			for _, warning := range dl.Warnings {
				fmt.Printf("Warning for %s (%s): %v\n", dl.Mod.Name, dl.Mod.FileName, warning)
			}

			file, err := buildModrinthManifestFile(dl)
			if err != nil {
				return ModrinthPack{}, err
			}
			manifestFiles = append(manifestFiles, file)

			fmt.Printf("%s (%s) added to manifest\n", dl.Mod.Name, dl.Mod.FileName)
		} else {
			dir := "overrides"
			if dl.Mod.Side == core.ClientSide {
				dir = "client-overrides"
			} else if dl.Mod.Side == core.ServerSide {
				dir = "server-overrides"
			}
			embed(dl, dir)
		}
	}

	// sort by `path` property before serialising to ensure reproducibility
	sort.Slice(manifestFiles, func(i, j int) bool {
		return manifestFiles[i].Path < manifestFiles[j].Path
	})

	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return ModrinthPack{}, fmt.Errorf("Error creating manifest: %w", err)
	}
	dependencies := map[string]string{
		"minecraft": mcVersion,
	}
	if quiltVersion, ok := pack.Versions["quilt"]; ok {
		dependencies["quilt-loader"] = quiltVersion
	} else if fabricVersion, ok := pack.Versions["fabric"]; ok {
		dependencies["fabric-loader"] = fabricVersion
	} else if forgeVersion, ok := pack.Versions["forge"]; ok {
		dependencies["forge"] = forgeVersion
	} else if neoforgeVersion, ok := pack.Versions["neoforge"]; ok {
		dependencies["neoforge"] = neoforgeVersion
	}

	return ModrinthPack{
		FormatVersion: 1,
		Game:          "minecraft",
		VersionID:     pack.Version,
		Name:          pack.Name,
		Summary:       pack.Description,
		Files:         manifestFiles,
		Dependencies:  dependencies,
	}, nil
}

func buildModrinthManifestFile(dl fileio.CompletedDownload) (ModrinthPackFile, error) {
	path := dl.Mod.GetRelDownloadPath()

	hashes := map[string]string{
		"sha1":   dl.Hashes["sha1"],
		"sha512": dl.Hashes["sha512"],
	}
	fileSize, err := strconv.ParseUint(dl.Hashes["length-bytes"], 10, 64)
	if err != nil {
		return ModrinthPackFile{}, fmt.Errorf("failed to parse file size for %s (%s): %w", dl.Mod.Name, dl.Mod.FileName, err)
	}

	// Create env options based on configured optional/side
	var envInstalled string
	if dl.Mod.Option != nil && dl.Mod.Option.Optional {
		envInstalled = "optional"
	} else {
		envInstalled = "required"
	}
	var clientEnv, serverEnv string
	if dl.Mod.Side == core.UniversalSide || dl.Mod.Side == core.EmptySide {
		clientEnv = envInstalled
		serverEnv = envInstalled
	} else if dl.Mod.Side == core.ClientSide {
		clientEnv = envInstalled
		serverEnv = "unsupported"
	} else if dl.Mod.Side == core.ServerSide {
		clientEnv = "unsupported"
		serverEnv = envInstalled
	}

	// Modrinth URLs must be RFC3986
	u, err := core.ReEncodeURL(dl.Mod.Download.URL)
	if err != nil {
		fmt.Printf("Error re-encoding download URL: %s\n", err.Error())
		u = dl.Mod.Download.URL
	}

	return ModrinthPackFile{
		Path:   path,
		Hashes: hashes,
		Env: &struct {
			Client string `json:"client"`
			Server string `json:"server"`
		}{Client: clientEnv, Server: serverEnv},
		Downloads: []string{u},
		FileSize:  uint32(fileSize),
	}, nil
}
