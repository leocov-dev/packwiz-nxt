package sources

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

// withMrClient starts an httptest.Server serving handler and swaps the
// package-level Modrinth client singleton to target it (via the client's
// exported BaseURL field), restoring the original client afterward.
func withMrClient(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL + "/")
	require.NoError(t, err)

	original := mrDefaultClient
	testClient := NewModrinthClient(&http.Client{})
	testClient.BaseURL = baseURL
	mrDefaultClient = testClient
	t.Cleanup(func() { mrDefaultClient = original })
}

func mrTestMod(name, projectID, installedVersion string) *core.Mod {
	return &core.Mod{
		Name:     name,
		FileName: "old.jar",
		Update: core.ModUpdate{
			"modrinth": core.ModSourceData{
				"mod-id":  projectID,
				"version": installedVersion,
			},
		},
	}
}

func TestMrUpdater_CheckUpdate(t *testing.T) {
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	t.Run("update available for a new version", func(t *testing.T) {
		withMrClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"v2","project_id":"abc","version_number":"2.0","game_versions":["1.20.1"],"date_published":"2024-01-01T00:00:00Z","files":[{"filename":"new.jar","primary":true,"url":"https://example.com/new.jar","hashes":{"sha1":"abc123"}}]}]`))
		}))

		mods := []*core.Mod{mrTestMod("Test Mod", "abc", "v1")}
		results, err := mrUpdater{}.CheckUpdate(mods, pack)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].UpdateAvailable)
		assert.Equal(t, "old.jar -> new.jar", results[0].UpdateString)
	})

	t.Run("no update when version matches installed", func(t *testing.T) {
		withMrClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"v1","project_id":"abc","version_number":"1.0","game_versions":["1.20.1"],"date_published":"2024-01-01T00:00:00Z","files":[{"filename":"old.jar","primary":true,"url":"https://example.com/old.jar","hashes":{"sha1":"abc123"}}]}]`))
		}))

		mods := []*core.Mod{mrTestMod("Test Mod", "abc", "v1")}
		results, err := mrUpdater{}.CheckUpdate(mods, pack)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].UpdateAvailable)
	})

	t.Run("decode failure is reported per-mod", func(t *testing.T) {
		badMod := &core.Mod{Name: "Bad Mod", Update: core.ModUpdate{"modrinth": nil}}
		results, err := mrUpdater{}.CheckUpdate([]*core.Mod{badMod}, pack)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Error(t, results[0].Error)
	})
}

func TestMrUpdater_DoUpdate(t *testing.T) {
	mod := mrTestMod("Test Mod", "abc", "v1")
	filename := "new.jar"
	fileURL := "https://example.com/new.jar"
	primary := true
	versionID := "v2"
	version := &modrinthApi.Version{
		ID: &versionID,
		Files: []*modrinthApi.File{{
			Filename: &filename,
			URL:      &fileURL,
			Primary:  &primary,
			Hashes:   map[string]string{"sha512": "deadbeef"},
		}},
	}
	cachedState := []interface{}{
		mrCachedStateStore{ProjectID: "abc", Version: version},
	}

	err := mrUpdater{}.DoUpdate([]*core.Mod{mod}, cachedState)
	require.NoError(t, err)
	assert.Equal(t, "new.jar", mod.FileName)
	assert.Equal(t, "deadbeef", mod.Download.Hash)
	assert.Equal(t, "sha512", mod.Download.HashFormat)
	assert.Equal(t, &versionID, mod.Update["modrinth"]["version"])
}
