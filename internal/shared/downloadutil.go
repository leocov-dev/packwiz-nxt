package shared

import (
	"archive/zip"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"io"
	"os"
	"path"
	"path/filepath"
)

func ListManualDownloads(session fileio.DownloadSession) {
	manualDownloads := session.GetManualDownloads()
	if len(manualDownloads) > 0 {
		fmt.Printf("Found %v manual downloads; these mods are unable to be downloaded by packwiz (due to API limitations) and must be manually downloaded:\n",
			len(manualDownloads))
		for _, dl := range manualDownloads {
			fmt.Printf("%s (%s) from %s\n", dl.Name, dl.FileName, dl.URL)
		}
		cacheDir, err := fileio.GetPackwizCache()
		if err != nil {
			Exitf("Error locating cache folder: %v", err)
		}

		fmt.Printf("Once you have done so, place these files in %s and re-run this command.\n",
			filepath.Join(cacheDir, fileio.DownloadCacheImportFolder))
		os.Exit(1)
	}
}

func AddToZip(dl fileio.CompletedDownload, exp *zip.Writer, dir string) bool {
	if dl.Error != nil {
		fmt.Printf("Download of %s (%s) failed: %v\n", dl.Mod.Name, dl.Mod.FileName, dl.Error)
		return false
	}
	for _, warning := range dl.Warnings {
		fmt.Printf("Warning for %s (%s): %v\n", dl.Mod.Name, dl.Mod.FileName, warning)
	}

	p := dl.Mod.GetRelDownloadPath()

	modFile, err := exp.Create(path.Join(dir, p))
	if err != nil {
		fmt.Printf("Error creating metadata file %s: %v\n", p, err)
		return false
	}
	_, err = io.Copy(modFile, dl.File)
	if err != nil {
		fmt.Printf("Error copying file %s: %v\n", p, err)
		return false
	}
	err = dl.File.Close()
	if err != nil {
		fmt.Printf("Error closing file %s: %v\n", p, err)
		return false
	}

	fmt.Printf("%s (%s) added to zip\n", dl.Mod.Name, dl.Mod.FileName)
	return true
}

// AddNonMetafileOverrides saves all non-metadata files into an overrides folder in the zip
func AddNonMetafileOverrides(index *core.IndexFS, exp *zip.Writer) {
	// TODO: what to do about index files that are not metafile mods,
	//  currently we are not handling them correctly
	for p, v := range index.Files {
		if !v.IsMetaFile() {
			file, err := exp.Create(path.Join("overrides", p))
			if err != nil {
				fmt.Printf("Error creating file: %s\n", err.Error())
				// TODO: exit(1)?
				continue
			}
			// Attempt to read the file from disk, without checking hashes (assumed to have no errors)
			src, err := os.Open(index.ResolveIndexPath(p))
			if err != nil {
				_ = src.Close()
				fmt.Printf("Error reading file: %s\n", err.Error())
				// TODO: exit(1)?
				continue
			}
			_, err = io.Copy(file, src)
			if err != nil {
				_ = src.Close()
				fmt.Printf("Error copying file: %s\n", err.Error())
				// TODO: exit(1)?
				continue
			}

			_ = src.Close()
		}
	}
}

func PrintDisclaimer(isCf bool) {
	fmt.Println("Disclaimer: you are responsible for ensuring you comply with ALL the licenses, or obtain appropriate permissions, for the files \"added to zip\" below")
	if isCf {
		fmt.Println("Note that mods bundled within a CurseForge pack must be in the Approved Non-CurseForge Mods list")
		fmt.Println("packwiz is currently unable to match metadata between mod sites - if any of these are available from CurseForge you should change them to use CurseForge metadata (e.g. by re-adding them using the cf commands)")
	} else {
		fmt.Println("packwiz is currently unable to match metadata between mod sites - if any of these are available from Modrinth you should change them to use Modrinth metadata (e.g. by re-adding them using the mr commands)")
	}
	fmt.Println()
}
