package cmd

import (
	"fmt"
	"github.com/leocov-dev/fork.packwiz/fileio"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/leocov-dev/fork.packwiz/core"
	"github.com/leocov-dev/fork.packwiz/internal/cmdshared"
)

// UpdateCmd represents the update command
var UpdateCmd = &cobra.Command{
	Use:     "update [name]",
	Short:   "Update an external file (or all external files) in the modpack",
	Aliases: []string{"upgrade"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: --check flag?
		// TODO: specify multiple files to update at once?

		fmt.Println("Loading modpack...")
		pack, err := core.LoadPack()
		if err != nil {
			cmdshared.Exitln(err)
		}
		index, err := pack.LoadIndex()
		if err != nil {
			cmdshared.Exitln(err)
		}

		var singleUpdatedName string
		if viper.GetBool("update.all") {
			filesWithUpdater := make(map[string][]*core.Mod)
			fmt.Println("Reading metadata files...")
			mods, err := index.LoadAllMods()
			if err != nil {
				cmdshared.Exitf("Failed to update all files: %v\n", err)
			}
			for _, modData := range mods {
				updaterFound := false
				for k := range modData.Update {
					slice, ok := filesWithUpdater[k]
					if !ok {
						_, ok = core.Updaters[k]
						if !ok {
							continue
						}
						slice = []*core.Mod{}
					}
					updaterFound = true
					filesWithUpdater[k] = append(slice, modData)
				}
				if !updaterFound {
					fmt.Printf("A supported update system for \"%s\" cannot be found.\n", modData.Name)
				}
			}

			fmt.Println("Checking for updates...")
			updatesFound := false
			updatableFiles := make(map[string][]*core.Mod)
			updaterCachedStateMap := make(map[string][]interface{})
			for k, v := range filesWithUpdater {
				checks, err := core.Updaters[k].CheckUpdate(v, pack)
				if err != nil {
					// TODO: do we return err code 1?
					fmt.Printf("Failed to check updates for %s: %s\n", k, err.Error())
					continue
				}
				for i, check := range checks {
					if check.Error != nil {
						// TODO: do we return err code 1?
						fmt.Printf("Failed to check updates for %s: %s\n", v[i].Name, check.Error.Error())
						continue
					}
					if check.UpdateAvailable {
						if v[i].Pin {
							fmt.Printf("Update skipped for pinned mod %s\n", v[i].Name)
							continue
						}

						if !updatesFound {
							fmt.Println("Updates found:")
							updatesFound = true
						}
						fmt.Printf("%s: %s\n", v[i].Name, check.UpdateString)
						updatableFiles[k] = append(updatableFiles[k], v[i])
						updaterCachedStateMap[k] = append(updaterCachedStateMap[k], check.CachedState)
					}
				}
			}

			if !updatesFound {
				fmt.Println("All files are up to date!")
				return
			}

			if !cmdshared.PromptYesNo("Do you want to update? [Y/n]: ") {
				fmt.Println("Cancelled!")
				return
			}

			for k, v := range updatableFiles {
				err := core.Updaters[k].DoUpdate(v, updaterCachedStateMap[k])
				if err != nil {
					// TODO: do we return err code 1?
					fmt.Println(err.Error())
					continue
				}
				for _, modData := range v {
					modWriter := fileio.NewModWriter()
					format, hash, err := modWriter.Write(modData)
					if err != nil {
						fmt.Println(err.Error())
						continue
					}

					err = index.RefreshFileWithHash(modData.GetFilePath(), format, hash, true)
					if err != nil {
						fmt.Println(err.Error())
						continue
					}
				}
			}
		} else {
			if len(args) < 1 || len(args[0]) == 0 {
				cmdshared.Exitln("Must specify a valid file, or use the --all flag!")
			}
			modPath, ok := index.FindMod(args[0])
			if !ok {
				cmdshared.Exitln("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
			}
			modData, err := core.LoadMod(modPath)
			if err != nil {
				cmdshared.Exitln(err)
			}
			if modData.Pin {
				cmdshared.Exitln("Version is pinned; run the unpin command to allow updating")
			}
			singleUpdatedName = modData.Name
			updaterFound := false
			for k := range modData.Update {
				updater, ok := core.Updaters[k]
				if !ok {
					continue
				}
				updaterFound = true

				check, err := updater.CheckUpdate([]*core.Mod{&modData}, pack)
				if err != nil {
					cmdshared.Exitln(err)
				}
				if len(check) != 1 {
					cmdshared.Exitln("Invalid update check response")
				}

				if check[0].UpdateAvailable {
					fmt.Printf("Update available: %s\n", check[0].UpdateString)

					err = updater.DoUpdate([]*core.Mod{&modData}, []interface{}{check[0].CachedState})
					if err != nil {
						cmdshared.Exitln(err)
					}

					modWriter := fileio.NewModWriter()
					format, hash, err := modWriter.Write(&modData)
					if err != nil {
						cmdshared.Exitln(err)
					}

					err = index.RefreshFileWithHash(modPath, format, hash, true)
					if err != nil {
						cmdshared.Exitln(err)
					}
				} else {
					fmt.Printf("\"%s\" is already up to date!\n", modData.Name)
					return
				}

				break
			}
			if !updaterFound {
				// TODO: use file name instead of Name when len(Name) == 0 in all places?
				cmdshared.Exitln("A supported update system for \"" + modData.Name + "\" cannot be found.")
			}
		}

		err = index.Write()
		if err != nil {
			cmdshared.Exitln(err)
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			cmdshared.Exitln(err)
		}
		err = pack.Write()
		if err != nil {
			cmdshared.Exitln(err)
		}
		if viper.GetBool("update.all") {
			fmt.Println("Files updated!")
		} else {
			fmt.Printf("\"%s\" updated!\n", singleUpdatedName)
		}
	},
}

func init() {
	rootCmd.AddCommand(UpdateCmd)

	UpdateCmd.Flags().BoolP("all", "a", false, "Update all external files")
	_ = viper.BindPFlag("update.all", UpdateCmd.Flags().Lookup("all"))
}
