package core

import "fmt"

// named update source to mod list
type UpdateSourceMap map[string][]*Mod

func BuildUpdateMap(mods []*Mod) UpdateSourceMap {
	filesWithUpdater := make(UpdateSourceMap)
	fmt.Println("Reading metadata files...")

	for _, modData := range mods {
		updaterFound := false
		for k := range modData.Update {
			slice, ok := filesWithUpdater[k]
			if !ok {
				_, ok = GetUpdater(k)
				if !ok {
					continue
				}
				slice = []*Mod{}
			}
			updaterFound = true
			filesWithUpdater[k] = append(slice, modData)
		}
		if !updaterFound {
			fmt.Printf("A supported update system for \"%s\" cannot be found.\n", modData.Name)
		}
	}

	return filesWithUpdater
}

type UpdateData struct {
	Mods        []*Mod
	CachedState []interface{}
}

type UpdateDataList map[string]UpdateData

func (ud UpdateDataList) Append(source string, mod *Mod, cachedState interface{}) {
	data, ok := ud[source]
	if !ok {
		data = UpdateData{
			Mods:        []*Mod{},
			CachedState: []interface{}{},
		}
	}

	data.Mods = append(data.Mods, mod)
	data.CachedState = append(data.CachedState, cachedState)

	ud[source] = data
}

func GetUpdatableMods(pack Pack) (UpdateDataList, error) {
	updatable := make(UpdateDataList)

	updateMap := BuildUpdateMap(pack.GetModsList())

	for source, mods := range updateMap {
		updater, _ := GetUpdater(source)
		checks, err := updater.CheckUpdate(mods, pack)
		if err != nil {
			return nil, err
		}

		for i, check := range checks {
			mod := mods[i]

			if check.Error != nil {
				return nil, fmt.Errorf("failed to check for updates for mod: %s - %s\n", mod.Slug, check.Error.Error())
			}

			if check.UpdateAvailable {
				if mod.Pin {
					fmt.Printf("skipping pinned mod: %s\n", mod.Slug)
					continue
				}

				updatable.Append(source, mod, check.CachedState)
			}
		}
	}

	return updatable, nil
}

func UpdateSingleMod(pack Pack, mod *Mod) error {
	updater, err := mod.GetUpdater()
	if err != nil {
		return err
	}
	checks, err := updater.CheckUpdate([]*Mod{mod}, pack)
	if err != nil {
		return err
	}
	if len(checks) != 1 {
		return fmt.Errorf("invalid update check response for mod: %s", mod.Name)
	}
	check := checks[0]

	if !check.UpdateAvailable {
		fmt.Printf("mod: %s is alreay up to date\n", mod.Name)
		return nil
	} else {
		updateData := make(UpdateDataList)
		updateData.Append(updater.GetName(), mod, check.CachedState)

		return UpdateMods(updateData)
	}
}

func UpdateMods(updateData UpdateDataList) error {
	for source, data := range updateData {
		updater, _ := GetUpdater(source)

		if err := updater.DoUpdate(data.Mods, data.CachedState); err != nil {
			return err
		}
	}

	return nil
}
