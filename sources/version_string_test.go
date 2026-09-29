package sources

import (
	"testing"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/stretchr/testify/assert"
)

func strp(s string) *string { return &s }

func TestModrinthVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version *modrinthApi.Version
		want    string
	}{
		{"nil", nil, ""},
		{"version number", &modrinthApi.Version{VersionNumber: strp("1.2.3"), Name: strp("Release 1.2.3")}, "1.2.3"},
		{"falls back to name", &modrinthApi.Version{Name: strp("Release 1.2.3")}, "Release 1.2.3"},
		{"empty number falls back to name", &modrinthApi.Version{VersionNumber: strp(""), Name: strp("N")}, "N"},
		{"neither", &modrinthApi.Version{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ModrinthVersionString(tt.version))
		})
	}
}

func TestCurseforgeVersionString(t *testing.T) {
	tests := []struct {
		name string
		file CfModFileInfo
		want string
	}{
		{"display name", CfModFileInfo{FriendlyName: "Balm 21.5.14", FileName: "balm.jar"}, "Balm 21.5.14"},
		{"falls back to file name", CfModFileInfo{FileName: "balm.jar"}, "balm.jar"},
		{"neither", CfModFileInfo{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CurseforgeVersionString(tt.file))
		})
	}
}

func TestGitHubVersionString(t *testing.T) {
	assert.Equal(t, "v1.0.0", GitHubVersionString(Release{TagName: "v1.0.0", Name: "First"}))
	assert.Equal(t, "", GitHubVersionString(Release{}))
}

func TestCurseforgeNewModSetsVersion(t *testing.T) {
	mod, err := CurseforgeNewMod(
		CfModInfo{ID: 1, Slug: "balm", Name: "Balm"},
		CfModFileInfo{ID: 2, FileName: "balm.jar", FriendlyName: "Balm 1.0"},
		false,
	)
	assert.NoError(t, err)
	assert.Equal(t, "Balm 1.0", mod.Version)
}

func TestCfDoUpdateSetsVersion(t *testing.T) {
	mod := &core.Mod{Name: "Balm", Update: core.ModUpdate{"curseforge": map[string]interface{}{}}}
	state := cachedStateStore{
		CfModInfo: CfModInfo{ID: 1, Name: "Balm"},
		fileID:    2,
		fileInfo:  &CfModFileInfo{ID: 2, FileName: "balm.jar", FriendlyName: "Balm 2.0"},
	}
	assert.NoError(t, CfUpdater{}.DoUpdate([]*core.Mod{mod}, []interface{}{state}))
	assert.Equal(t, "Balm 2.0", mod.Version)
}

func TestMrDoUpdateSetsVersion(t *testing.T) {
	mod := &core.Mod{Name: "Mod", Update: core.ModUpdate{"modrinth": map[string]interface{}{}}}
	yes := true
	url := "https://cdn/x.jar"
	fn := "x.jar"
	v := &modrinthApi.Version{
		ID:            strp("vid"),
		VersionNumber: strp("3.1.0"),
		Files: []*modrinthApi.File{{
			Primary:  &yes,
			URL:      &url,
			Filename: &fn,
			Hashes:   map[string]string{"sha512": "abc"},
		}},
	}
	state := mrCachedStateStore{ProjectID: "p", Version: v}
	assert.NoError(t, mrUpdater{}.DoUpdate([]*core.Mod{mod}, []interface{}{state}))
	assert.Equal(t, "3.1.0", mod.Version)
}
