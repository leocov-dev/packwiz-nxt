package core

import "fmt"

// named update source to mod list
type UpdateSourceMap map[string][]*Mod

// resolveRegistry returns reg if non-nil, otherwise falls back to DefaultRegistry.
// This lets callers pass nil to preserve the CLI's historical behavior of using
// the process-wide default registry.
func resolveRegistry(reg *Registry) *Registry {
	if reg == nil {
		return DefaultRegistry
	}
	return reg
}

// BuildUpdateMap groups mods by their configured update source, keeping only
// sources for which an Updater is registered in reg (or DefaultRegistry, if
// reg is nil).
func BuildUpdateMap(reg *Registry, mods []*Mod) UpdateSourceMap {
	reg = resolveRegistry(reg)

	filesWithUpdater := make(UpdateSourceMap)
	fmt.Println("Reading metadata files...")

	for _, modData := range mods {
		updaterFound := false
		for k := range modData.Update {
			slice, ok := filesWithUpdater[k]
			if !ok {
				_, ok = reg.GetUpdater(k)
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

// GetUpdatableMods checks all of pack's mods for available updates, using the
// Updaters registered in reg (or DefaultRegistry, if reg is nil).
func GetUpdatableMods(reg *Registry, pack Pack) (UpdateDataList, error) {
	reg = resolveRegistry(reg)

	updatable := make(UpdateDataList)

	updateMap := BuildUpdateMap(reg, pack.GetModsList())

	for source, mods := range updateMap {
		updater, ok := reg.GetUpdater(source)
		if !ok {
			return nil, fmt.Errorf("no updater registered for source: %s", source)
		}
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

// UpdateSingleMod checks for and applies an update to a single mod, using the
// Updaters registered in reg (or DefaultRegistry, if reg is nil).
func UpdateSingleMod(reg *Registry, pack Pack, mod *Mod) error {
	reg = resolveRegistry(reg)

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

		return updateMods(reg, updateData)
	}
}

// UpdateAllMods checks for and applies updates to all of pack's mods, using
// the Updaters registered in reg (or DefaultRegistry, if reg is nil).
func UpdateAllMods(reg *Registry, pack Pack) error {
	reg = resolveRegistry(reg)

	updateData, err := GetUpdatableMods(reg, pack)
	if err != nil {
		return err
	}

	if len(updateData) == 0 {
		fmt.Println("all mods already up to date")
		return nil
	}

	return updateMods(reg, updateData)
}

func updateMods(reg *Registry, updateData UpdateDataList) error {
	reg = resolveRegistry(reg)

	for source, data := range updateData {
		updater, ok := reg.GetUpdater(source)
		if !ok {
			return fmt.Errorf("no updater registered for source: %s", source)
		}

		if err := updater.DoUpdate(data.Mods, data.CachedState); err != nil {
			return err
		}
	}

	return nil
}
