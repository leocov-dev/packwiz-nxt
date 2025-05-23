package cmdcurseforge

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/commands/cmdcurseforge/packinterop"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"github.com/spf13/viper"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/leocov-dev/packwiz-nxt/core"
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
		var packImport packinterop.ImportPackMetadata

		// TODO: refactor/extract file checking?
		if strings.HasPrefix(inputFile, "http") {
			// TODO: implement
			shared.Exitln("HTTP not supported (yet)")
		} else {
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
						shared.Exitf("Error opening file: %s\n", err)
					}
				} else {
					fmt.Printf("Error opening file: %s\n", err)
					fmt.Printf("Also attempted minecraftinstance.json: %s\n", errInstance)
					fmt.Printf("Also attempted manifest.json: %s\n", errManifest)
					if errCurse != nil {
						fmt.Printf("Also attempted to load a Curse/Twitch modpack named \"%s\": %s\n", inputFile, errCurse)
					}
					os.Exit(1)
				}
			}
			defer f.Close()

			buf := bufio.NewReader(f)
			header, err := buf.Peek(2)
			if err != nil {
				shared.Exitf("Error reading file: %s\n", err)
			}

			// Check if file is a zip
			if string(header) == "PK" {
				// Read the whole file (as bufio doesn't work for zips)
				zipData, err := io.ReadAll(buf)
				if err != nil {
					shared.Exitf("Error reading file: %s\n", err)
				}
				// Get zip size
				stat, err := f.Stat()
				if err != nil {
					shared.Exitf("Error reading file: %s\n", err)
				}
				zr, err := zip.NewReader(bytes.NewReader(zipData), stat.Size())
				if err != nil {
					shared.Exitf("Error parsing zip: %s\n", err)
				}

				// Search the zip for minecraftinstance.json or manifest.json
				var metaFile *zip.File
				for _, v := range zr.File {
					if v.Name == "minecraftinstance.json" || v.Name == "manifest.json" {
						metaFile = v
					}
				}

				if metaFile == nil {
					shared.Exitln("Can't find manifest.json or minecraftinstance.json, is this a valid pack?")
				}

				packImport = packinterop.ReadMetadata(packinterop.GetZipPackSource(metaFile, zr))
			} else {
				packImport = packinterop.ReadMetadata(packinterop.GetDiskPackSource(buf, filepath.ToSlash(filepath.Base(inputFile)), filepath.Dir(inputFile)))
			}
		}
		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

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

		modInfos, err := sources.GetCurseforgeClient().GetModInfoMultiple(modIDs)
		if err != nil {
			shared.Exitf("Failed to obtain project information: %s\n", err)
		}

		modInfosMap := make(map[uint32]sources.CfModInfo)
		for _, v := range modInfos {
			modInfosMap[v.ID] = v
		}

		// TODO: multithreading????

		modFileInfosMap := make(map[uint32]sources.CfModFileInfo)
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
			var fileInfo sources.CfModFileInfo
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

		modFileInfos, err := sources.GetCurseforgeClient().GetFileInfoMultiple(remainingFileIDs)
		if err != nil {
			shared.Exitf("Failed to obtain project file information: %s\n", err)
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

			mod, err := sources.CurseforgeNewMod(modInfoValue, modFileInfoValue, v.OptionalDisabled)
			if err != nil {
				shared.Exitf("Failed to save project \"%s\": %s\n", modInfoValue.Name, err)
			}

			pack.SetMod(mod)

			modFilePath := getPathForFile(modInfoValue.GameID, modInfoValue.ClassID, modInfoValue.PrimaryCategoryID, modInfoValue.Slug)
			ref, err := filepath.Abs(filepath.Join(filepath.Dir(modFilePath), modFileInfoValue.FileName))
			if err == nil {
				referencedModPaths = append(referencedModPaths, ref)
			}

			fmt.Printf("Imported dependency \"%s\" successfully!\n", modInfoValue.Name)
			successes++
		}

		fmt.Printf("Successfully imported %d/%d dependencies!\n", successes, len(modsList))

		fmt.Println("Reading override files...")
		filesList, err := packImport.GetFiles()
		if err != nil {
			shared.Exitf("Failed to read override files: %s\n", err)
		}

		successes = 0
		for _, v := range filesList {
			filePath := filepath.Join(packDir, filepath.FromSlash(v.Name()))
			filePathAbs, err := filepath.Abs(filePath)
			if err == nil {
				found := false
				for _, v := range referencedModPaths {
					if v == filePathAbs {
						found = true
						break
					}
				}
				if found {
					fmt.Printf("Ignored file \"%s\" (referenced by metadata)\n", filePath)
					successes++
					continue
				}
				if v.Name() == "manifest.json" || v.Name() == "minecraftinstance.json" || v.Name() == ".curseclient" {
					fmt.Printf("Ignored file \"%s\"\n", v.Name())
					successes++
					continue
				}
			}

			f, err := os.Create(filePath)
			if err != nil {
				// Attempt to create the containing directory
				err2 := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
				if err2 == nil {
					f, err = os.Create(filePath)
				}
				if err != nil {
					fmt.Printf("Failed to write file \"%s\": %s\n", filePath, err)
					if err2 != nil {
						fmt.Printf("Failed to create directories: %s\n", err)
					}
					continue
				}
			}

			src, err := v.Open()
			if err != nil {
				fmt.Printf("Failed to read file \"%s\": %s\n", filePath, err)
				f.Close()
				continue
			}
			_, err = io.Copy(f, src)
			if err != nil {
				fmt.Printf("Failed to copy file \"%s\": %s\n", filePath, err)
				f.Close()
				src.Close()
				continue
			}

			fmt.Printf("Copied file \"%s\" successfully!\n", filePath)
			f.Close()
			src.Close()
			successes++
		}

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitln(err)
		}
	},
}

func getPathForFile(gameID uint32, classID uint32, categoryID uint32, slug string) string {
	metaFolder := viper.GetString("meta-folder")
	if metaFolder == "" {
		metaFolder = sources.GetCfModType(gameID, classID, categoryID)
	}
	return filepath.Join(viper.GetString("meta-folder-base"), metaFolder, slug+core.MetaExtension)
}
