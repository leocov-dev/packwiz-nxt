package cmdmodrinth

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"net/url"
	"os"
	"sort"
	"strconv"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/spf13/cobra"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the current modpack into a .mrpack for Modrinth",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		packFile, _, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		fileName := viper.GetString("modrinth.export.output")
		if fileName == "" {
			fileName = pack.GetExportName() + ".mrpack"
		}
		expFile, err := os.Create(fileName)
		if err != nil {
			shared.Exitf("Failed to create zip: %s\n", err.Error())
		}
		exp := zip.NewWriter(expFile)

		// Add an overrides folder even if there are no files to go in it
		_, err = exp.Create("overrides/")
		if err != nil {
			shared.Exitf("Failed to add overrides folder: %s\n", err.Error())
		}

		mods := pack.GetModsList()

		fmt.Printf("Retrieving %v external files...\n", len(mods))

		restrictDomains := viper.GetBool("modrinth.export.restrictDomains")

		for _, mod := range mods {
			if !canBeIncludedDirectly(mod, restrictDomains) {
				shared.PrintDisclaimer(false)
				break
			}
		}

		session, err := fileio.CreateDownloadSession(mods, []string{"sha1", "sha512", "length-bytes"})
		if err != nil {
			shared.Exitf("Error retrieving external files: %v\n", err)
		}

		shared.ListManualDownloads(session)

		manifestFiles := make([]sources.ModrinthPackFile, 0)
		for dl := range session.StartDownloads() {
			if canBeIncludedDirectly(dl.Mod, restrictDomains) {
				if dl.Error != nil {
					fmt.Printf("Download of %s (%s) failed: %v\n", dl.Mod.Name, dl.Mod.FileName, dl.Error)
					continue
				}
				for _, warning := range dl.Warnings {
					fmt.Printf("Warning for %s (%s): %v\n", dl.Mod.Name, dl.Mod.FileName, warning)
				}

				path := dl.Mod.GetRelDownloadPath()

				hashes := make(map[string]string)
				hashes["sha1"] = dl.Hashes["sha1"]
				hashes["sha512"] = dl.Hashes["sha512"]
				fileSize, err := strconv.ParseUint(dl.Hashes["length-bytes"], 10, 64)
				if err != nil {
					panic(err)
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

				manifestFiles = append(manifestFiles, sources.ModrinthPackFile{
					Path:   path,
					Hashes: hashes,
					Env: &struct {
						Client string `json:"client"`
						Server string `json:"server"`
					}{Client: clientEnv, Server: serverEnv},
					Downloads: []string{u},
					FileSize:  uint32(fileSize),
				})

				fmt.Printf("%s (%s) added to manifest\n", dl.Mod.Name, dl.Mod.FileName)
			} else {
				if dl.Mod.Side == core.ClientSide {
					_ = shared.AddToZip(dl, exp, "client-overrides")
				} else if dl.Mod.Side == core.ServerSide {
					_ = shared.AddToZip(dl, exp, "server-overrides")
				} else {
					_ = shared.AddToZip(dl, exp, "overrides")
				}
			}
		}
		// sort by `path` property before serialising to ensure reproducibility
		sort.Slice(manifestFiles, func(i, j int) bool {
			return manifestFiles[i].Path < manifestFiles[j].Path
		})

		err = session.SaveIndex()
		if err != nil {
			shared.Exitf("Error saving cache index: %v\n", err)
		}

		dependencies := make(map[string]string)
		dependencies["minecraft"], err = pack.GetMCVersion()
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			shared.Exitln("Error creating manifest: " + err.Error())
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

		manifest := sources.ModrinthPack{
			FormatVersion: 1,
			Game:          "minecraft",
			VersionID:     pack.Version,
			Name:          pack.Name,
			Summary:       pack.Description,
			Files:         manifestFiles,
			Dependencies:  dependencies,
		}

		if len(pack.Version) == 0 {
			fmt.Println("Warning: pack.toml version field must not be empty to create a valid Modrinth pack")
		}

		manifestFile, err := exp.Create("modrinth.index.json")
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			shared.Exitln("Error creating manifest: " + err.Error())
		}

		w := json.NewEncoder(manifestFile)
		w.SetIndent("", "    ") // Documentation uses 4 spaces
		err = w.Encode(manifest)
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			shared.Exitln("Error writing manifest: " + err.Error())
		}

		//shared.AddNonMetafileOverrides(&index, exp)

		err = exp.Close()
		if err != nil {
			shared.Exitln("Error writing export file: " + err.Error())
		}
		err = expFile.Close()
		if err != nil {
			shared.Exitln("Error writing export file: " + err.Error())
		}

		fmt.Println("Modpack exported to " + fileName)
	},
}

var whitelistedHosts = []string{
	"cdn.modrinth.com",
	"github.com",
	"raw.githubusercontent.com",
	"gitlab.com",
}

func canBeIncludedDirectly(mod *core.Mod, restrictDomains bool) bool {
	if mod.Download.Mode == core.ModeURL || mod.Download.Mode == "" {
		if !restrictDomains {
			return true
		}

		modUrl, err := url.Parse(mod.Download.URL)
		if err == nil {
			if slices.Contains(whitelistedHosts, modUrl.Host) {
				return true
			}
		}
	}
	return false
}

func init() {
	modrinthCmd.AddCommand(exportCmd)
	exportCmd.Flags().Bool("restrictDomains", true, "Restricts domains to those allowed by modrinth.com")
	exportCmd.Flags().StringP("output", "o", "", "The file to export the modpack to")
	_ = viper.BindPFlag("modrinth.export.restrictDomains", exportCmd.Flags().Lookup("restrictDomains"))
	_ = viper.BindPFlag("modrinth.export.output", exportCmd.Flags().Lookup("output"))
}
