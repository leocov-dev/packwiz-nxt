package sources

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

type cfFileInfoDependency = struct {
	ModID uint32         `json:"modId"`
	Type  dependencyType `json:"relationType"`
}

func TestBuildDependencyQueue(t *testing.T) {
	t.Run("only required dependencies are included", func(t *testing.T) {
		fileInfo := CfModFileInfo{
			Dependencies: []cfFileInfoDependency{
				{ModID: 1, Type: dependencyTypeOptional},
				{ModID: 2, Type: DependencyTypeRequired},
				{ModID: 3, Type: dependencyTypeTool},
				{ModID: 4, Type: DependencyTypeRequired},
			},
		}
		got := buildDependencyQueue(fileInfo, "1.20.1", false)
		assert.Equal(t, []uint32{2, 4}, got)
	})

	t.Run("no dependencies returns empty", func(t *testing.T) {
		got := buildDependencyQueue(CfModFileInfo{}, "1.20.1", false)
		assert.Empty(t, got)
	})

	t.Run("quilt override applied when applicable", func(t *testing.T) {
		fileInfo := CfModFileInfo{
			Dependencies: []cfFileInfoDependency{
				{ModID: 306612, Type: DependencyTypeRequired}, // Fabric API
			},
		}
		got := buildDependencyQueue(fileInfo, "1.20.1", true)
		assert.Equal(t, []uint32{634179}, got)
	})
}

func TestCurseforgeNewMod(t *testing.T) {
	modInfo := CfModInfo{
		ID:                12345,
		Slug:              "jei",
		Name:              "JEI",
		GameID:            minecraftGameId,
		ClassID:           6, // mods
		PrimaryCategoryID: 0,
	}
	fileInfo := CfModFileInfo{
		ID:          6789,
		FileName:    "jei-1.20.1.jar",
		Fingerprint: 111,
	}

	t.Run("builds a mod with expected fields", func(t *testing.T) {
		mod, err := CurseforgeNewMod(modInfo, fileInfo, false)
		require.NoError(t, err)
		assert.Equal(t, "jei", mod.Slug)
		assert.Equal(t, "JEI", mod.Name)
		assert.Equal(t, "jei-1.20.1.jar", mod.FileName)
		assert.Equal(t, "both", string(mod.Side))
		assert.Equal(t, "mods", mod.ModType)
		assert.Equal(t, "111", mod.Download.Hash)
		assert.Equal(t, "murmur2", mod.Download.HashFormat)
		assert.Nil(t, mod.Option)
	})

	t.Run("optionalDisabled true sets Option", func(t *testing.T) {
		mod, err := CurseforgeNewMod(modInfo, fileInfo, true)
		require.NoError(t, err)
		require.NotNil(t, mod.Option)
		assert.True(t, mod.Option.Optional)
		assert.False(t, mod.Option.Default)
	})
}

func TestCreateCurseforgeDependencies(t *testing.T) {
	deps := []CfInstallableDep{
		{
			CfModInfo: CfModInfo{ID: 1, Slug: "a", Name: "A", ClassID: 6},
			FileInfo:  CfModFileInfo{ID: 10, FileName: "a.jar"},
		},
		{
			CfModInfo: CfModInfo{ID: 2, Slug: "b", Name: "B", ClassID: 6},
			FileInfo:  CfModFileInfo{ID: 20, FileName: "b.jar"},
		},
	}

	mods, err := CreateCurseforgeDependencies(deps)
	require.NoError(t, err)
	require.Len(t, mods, 2)
	assert.Equal(t, "a", mods[0].Slug)
	assert.Equal(t, "b", mods[1].Slug)
}

func TestGetLatestFile(t *testing.T) {
	t.Run("fileID 0 with LatestFiles present needs no network call", func(t *testing.T) {
		modInfo := CfModInfo{
			LatestFiles: []CfModFileInfo{{ID: 1, FileName: "a.jar", GameVersions: []string{"1.20.1"}}},
		}
		fileInfo, err := GetLatestFile(modInfo, []string{"1.20.1"}, 0, nil)
		require.NoError(t, err)
		assert.Equal(t, "a.jar", fileInfo.FileName)
	})

	t.Run("explicit fileID fetches file info over the network", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"id":5,"modId":1,"fileName":"pinned.jar"}}`))
		}))
		withCfClient(t, httpClient)

		fileInfo, err := GetLatestFile(CfModInfo{ID: 1}, []string{"1.20.1"}, 5, nil)
		require.NoError(t, err)
		assert.Equal(t, "pinned.jar", fileInfo.FileName)
	})

	t.Run("no files at all is an error", func(t *testing.T) {
		_, err := GetLatestFile(CfModInfo{ID: 1}, []string{"1.20.1"}, 0, nil)
		assert.Error(t, err)
	})
}

func TestCurseforgeFindMissingDependencies(t *testing.T) {
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	fileInfoData := CfModFileInfo{
		Dependencies: []cfFileInfoDependency{
			{ModID: 99, Type: DependencyTypeRequired},
		},
	}

	t.Run("resolves a single required dependency", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":99,"slug":"dep-mod","name":"Dep Mod","classId":6,"latestFiles":[{"id":100,"fileName":"dep.jar","gameVersions":["1.20.1"]}]}]}`))
		}))
		withCfClient(t, httpClient)

		mods, err := CurseforgeFindMissingDependencies(pack, fileInfoData, "1.20.1")
		require.NoError(t, err)
		require.Len(t, mods, 1)
		assert.Equal(t, "dep-mod", mods[0].Slug)
		assert.Equal(t, "dep.jar", mods[0].FileName)
	})

	t.Run("no required dependencies returns no mods, no network call", func(t *testing.T) {
		mods, err := CurseforgeFindMissingDependencies(pack, CfModFileInfo{}, "1.20.1")
		require.NoError(t, err)
		assert.Empty(t, mods)
	})
}
