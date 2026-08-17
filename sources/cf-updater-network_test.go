package sources

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

// withCfClient swaps the package-level CurseForge client singleton for the
// duration of the test, restoring the original afterward.
func withCfClient(t *testing.T, httpClient *http.Client) {
	t.Helper()
	withTestCfApiKey(t)
	original := cfDefaultClient
	cfDefaultClient = *NewCfApiClient(httpClient, core.NoopLogger{})
	t.Cleanup(func() { cfDefaultClient = original })
}

func cfTestMod(name string, projectID, fileID uint32) *core.Mod {
	return &core.Mod{
		Name:     name,
		FileName: "old.jar",
		Update: core.ModUpdate{
			"curseforge": core.ModSourceData{
				"project-id": projectID,
				"file-id":    fileID,
			},
		},
	}
}

func TestCfUpdater_CheckUpdate(t *testing.T) {
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	t.Run("update available when a newer file exists", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":1,"latestFiles":[{"id":2,"fileName":"new.jar","gameVersions":["1.20.1"]}]}]}`))
		}))
		withCfClient(t, httpClient)

		mods := []*core.Mod{cfTestMod("Test Mod", 1, 1)}
		results, err := CfUpdater{}.CheckUpdate(mods, pack)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].UpdateAvailable)
		assert.Equal(t, "old.jar -> new.jar", results[0].UpdateString)
	})

	t.Run("no update available when current file is already latest", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":1,"latestFiles":[{"id":2,"fileName":"new.jar","gameVersions":["1.20.1"]}]}]}`))
		}))
		withCfClient(t, httpClient)

		mods := []*core.Mod{cfTestMod("Test Mod", 1, 2)}
		results, err := CfUpdater{}.CheckUpdate(mods, pack)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].UpdateAvailable)
	})

	t.Run("per-mod decode failure is reported without failing the whole batch", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		}))
		withCfClient(t, httpClient)

		badMod := &core.Mod{Name: "Bad Mod", Update: core.ModUpdate{"curseforge": core.ModSourceData{"project-id": "not-a-number"}}}
		results, err := CfUpdater{}.CheckUpdate([]*core.Mod{badMod}, pack)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Error(t, results[0].Error)
	})
}

func TestCfUpdater_DoUpdate(t *testing.T) {
	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":2,"modId":1,"fileName":"new.jar"}}`))
	}))
	withCfClient(t, httpClient)

	mod := cfTestMod("Test Mod", 1, 1)
	cachedState := []interface{}{
		cachedStateStore{CfModInfo{ID: 1, Name: "Test Mod"}, 2, nil},
	}

	err := CfUpdater{}.DoUpdate([]*core.Mod{mod}, cachedState)
	require.NoError(t, err)
	assert.Equal(t, "new.jar", mod.FileName)
	assert.Equal(t, uint32(1), mod.Update["curseforge"]["project-id"])
	assert.Equal(t, uint32(2), mod.Update["curseforge"]["file-id"])
}

func TestCfDownloader_GetFilesMetadata(t *testing.T) {
	t.Run("direct download URL populates CfDownloadMetadata", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":2,"modId":1,"fileName":"new.jar","downloadUrl":"https://example.com/new.jar"}]}`))
		}))
		withCfClient(t, httpClient)

		mods := []*core.Mod{cfTestMod("Test Mod", 1, 2)}
		data, err := CfDownloader{}.GetFilesMetadata(mods)
		require.NoError(t, err)
		require.Len(t, data, 1)
		manual, _ := data[0].GetManualDownload()
		assert.False(t, manual)
	})

	t.Run("empty download URL requires manual download", func(t *testing.T) {
		callCount := 0
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
			if callCount == 1 {
				_, _ = w.Write([]byte(`{"data":[{"id":2,"modId":1,"fileName":"new.jar","downloadUrl":""}]}`))
			} else {
				_, _ = w.Write([]byte(`{"data":[{"id":1,"name":"Test Mod","links":{"websiteUrl":"https://curseforge.com/test-mod"}}]}`))
			}
		}))
		withCfClient(t, httpClient)

		mods := []*core.Mod{cfTestMod("Test Mod", 1, 2)}
		data, err := CfDownloader{}.GetFilesMetadata(mods)
		require.NoError(t, err)
		require.Len(t, data, 1)
		manual, info := data[0].GetManualDownload()
		assert.True(t, manual)
		assert.Equal(t, "Test Mod", info.Name)
	})

	t.Run("empty mods returns empty slice", func(t *testing.T) {
		data, err := CfDownloader{}.GetFilesMetadata(nil)
		require.NoError(t, err)
		assert.Empty(t, data)
	})
}
