package cmdcurseforge

import (
	"errors"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"strings"

	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/dixonwille/wmenu.v4"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
)

var addonIDFlag uint32
var fileIDFlag uint32

var categoryFlag string

func init() {
	curseforgeCmd.AddCommand(installCmd)

	installCmd.Flags().Uint32Var(&addonIDFlag, "addon-id", 0, "The CurseForge project ID to use")
	installCmd.Flags().Uint32Var(&fileIDFlag, "file-id", 0, "The CurseForge file ID to use")
	installCmd.Flags().StringVar(&categoryFlag, "category", "", "The category to add files from (slug, as stored in URLs); the category in the URL takes precedence")
}

const maxCycles = 20

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "add [URL|slug|search]",
	Short:   "Add a project from a CurseForge URL, slug, ID or search",
	Aliases: []string{"install", "get"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		// ---
		mcVersions, err := pack.GetSupportedMCVersions()
		if err != nil {
			shared.Exitln(err)
		}
		primaryMCVersion, err := pack.GetMCVersion()
		if err != nil {
			shared.Exitln(err)
		}

		// ---
		category := categoryFlag
		var modID, fileID uint32
		var slug string

		// If mod/file IDs are provided in command line, use those
		if fileIDFlag != 0 {
			fileID = fileIDFlag
		}
		if addonIDFlag != 0 {
			modID = addonIDFlag
		}

		if (len(args) == 0 || len(args[0]) == 0) && modID == 0 {
			shared.Exitln("You must specify a project; with the ID flags, or by passing a URL, slug or search term directly.")
		}
		if modID == 0 && len(args) == 1 {
			parsedCategory, parsedSlug, parsedFileID, err := sources.CurseforgeParseUrl(args[0])
			if err != nil {
				shared.Exitf("Failed to parse URL: %v\n", err)
			}

			if parsedCategory != "" {
				category = parsedCategory
			}
			if parsedSlug != "" {
				slug = parsedSlug
			}
			if parsedFileID != 0 {
				fileID = parsedFileID
			}
		}

		modInfoObtained := false
		var modInfoData sources.CfModInfo

		if modID == 0 {
			var cancelled bool
			if slug == "" {
				searchTerm := strings.Join(args, " ")
				cancelled, modInfoData = SearchCurseforgeInternal(searchTerm, false, category, mcVersions, sources.CfGetSearchLoaderType(*pack))
			} else {
				cancelled, modInfoData = SearchCurseforgeInternal(slug, true, category, mcVersions, sources.CfGetSearchLoaderType(*pack))
			}
			if cancelled {
				return
			}
			modID = modInfoData.ID
			modInfoObtained = true
		}

		if modID == 0 {
			shared.Exitln("No projects found!")
		}

		if !modInfoObtained {
			modInfoData, err = sources.GetCurseforgeClient().GetModInfo(modID)
			if err != nil {
				shared.Exitf("Failed to get project info: %v\n", err)
			}
		}

		fileInfoData, err := sources.GetLatestFile(modInfoData, mcVersions, fileID, pack.GetCompatibleLoaders())
		if err != nil {
			shared.Exitf("Failed to get file for project: %v\n", err)
		}

		var missingDependencies []*core.Mod
		if len(fileInfoData.Dependencies) > 0 {

			missingDependencies, err = sources.CurseforgeFindMissingDependencies(*pack, fileInfoData, primaryMCVersion)
			if err != nil {
				shared.Exitln(err)
			}

			if len(missingDependencies) > 0 {
				fmt.Println("Dependencies found:")
				for _, v := range missingDependencies {
					fmt.Println(v.Slug)
				}

				if !shared.PromptYesNo("Would you like to add them? [Y/n]: ") {
					// if NO is chosen then we'll nil the slice to prevent installing
					missingDependencies = nil
				}
			}
		}

		mainMod, err := sources.CurseforgeNewMod(modInfoData, fileInfoData, false)
		if err != nil {
			shared.Exitln(err)
		}

		newMods := append(missingDependencies, mainMod)

		if len(newMods) == 0 {
			shared.Exitln("no mods were installed")
		}

		for _, mod := range newMods {
			pack.SetMod(mod)
		}

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitln(err)
		}

		fmt.Printf("Project \"%s\" successfully added! (%s)\n", modInfoData.Name, fileInfoData.FileName)
	},
}

// Used to implement interface for fuzzy matching
type ModResultsList []sources.CfModInfo

func (r ModResultsList) String(i int) string {
	return r[i].Name
}

func (r ModResultsList) Len() int {
	return len(r)
}

func SearchCurseforgeInternal(
	searchTerm string,
	isSlug bool,
	category string,
	mcVersions []string,
	searchLoaderType sources.ModloaderType,
) (bool, sources.CfModInfo) {
	if isSlug {
		fmt.Println("Looking up CurseForge slug...")
	} else {
		fmt.Println("Searching CurseForge...")
	}

	var categoryID, classID uint32

	if category == "mc-mods" {
		classID = 6
	}

	if classID == 0 && category != "" {
		var err error
		categoryID, classID, err = sources.CurseforgeCategoryLookup(category)
		if err != nil {
			shared.Exitln(err)
		}
	}

	// If there are more than one acceptable version, we shouldn't filter by game version at all (as we can't filter by multiple)
	filterGameVersion := ""
	if len(mcVersions) == 1 {
		filterGameVersion = sources.GetCurseforgeVersion(mcVersions[0])
	}
	var search, slug string
	if isSlug {
		slug = searchTerm
	} else {
		search = searchTerm
	}
	results, err := sources.GetCurseforgeClient().GetSearch(search, slug, classID, categoryID, filterGameVersion, searchLoaderType)
	if err != nil {
		shared.Exitf("Failed to search for project: %v\n", err)
	}
	if len(results) == 0 {
		shared.Exitln("No projects found!")
		return false, sources.CfModInfo{}
	} else if len(results) == 1 {
		return false, results[0]
	} else {
		// Fuzzy search on results list
		fuzzySearchResults := fuzzy.FindFrom(searchTerm, ModResultsList(results))

		if viper.GetBool("non-interactive") {
			if len(fuzzySearchResults) > 0 {
				return false, results[fuzzySearchResults[0].Index]
			}
			return false, results[0]
		}

		menu := wmenu.NewMenu("Choose a number:")

		menu.Option("Cancel", nil, false, nil)
		if len(fuzzySearchResults) == 0 {
			for i, v := range results {
				menu.Option(v.Name+" ("+v.Summary+")", v, i == 0, nil)
			}
		} else {
			for i, v := range fuzzySearchResults {
				menu.Option(results[v.Index].Name+" ("+results[v.Index].Summary+")", results[v.Index], i == 0, nil)
			}
		}

		var modInfoData sources.CfModInfo
		var cancelled bool
		menu.Action(func(menuRes []wmenu.Opt) error {
			if len(menuRes) != 1 || menuRes[0].Value == nil {
				fmt.Println("Cancelled!")
				cancelled = true
				return nil
			}

			// Why is variable shadowing a thing!!!!
			var ok bool
			modInfoData, ok = menuRes[0].Value.(sources.CfModInfo)
			if !ok {
				return errors.New("error converting interface from wmenu")
			}
			return nil
		})
		err = menu.Run()
		if err != nil {
			shared.Exitln(err)
		}

		if cancelled {
			return true, sources.CfModInfo{}
		}
		return false, modInfoData
	}
}
