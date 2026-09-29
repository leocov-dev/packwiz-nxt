package sources

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

// withGhClient swaps the package-level GitHub client singleton for the
// duration of the test, restoring the original afterward.
func withGhClient(t *testing.T, httpClient *http.Client) {
	t.Helper()
	original := ghDefaultClient
	ghDefaultClient = *NewGithubClient(httpClient, core.NoopLogger{})
	t.Cleanup(func() { ghDefaultClient = original })
}

func ghTestMod(name, slug, tag string) *core.Mod {
	return &core.Mod{
		Name:     name,
		FileName: "old.jar",
		Update: core.ModUpdate{
			"github": core.ModSourceData{
				"slug":   slug,
				"tag":    tag,
				"branch": "",
				"regex":  `^.+\.jar$`,
			},
		},
	}
}

func TestGhUpdater_CheckUpdate(t *testing.T) {
	t.Run("update available for a new release tag", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"tag_name":"v2.0","assets":[{"name":"mod-v2.0.jar","browser_download_url":"https://example.com/mod-v2.0.jar"}]}]`))
		}))
		withGhClient(t, httpClient)

		mods := []*core.Mod{ghTestMod("Test Mod", "foo/bar", "v1.0")}
		results, err := ghUpdater{}.CheckUpdate(mods, core.Pack{})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].UpdateAvailable)
		assert.Equal(t, "old.jar -> mod-v2.0.jar", results[0].UpdateString)
		assert.Equal(t, "v2.0", results[0].LatestVersion)
	})

	t.Run("no update when tag matches installed", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"tag_name":"v1.0","assets":[{"name":"mod-v1.0.jar"}]}]`))
		}))
		withGhClient(t, httpClient)

		mods := []*core.Mod{ghTestMod("Test Mod", "foo/bar", "v1.0")}
		results, err := ghUpdater{}.CheckUpdate(mods, core.Pack{})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].UpdateAvailable)
		assert.Empty(t, results[0].LatestVersion)
	})

	t.Run("decode failure is reported per-mod", func(t *testing.T) {
		badMod := &core.Mod{Name: "Bad Mod", Update: core.ModUpdate{"github": nil}}
		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{badMod}, core.Pack{})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Error(t, results[0].Error)
	})
}

func TestGhUpdater_DoUpdate(t *testing.T) {
	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-remaining", "999")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`jar-file-contents`))
	}))
	withGhClient(t, httpClient)

	mod := ghTestMod("Test Mod", "foo/bar", "v1.0")
	cachedState := []interface{}{
		ghCachedStateStore{Slug: "foo/bar", Tag: "v2.0", Asset: Asset{
			Name:               "mod-v2.0.jar",
			BrowserDownloadURL: "https://example.com/mod-v2.0.jar",
		}},
	}

	err := ghUpdater{}.DoUpdate([]*core.Mod{mod}, cachedState)
	require.NoError(t, err)
	assert.Equal(t, "mod-v2.0.jar", mod.FileName)
	assert.Equal(t, "v2.0", mod.Update["github"]["tag"])
	assert.NotEmpty(t, mod.Download.Hash)
	assert.Equal(t, "sha256", mod.Download.HashFormat)
	assert.Equal(t, "v2.0", mod.Version)
}

func TestGhUpdater_DoUpdate_FailureLeavesVersion(t *testing.T) {
	// A transport error while hashing the asset must abort before any field,
	// including Version, is touched.
	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		require.True(t, ok)
		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		_ = conn.Close()
	}))
	withGhClient(t, httpClient)

	mod := ghTestMod("Test Mod", "foo/bar", "v1.0")
	mod.Version = "v1.0"
	cachedState := []interface{}{
		ghCachedStateStore{Slug: "foo/bar", Tag: "v2.0", Asset: Asset{
			Name:               "mod-v2.0.jar",
			BrowserDownloadURL: "https://example.com/mod-v2.0.jar",
		}},
	}

	err := ghUpdater{}.DoUpdate([]*core.Mod{mod}, cachedState)
	require.Error(t, err)
	assert.Equal(t, "v1.0", mod.Version)
}

func TestGhUpdater_DoUpdate_EmptyTagKeepsVersion(t *testing.T) {
	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-remaining", "999")
		_, _ = w.Write([]byte(`x`))
	}))
	withGhClient(t, httpClient)

	mod := ghTestMod("Test Mod", "foo/bar", "v1.0")
	mod.Version = "v1.0"
	cachedState := []interface{}{
		ghCachedStateStore{Slug: "foo/bar", Tag: "", Asset: Asset{Name: "x.jar", BrowserDownloadURL: "https://example.com/x.jar"}},
	}
	require.NoError(t, ghUpdater{}.DoUpdate([]*core.Mod{mod}, cachedState))
	assert.Equal(t, "v1.0", mod.Version)
}

func TestAsset_getSha256(t *testing.T) {
	// BrowserDownloadURL is caller-supplied (unlike the API host), so it can
	// point directly at an httptest.Server without the redirect-transport helper.
	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-remaining", "999")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`known content`))
	}))
	withGhClient(t, httpClient)

	asset := Asset{BrowserDownloadURL: "https://example.com/file.jar"}
	hash, err := asset.getSha256()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}
