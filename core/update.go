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
	reg.logger.Infof("Reading metadata files...\n")

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
			reg.logger.Warnf("A supported update system for \"%s\" cannot be found.\n", modData.Name)
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

// UpdateCheckResult carries the per-mod outcome of a dry-run compatibility
// check, as returned by CheckAllMods. Err is non-nil if this mod's check
// failed - the other check-result fields are then zero-valued and should be
// ignored.
type UpdateCheckResult struct {
	Mod             *Mod
	Source          string
	UpdateAvailable bool
	UpdateString    string
	CachedState     any
	Err             error
}

// CheckAllMods checks all of pack's mods for available updates, using the
// Updaters registered in reg (or DefaultRegistry, if reg is nil). Unlike
// GetUpdatableMods, a failure checking one mod (or one source's whole batch)
// does not abort the rest of the check - it is reported on the affected
// mod(s)' UpdateCheckResult.Err instead, so a single unresolvable mod doesn't
// hide the results for every other mod in the pack. Pinned mods are still
// included in the results (CheckAllMods does not filter them, unlike
// GetUpdatableMods) so a caller can distinguish "pinned, update available but
// would be skipped" from "up to date".
func CheckAllMods(reg *Registry, pack Pack) ([]UpdateCheckResult, error) {
	reg = resolveRegistry(reg)

	var results []UpdateCheckResult

	updateMap := BuildUpdateMap(reg, pack.GetModsList())

	for source, mods := range updateMap {
		updater, ok := reg.GetUpdater(source)
		if !ok {
			return nil, fmt.Errorf("no updater registered for source: %s", source)
		}
		checks, err := updater.CheckUpdate(mods, pack)
		if err != nil {
			for _, mod := range mods {
				results = append(results, UpdateCheckResult{Mod: mod, Source: source, Err: err})
			}
			continue
		}

		for i, check := range checks {
			results = append(results, UpdateCheckResult{
				Mod:             mods[i],
				Source:          source,
				UpdateAvailable: check.UpdateAvailable,
				UpdateString:    check.UpdateString,
				CachedState:     check.CachedState,
				Err:             check.Error,
			})
		}
	}

	return results, nil
}

// GetUpdatableMods checks all of pack's mods for available updates, using the
// Updaters registered in reg (or DefaultRegistry, if reg is nil).
func GetUpdatableMods(reg *Registry, pack Pack) (UpdateDataList, error) {
	reg = resolveRegistry(reg)

	results, err := CheckAllMods(reg, pack)
	if err != nil {
		return nil, err
	}

	updatable := make(UpdateDataList)

	for _, r := range results {
		if r.Err != nil {
			return nil, fmt.Errorf("failed to check for updates for mod: %s - %s\n", r.Mod.Slug, r.Err.Error())
		}

		if r.UpdateAvailable {
			if r.Mod.Pin {
				reg.logger.Infof("skipping pinned mod: %s\n", r.Mod.Slug)
				continue
			}

			updatable.Append(r.Source, r.Mod, r.CachedState)
		}
	}

	return updatable, nil
}

// UpdateSingleMod checks for and applies an update to a single mod, using the
// Updaters registered in reg (or DefaultRegistry, if reg is nil).
func UpdateSingleMod(reg *Registry, pack Pack, mod *Mod) error {
	reg = resolveRegistry(reg)

	updater, err := mod.GetUpdater(reg)
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
		reg.logger.Infof("mod: %s is already up to date\n", mod.Name)
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
		reg.logger.Infof("all mods already up to date\n")
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
