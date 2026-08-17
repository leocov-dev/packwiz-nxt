package cmdmodrinth

import (
	"archive/zip"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
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

		mods := pack.GetModsList()

		fmt.Printf("Retrieving %v external files...\n", len(mods))

		restrictDomains := viper.GetBool("modrinth.export.restrictDomains")

		for _, mod := range mods {
			if !sources.CanBeIncludedDirectly(mod, restrictDomains) {
				shared.PrintDisclaimer(false)
				break
			}
		}

		session, err := fileio.CreateDownloadSession(nil, mods, []string{"sha1", "sha512", "length-bytes"})
		if err != nil {
			shared.Exitf("Error retrieving external files: %v\n", err)
		}

		shared.ListManualDownloads(session)

		err = shared.WithZipWriter(fileName, func(exp *zip.Writer) error {
			// Add an overrides folder even if there are no files to go in it
			if _, err := exp.Create("overrides/"); err != nil {
				return fmt.Errorf("Failed to add overrides folder: %w", err)
			}

			manifest, err := sources.BuildModrinthManifest(cmd.Context(), *pack, session, restrictDomains, func(dl fileio.CompletedDownload, dir string) {
				_ = shared.AddToZip(dl, exp, dir)
			})
			if err != nil {
				return err
			}

			if err := session.SaveIndex(); err != nil {
				return fmt.Errorf("Error saving cache index: %w", err)
			}

			if len(pack.Version) == 0 {
				fmt.Println("Warning: pack.toml version field must not be empty to create a valid Modrinth pack")
			}

			manifestFile, err := exp.Create("modrinth.index.json")
			if err != nil {
				return fmt.Errorf("Error creating manifest: %w", err)
			}

			w := json.NewEncoder(manifestFile)
			w.SetIndent("", "    ") // Documentation uses 4 spaces
			if err := w.Encode(manifest); err != nil {
				return fmt.Errorf("Error writing manifest: %w", err)
			}

			return nil
		})
		if err != nil {
			shared.Exitln(err)
		}

		fmt.Println("Modpack exported to " + fileName)
	},
}

func init() {
	modrinthCmd.AddCommand(exportCmd)
	exportCmd.Flags().Bool("restrictDomains", true, "Restricts domains to those allowed by modrinth.com")
	exportCmd.Flags().StringP("output", "o", "", "The file to export the modpack to")
	_ = viper.BindPFlag("modrinth.export.restrictDomains", exportCmd.Flags().Lookup("restrictDomains"))
	_ = viper.BindPFlag("modrinth.export.output", exportCmd.Flags().Lookup("output"))
}
