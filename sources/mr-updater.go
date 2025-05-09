package sources

import (
	"errors"
	"fmt"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/mitchellh/mapstructure"
)

func init() {
	core.AddUpdater(mrUpdater{})
}

type mrUpdateData struct {
	// TODO(format): change to "project-id"
	ProjectID string `mapstructure:"mod-id"`
	// TODO(format): change to "version-id"
	InstalledVersion string `mapstructure:"version"`
}

func (u mrUpdateData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}

type mrUpdater struct{}

func (u mrUpdater) GetName() string {
	return "modrinth"
}

func (u mrUpdater) ParseUpdate(updateUnparsed map[string]interface{}) (interface{}, error) {
	var updateData mrUpdateData
	err := mapstructure.Decode(updateUnparsed, &updateData)
	return updateData, err
}

type mrCachedStateStore struct {
	ProjectID string
	Version   *modrinthApi.Version
}

func (u mrUpdater) CheckUpdate(mods []*core.Mod, pack core.Pack) ([]core.UpdateCheck, error) {
	results := make([]core.UpdateCheck, len(mods))

	for i, mod := range mods {
		var data mrUpdateData
		err := mod.DecodeNamedModSourceData("modrinth", &data)
		if err != nil {
			results[i] = core.UpdateCheck{Error: errors.New("failed to parse update metadata")}
			continue
		}

		newVersion, err := GetModrinthLatestVersion(data.ProjectID, mod.Name, pack)
		if err != nil {
			results[i] = core.UpdateCheck{Error: fmt.Errorf("failed to get latest version: %v", err)}
			continue
		}

		if *newVersion.ID == data.InstalledVersion { //The latest version from the site is the same as the installed one
			results[i] = core.UpdateCheck{UpdateAvailable: false}
			continue
		}

		if len(newVersion.Files) == 0 {
			results[i] = core.UpdateCheck{Error: errors.New("new version doesn't have any files")}
			continue
		}

		newFilename := newVersion.Files[0].Filename
		// Prefer the primary file
		for _, v := range newVersion.Files {
			if *v.Primary {
				newFilename = v.Filename
			}
		}

		results[i] = core.UpdateCheck{
			UpdateAvailable: true,
			UpdateString:    mod.FileName + " -> " + *newFilename,
			CachedState:     mrCachedStateStore{data.ProjectID, newVersion},
		}
	}

	return results, nil
}

func (u mrUpdater) DoUpdate(mods []*core.Mod, cachedState []interface{}) error {
	for i, mod := range mods {
		modState := cachedState[i].(mrCachedStateStore)
		var version = modState.Version

		var file = version.Files[0]
		// Prefer the primary file
		for _, v := range version.Files {
			if *v.Primary {
				file = v
			}
		}

		algorithm, hash := mrGetBestHash(file)
		if algorithm == "" {
			return errors.New("file for project " + mod.Name + " doesn't have a valid hash")
		}

		mod.FileName = *file.Filename
		mod.Download = core.ModDownload{
			URL:        *file.URL,
			HashFormat: algorithm,
			Hash:       hash,
		}
		mod.Update["modrinth"]["version"] = version.ID
	}

	return nil
}
