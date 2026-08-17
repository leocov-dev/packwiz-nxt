package sources

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultGithubAssetRegex = `^.+(?<!-api|-dev|-dev-preshadow|-sources)\.jar$`

func TestSelectReleaseAsset(t *testing.T) {
	t.Run("no assets errors", func(t *testing.T) {
		_, err := selectReleaseAsset(nil, defaultGithubAssetRegex)
		assert.Error(t, err)
	})

	t.Run("single asset matching regex is returned", func(t *testing.T) {
		asset, err := selectReleaseAsset([]Asset{{Name: "mymod-1.0.jar"}}, defaultGithubAssetRegex)
		require.NoError(t, err)
		assert.Equal(t, "mymod-1.0.jar", asset.Name)
	})

	t.Run("no asset matches regex errors", func(t *testing.T) {
		_, err := selectReleaseAsset([]Asset{{Name: "README.md"}}, defaultGithubAssetRegex)
		assert.Error(t, err)
	})

	t.Run("multiple assets matching regex errors", func(t *testing.T) {
		_, err := selectReleaseAsset([]Asset{{Name: "mymod-1.0.jar"}, {Name: "mymod-1.0-extra.jar"}}, defaultGithubAssetRegex)
		assert.Error(t, err)
	})

	t.Run("default regex excludes api/dev/dev-preshadow/sources suffixed jars", func(t *testing.T) {
		assets := []Asset{
			{Name: "mymod-1.0.jar"},
			{Name: "mymod-1.0-sources.jar"},
			{Name: "mymod-1.0-dev.jar"},
			{Name: "mymod-1.0-dev-preshadow.jar"},
			{Name: "mymod-1.0-api.jar"},
		}
		asset, err := selectReleaseAsset(assets, defaultGithubAssetRegex)
		require.NoError(t, err)
		assert.Equal(t, "mymod-1.0.jar", asset.Name)
	})
}

func TestGithubRegex(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantMatch bool
		wantSlug  string
	}{
		{"https URL", "https://github.com/owner/repo", true, "owner/repo"},
		{"http URL with www", "http://www.github.com/owner/repo", true, "owner/repo"},
		{"URL with extra path segments", "https://github.com/owner/repo/releases", true, "owner/repo"},
		{"bare slug does not match", "owner/repo", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := GithubRegex.FindStringSubmatch(tt.in)
			if !tt.wantMatch {
				assert.Nil(t, matches)
				return
			}
			require.Len(t, matches, 2)
			assert.Equal(t, tt.wantSlug, matches[1])
		})
	}
}

func TestFetchRepo(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"name":"repo","full_name":"owner/repo"}`))
		}))
		withGhClient(t, httpClient)

		repo, err := fetchRepo("owner/repo")
		require.NoError(t, err)
		assert.Equal(t, "owner/repo", repo.FullName)
	})

	t.Run("empty full_name is an error", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		withGhClient(t, httpClient)

		_, err := fetchRepo("owner/repo")
		assert.Error(t, err)
	})
}

func TestGetLatestRelease(t *testing.T) {
	t.Run("no branch filter returns first release", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"tag_name":"v2.0","target_commitish":"main"},{"tag_name":"v1.0","target_commitish":"main"}]`))
		}))
		withGhClient(t, httpClient)

		release, err := getLatestRelease("owner/repo", "")
		require.NoError(t, err)
		assert.Equal(t, "v2.0", release.TagName)
	})

	t.Run("branch filter finds matching release", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"tag_name":"v2.0","target_commitish":"dev"},{"tag_name":"v1.0","target_commitish":"main"}]`))
		}))
		withGhClient(t, httpClient)

		release, err := getLatestRelease("owner/repo", "main")
		require.NoError(t, err)
		assert.Equal(t, "v1.0", release.TagName)
	})

	t.Run("branch filter with no match errors", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"tag_name":"v1.0","target_commitish":"main"}]`))
		}))
		withGhClient(t, httpClient)

		_, err := getLatestRelease("owner/repo", "missing-branch")
		assert.Error(t, err)
	})

	t.Run("no releases errors", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}))
		withGhClient(t, httpClient)

		_, err := getLatestRelease("owner/repo", "")
		assert.Error(t, err)
	})
}

func TestInstallMod(t *testing.T) {
	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-remaining", "999")
		if r.URL.Path == "/file.jar" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`jar contents`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"tag_name":"v1.0","assets":[{"name":"mod.jar","browser_download_url":"https://example.com/file.jar"}]}]`))
	}))
	withGhClient(t, httpClient)

	repo := Repo{Name: "repo", FullName: "owner/repo"}
	mod, err := installMod(repo, "", defaultGithubAssetRegex, "mods")
	require.NoError(t, err)
	assert.Equal(t, "mod.jar", mod.FileName)
	assert.NotEmpty(t, mod.Download.Hash)
}
