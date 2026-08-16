package sources

import (
	"errors"
	"fmt"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/mitchellh/mapstructure"
)

func init() {
	core.AddUpdater(ghUpdater{})
}

type ghUpdateData struct {
	Slug   string `mapstructure:"slug"`
	Tag    string `mapstructure:"tag"`
	Branch string `mapstructure:"branch"`
	Regex  string `mapstructure:"regex"`
}

type ghUpdater struct{}

func (u ghUpdater) GetName() string {
	return "github"
}

func (u ghUpdater) ParseUpdate(updateUnparsed map[string]interface{}) (interface{}, error) {
	var updateData ghUpdateData
	err := mapstructure.Decode(updateUnparsed, &updateData)
	return updateData, err
}

type ghCachedStateStore struct {
	Slug  string
	Tag   string
	Asset Asset
}

func (u ghUpdater) CheckUpdate(mods []*core.Mod, _ core.Pack) ([]core.UpdateCheck, error) {
	results := make([]core.UpdateCheck, len(mods))

	for i, mod := range mods {

		var data ghUpdateData
		err := mod.DecodeNamedModSourceData("github", &data)
		if err != nil {
			results[i] = core.UpdateCheck{Error: errors.New("failed to parse update metadata")}
			continue
		}

		newRelease, err := getLatestRelease(data.Slug, data.Branch)
		if err != nil {
			results[i] = core.UpdateCheck{Error: fmt.Errorf("failed to get latest release: %v", err)}
			continue
		}

		if newRelease.TagName == data.Tag { // The latest release is the same as the installed one
			results[i] = core.UpdateCheck{UpdateAvailable: false}
			continue
		}

		newFile, err := selectReleaseAsset(newRelease.Assets, data.Regex)
		if err != nil {
			results[i] = core.UpdateCheck{Error: err}
			continue
		}

		results[i] = core.UpdateCheck{
			UpdateAvailable: true,
			UpdateString:    mod.FileName + " -> " + newFile.Name,
			CachedState:     ghCachedStateStore{data.Slug, newRelease.TagName, newFile},
		}
	}

	return results, nil
}

func (u ghUpdater) DoUpdate(mods []*core.Mod, cachedState []interface{}) error {
	for i, mod := range mods {
		modState := cachedState[i].(ghCachedStateStore)
		file := modState.Asset

		hash, err := file.getSha256()
		if err != nil {
			return err
		}

		mod.FileName = file.Name
		mod.Download = core.ModDownload{
			URL:        file.BrowserDownloadURL,
			HashFormat: "sha256",
			Hash:       hash,
		}
		mod.Update["github"]["tag"] = modState.Tag
	}

	return nil
}
