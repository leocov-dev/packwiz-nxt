package cmdcurseforge

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/commands/cmdcurseforge/packinterop"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	curseforgeCmd.AddCommand(importCmd)
}

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import [modpack path]",
	Short: "Import a curseforge modpack from a downloaded pack zip or an installed metadata json file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]

		packImport, err := resolveImportSource(cmd.Context(), inputFile)
		if err != nil {
			shared.Exitln(err)
		}

		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		pack, referencedModPaths, err := sources.CurseforgeImportPack(packImport, packFile, packDir)
		if err != nil {
			shared.Exitln(err)
		}

		fmt.Println("Reading override files...")
		filesList, err := packImport.GetFiles()
		if err != nil {
			shared.Exitf("Failed to read override files: %s\n", err)
		}

		overrideFiles := make([]fileio.ImportOverrideFile, len(filesList))
		for i, v := range filesList {
			overrideFiles[i] = v
		}
		skipNames := []string{"manifest.json", "minecraftinstance.json", ".curseclient"}
		fileio.CopyImportOverrides(overrideFiles, packDir, referencedModPaths, skipNames)

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitln(err)
		}
	},
}

// resolveImportSource takes the (possibly ambiguous) positional argument
// passed to `curseforge import` and locates the actual pack metadata source
// it refers to: a manifest.json/minecraftinstance.json file, a zip
// containing one of those, a directory containing one of those, a URL to a
// modpack export zip, or (on Windows) a named Curse/Twitch install. It then
// parses that source into ImportPackMetadata.
func resolveImportSource(ctx context.Context, inputFile string) (packinterop.ImportPackMetadata, error) {
	if strings.HasPrefix(inputFile, "http") {
		// A CurseForge modpack export fetched over HTTP is always a zip - no need for
		// the local-file detection (directory/curse-instance) below, just fetch and
		// parse it the same way the local-zip branch does.
		resp, err := core.GetWithUAContext(ctx, inputFile, "application/octet-stream")
		if err != nil {
			return nil, fmt.Errorf("Error downloading modpack: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Error downloading modpack: invalid status code %v", resp.StatusCode)
		}
		zipData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Error downloading modpack: %w", err)
		}
		zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
		if err != nil {
			return nil, fmt.Errorf("Error parsing zip: %w", err)
		}
		return resolveZipImportSource(zr)
	}

	// Attempt to read from file
	var f *os.File
	inputFileStat, err := os.Stat(inputFile)
	if err == nil && inputFileStat.IsDir() {
		// Apparently os.Open doesn't fail when file given is a directory, only when it gets read
		err = errors.New("cannot open directory")
	}
	if err == nil {
		f, err = os.Open(inputFile)
	}
	if err != nil {
		found := false
		var errInstance error
		var errManifest error
		var errCurse error

		// Look for other files/folders
		if _, errInstance = os.Stat(filepath.Join(inputFile, "minecraftinstance.json")); errInstance == nil {
			inputFile = filepath.Join(inputFile, "minecraftinstance.json")
			found = true
		} else if _, errManifest = os.Stat(filepath.Join(inputFile, "manifest.json")); errManifest == nil {
			inputFile = filepath.Join(inputFile, "manifest.json")
			found = true
		} else if runtime.GOOS == "windows" {
			var dir string
			dir, errCurse = getCurseDir()
			if errCurse == nil {
				curseInstanceFile := filepath.Join(dir, "Minecraft", "Instances", inputFile, "minecraftinstance.json")
				if _, errCurse = os.Stat(curseInstanceFile); errCurse == nil {
					inputFile = curseInstanceFile
					found = true
				}
			}
		}

		if found {
			f, err = os.Open(inputFile)
			if err != nil {
				return nil, fmt.Errorf("Error opening file: %w", err)
			}
		} else {
			msg := fmt.Sprintf("Error opening file: %s\n", err)
			msg += fmt.Sprintf("Also attempted minecraftinstance.json: %s\n", errInstance)
			msg += fmt.Sprintf("Also attempted manifest.json: %s\n", errManifest)
			if errCurse != nil {
				msg += fmt.Sprintf("Also attempted to load a Curse/Twitch modpack named \"%s\": %s\n", inputFile, errCurse)
			}
			fmt.Print(msg)
			os.Exit(1)
		}
	}
	defer f.Close()

	buf := bufio.NewReader(f)
	header, err := buf.Peek(2)
	if err != nil {
		return nil, fmt.Errorf("Error reading file: %w", err)
	}

	// Check if file is a zip
	if string(header) == "PK" {
		// Read the whole file (as bufio doesn't work for zips)
		zipData, err := io.ReadAll(buf)
		if err != nil {
			return nil, fmt.Errorf("Error reading file: %w", err)
		}
		// Get zip size
		stat, err := f.Stat()
		if err != nil {
			return nil, fmt.Errorf("Error reading file: %w", err)
		}
		zr, err := zip.NewReader(bytes.NewReader(zipData), stat.Size())
		if err != nil {
			return nil, fmt.Errorf("Error parsing zip: %w", err)
		}

		return resolveZipImportSource(zr)
	}

	return packinterop.ReadMetadata(packinterop.GetDiskPackSource(buf, filepath.ToSlash(filepath.Base(inputFile)), filepath.Dir(inputFile)))
}

// resolveZipImportSource locates minecraftinstance.json/manifest.json inside an already-
// opened modpack export zip and parses it into ImportPackMetadata. Shared by the local-file
// zip branch and the HTTP branch of resolveImportSource, both of which only differ in how
// they obtain the zip.Reader.
func resolveZipImportSource(zr *zip.Reader) (packinterop.ImportPackMetadata, error) {
	// Search the zip for minecraftinstance.json or manifest.json
	var metaFile *zip.File
	for _, v := range zr.File {
		if v.Name == "minecraftinstance.json" || v.Name == "manifest.json" {
			metaFile = v
		}
	}

	if metaFile == nil {
		return nil, errors.New("Can't find manifest.json or minecraftinstance.json, is this a valid pack?")
	}

	return packinterop.ReadMetadata(packinterop.GetZipPackSource(metaFile, zr))
}
