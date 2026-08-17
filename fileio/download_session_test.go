package fileio

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/core/mocks"
)

func withTestCache(t *testing.T) {
	t.Helper()
	resetViper(t)
	viper.Set("cache.directory", t.TempDir())
}

func drainDownloads(ch chan CompletedDownload) []CompletedDownload {
	var all []CompletedDownload
	for dl := range ch {
		all = append(all, dl)
	}
	return all
}

func TestCreateDownloadSession_URLMode(t *testing.T) {
	withTestCache(t)

	const content = "jar file contents"
	hash := sha256Hex(content)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	mod := &core.Mod{
		Name: "URL Mod",
		Download: core.ModDownload{
			URL:        server.URL,
			HashFormat: "sha256",
			Hash:       hash,
		},
	}

	session, err := CreateDownloadSession(nil, []*core.Mod{mod}, []string{"sha256"})
	require.NoError(t, err)

	downloads := drainDownloads(session.StartDownloads(context.Background()))
	require.Len(t, downloads, 1)
	require.NoError(t, downloads[0].Error)
	assert.Equal(t, mod, downloads[0].Mod)
	assert.Equal(t, hash, downloads[0].Hashes["sha256"])
	require.NoError(t, downloads[0].File.Close())
}

func TestCreateDownloadSession_URLMode_HashMismatch(t *testing.T) {
	withTestCache(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("actual content"))
	}))
	t.Cleanup(server.Close)

	mod := &core.Mod{
		Name: "Bad Hash Mod",
		Download: core.ModDownload{
			URL:        server.URL,
			HashFormat: "sha256",
			Hash:       sha256Hex("expected different content"),
		},
	}

	session, err := CreateDownloadSession(nil, []*core.Mod{mod}, []string{"sha256"})
	require.NoError(t, err)

	downloads := drainDownloads(session.StartDownloads(context.Background()))
	require.Len(t, downloads, 1)
	assert.Error(t, downloads[0].Error)
}

func TestCreateDownloadSession_URLMode_NonOKStatus(t *testing.T) {
	withTestCache(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	mod := &core.Mod{
		Name:     "Missing Mod",
		Download: core.ModDownload{URL: server.URL, HashFormat: "sha256", Hash: "irrelevant"},
	}

	session, err := CreateDownloadSession(nil, []*core.Mod{mod}, []string{"sha256"})
	require.NoError(t, err)

	downloads := drainDownloads(session.StartDownloads(context.Background()))
	require.Len(t, downloads, 1)
	assert.Error(t, downloads[0].Error)
}

func TestCreateDownloadSession_MetadataMode_DirectDownload(t *testing.T) {
	withTestCache(t)

	const content = "metadata-fetched content"
	hash := sha256Hex(content)

	mod := &core.Mod{
		Name:     "Metadata Mod",
		Download: core.ModDownload{Mode: "metadata:test-source", HashFormat: "sha256", Hash: hash},
	}

	mockData := mocks.NewMockMetaDownloaderData(t)
	mockData.EXPECT().GetManualDownload().Return(false, core.ManualDownload{})
	mockData.EXPECT().DownloadFile().Return(io.NopCloser(strings.NewReader(content)), nil)

	mockDownloader := mocks.NewMockMetaDownloader(t)
	mockDownloader.EXPECT().GetFilesMetadata([]*core.Mod{mod}).
		Return([]core.MetaDownloaderData{mockData}, nil)

	reg := core.NewRegistry()
	reg.AddMetaDownloader("test-source", mockDownloader)

	session, err := CreateDownloadSession(reg, []*core.Mod{mod}, []string{"sha256"})
	require.NoError(t, err)

	downloads := drainDownloads(session.StartDownloads(context.Background()))
	require.Len(t, downloads, 1)
	require.NoError(t, downloads[0].Error)
	assert.Equal(t, hash, downloads[0].Hashes["sha256"])
}

func TestCreateDownloadSession_MetadataMode_ManualDownloadRequired(t *testing.T) {
	withTestCache(t)

	mod := &core.Mod{
		Name:     "Manual Mod",
		Download: core.ModDownload{Mode: "metadata:test-source", HashFormat: "sha256", Hash: "unknownhash"},
	}

	manual := core.ManualDownload{Name: "Manual Mod", FileName: "manual.jar", URL: "https://example.com/manual"}

	mockData := mocks.NewMockMetaDownloaderData(t)
	mockData.EXPECT().GetManualDownload().Return(true, manual)

	mockDownloader := mocks.NewMockMetaDownloader(t)
	mockDownloader.EXPECT().GetFilesMetadata([]*core.Mod{mod}).
		Return([]core.MetaDownloaderData{mockData}, nil)

	reg := core.NewRegistry()
	reg.AddMetaDownloader("test-source", mockDownloader)

	session, err := CreateDownloadSession(reg, []*core.Mod{mod}, []string{"sha256"})
	require.NoError(t, err)

	assert.Equal(t, []core.ManualDownload{manual}, session.GetManualDownloads())

	downloads := drainDownloads(session.StartDownloads(context.Background()))
	assert.Empty(t, downloads)
}

func TestCreateDownloadSession_UnknownDownloadMode(t *testing.T) {
	withTestCache(t)

	mod := &core.Mod{Name: "Bad Mode Mod", Download: core.ModDownload{Mode: "not-a-real-mode"}}

	_, err := CreateDownloadSession(nil, []*core.Mod{mod}, []string{"sha256"})
	assert.Error(t, err)
}
