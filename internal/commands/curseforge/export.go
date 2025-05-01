package curseforge

import (
	"archive/zip"
	"bufio"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"os"
	"strconv"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
	"github.com/leocov-dev/packwiz-nxt/internal/commands/curseforge/packinterop"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the current modpack into a .zip for curseforge",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		side := viper.GetString("curseforge.export.side")
		if side != core.UniversalSide && side != core.ServerSide && side != core.ClientSide {
			cmdshared.Exitf("Invalid side %q, must be one of client, server, or both (default)\n", side)
		}

		fmt.Println("Loading modpack...")
		pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			cmdshared.Exitln(err)
		}
		index, err := fileio.LoadPackIndexFile(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}
		// Do a refresh to ensure files are up to date
		err = index.Refresh()
		if err != nil {
			cmdshared.Exitln(err)
		}

		repr := index.ToWritable()
		writer := fileio.NewIndexWriter()
		format, hash, err := writer.Write(&repr)
		if err != nil {
			cmdshared.Exitln(err)
		}

		pack.RefreshIndexHash(format, hash)

		packWriter := fileio.NewPackWriter()
		err = packWriter.Write(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}

		fmt.Println("Reading external files...")
		mods, err := index.LoadAllMods()
		if err != nil {
			cmdshared.Exitf("Error reading file: %v\n", err)
		}
		i := 0
		// Filter mods by side
		// TODO: opt-in optional disabled filtering?
		for _, mod := range mods {
			if mod.Side == side || mod.Side == core.EmptySide || mod.Side == core.UniversalSide || side == core.UniversalSide {
				mods[i] = mod
				i++
			}
		}
		mods = mods[:i]

		var exportData cfExportData
		exportDataUnparsed, ok := pack.Export["curseforge"]
		if ok {
			exportData, err = parseExportData(exportDataUnparsed)
			if err != nil {
				cmdshared.Exitf("Failed to parse export metadata: %s\n", err.Error())
			}
		}

		fileName := viper.GetString("curseforge.export.output")
		if fileName == "" {
			fileName = pack.GetPackName() + ".zip"
		}

		expFile, err := os.Create(fileName)
		if err != nil {
			cmdshared.Exitf("Failed to create zip: %s\n", err.Error())
		}
		exp := zip.NewWriter(expFile)

		// Add an overrides folder even if there are no files to go in it
		_, err = exp.Create("overrides/")
		if err != nil {
			cmdshared.Exitf("Failed to add overrides folder: %s\n", err.Error())
		}

		cfFileRefs := make([]packinterop.AddonFileReference, 0, len(mods))
		nonCfMods := make([]*core.Mod, 0)
		for _, mod := range mods {
			projectRaw, ok := mod.GetParsedUpdateData("curseforge")
			// If the mod has curseforge metadata, add it to cfFileRefs
			if ok {
				p := projectRaw.(cfUpdateData)
				cfFileRefs = append(cfFileRefs, packinterop.AddonFileReference{
					ProjectID:        p.ProjectID,
					FileID:           p.FileID,
					OptionalDisabled: mod.Option != nil && mod.Option.Optional && !mod.Option.Default,
				})
			} else {
				nonCfMods = append(nonCfMods, mod)
			}
		}

		// Download external files and save directly into the zip
		if len(nonCfMods) > 0 {
			fmt.Printf("Retrieving %v external files to store in the modpack zip...\n", len(nonCfMods))
			cmdshared.PrintDisclaimer(true)

			session, err := core.CreateDownloadSession(nonCfMods, []string{})
			if err != nil {
				cmdshared.Exitf("Error retrieving external files: %v\n", err)
			}

			cmdshared.ListManualDownloads(session)

			for dl := range session.StartDownloads() {
				_ = cmdshared.AddToZip(dl, exp, "overrides", &index)
			}

			err = session.SaveIndex()
			if err != nil {
				cmdshared.Exitf("Error saving cache index: %v\n", err)
			}
		}

		manifestFile, err := exp.Create("manifest.json")
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			cmdshared.Exitln("Error creating manifest: " + err.Error())
		}

		err = packinterop.WriteManifestFromPack(pack, cfFileRefs, exportData.ProjectID, manifestFile)
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			cmdshared.Exitln("Error writing manifest: " + err.Error())
		}

		err = createModlist(exp, mods)
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			cmdshared.Exitln("Error creating mod list: " + err.Error())
		}

		cmdshared.AddNonMetafileOverrides(&index, exp)

		err = exp.Close()
		if err != nil {
			cmdshared.Exitln("Error writing export file: " + err.Error())
		}
		err = expFile.Close()
		if err != nil {
			cmdshared.Exitln("Error writing export file: " + err.Error())
		}

		fmt.Println("Modpack exported to " + fileName)
	},
}

func createModlist(zw *zip.Writer, mods []*core.Mod) error {
	modlistFile, err := zw.Create("modlist.html")
	if err != nil {
		return err
	}

	w := bufio.NewWriter(modlistFile)

	_, err = w.WriteString("<ul>\r\n")
	if err != nil {
		return err
	}
	for _, mod := range mods {
		projectRaw, ok := mod.GetParsedUpdateData("curseforge")
		if !ok {
			// TODO: read homepage URL or something similar?
			// TODO: how to handle mods that don't have metadata???
			_, err = w.WriteString("<li>" + mod.Name + "</li>\r\n")
			if err != nil {
				return err
			}
			continue
		}
		project := projectRaw.(cfUpdateData)
		_, err = w.WriteString("<li><a href=\"https://www.curseforge.com/projects/" + strconv.FormatUint(uint64(project.ProjectID), 10) + "\">" + mod.Name + "</a></li>\r\n")
		if err != nil {
			return err
		}
	}
	_, err = w.WriteString("</ul>\r\n")
	if err != nil {
		return err
	}
	return w.Flush()
}

func init() {
	curseforgeCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringP("side", "s", "client", "The side to export mods with")
	_ = viper.BindPFlag("curseforge.export.side", exportCmd.Flags().Lookup("side"))
	exportCmd.Flags().StringP("output", "o", "", "The file to export the modpack to")
	_ = viper.BindPFlag("curseforge.export.output", exportCmd.Flags().Lookup("output"))
}
