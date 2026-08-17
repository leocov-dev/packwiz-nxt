package sources

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/config"
	"github.com/leocov-dev/packwiz-nxt/core"
)

// withTestCfApiKey sets a decodable CurseForge API key for the duration of
// the test, restoring the previous (unset) value afterward - makeGet/makePost
// fail fast without one.
func withTestCfApiKey(t *testing.T) {
	t.Helper()
	config.SetCurseforgeApiKey(base64.StdEncoding.EncodeToString([]byte("test-key")))
	t.Cleanup(func() { config.SetCurseforgeApiKey("") })
}

type cfFileInfoHash = struct {
	Value     string   `json:"value"`
	Algorithm hashAlgo `json:"algo"`
}

func TestCfModFileInfo_GetBestHash(t *testing.T) {
	t.Run("no hashes falls back to fingerprint as murmur2", func(t *testing.T) {
		info := CfModFileInfo{Fingerprint: 12345}
		hash, format := info.GetBestHash()
		assert.Equal(t, "12345", hash)
		assert.Equal(t, "murmur2", format)
	})

	t.Run("only md5 present", func(t *testing.T) {
		info := CfModFileInfo{
			Fingerprint: 12345,
			Hashes:      []cfFileInfoHash{{Algorithm: hashAlgoMD5, Value: "md5hash"}},
		}
		hash, format := info.GetBestHash()
		assert.Equal(t, "md5hash", hash)
		assert.Equal(t, "md5", format)
	})

	t.Run("only sha1 present", func(t *testing.T) {
		info := CfModFileInfo{
			Fingerprint: 12345,
			Hashes:      []cfFileInfoHash{{Algorithm: hashAlgoSHA1, Value: "sha1hash"}},
		}
		hash, format := info.GetBestHash()
		assert.Equal(t, "sha1hash", hash)
		assert.Equal(t, "sha1", format)
	})

	t.Run("both present, md5 then sha1: sha1 wins", func(t *testing.T) {
		info := CfModFileInfo{
			Hashes: []cfFileInfoHash{
				{Algorithm: hashAlgoMD5, Value: "md5hash"},
				{Algorithm: hashAlgoSHA1, Value: "sha1hash"},
			},
		}
		hash, format := info.GetBestHash()
		assert.Equal(t, "sha1hash", hash)
		assert.Equal(t, "sha1", format)
	})

	t.Run("both present, sha1 then md5: sha1 still wins", func(t *testing.T) {
		info := CfModFileInfo{
			Hashes: []cfFileInfoHash{
				{Algorithm: hashAlgoSHA1, Value: "sha1hash"},
				{Algorithm: hashAlgoMD5, Value: "md5hash"},
			},
		}
		hash, format := info.GetBestHash()
		assert.Equal(t, "sha1hash", hash)
		assert.Equal(t, "sha1", format)
	})

	t.Run("unrecognized algorithm falls back to fingerprint", func(t *testing.T) {
		info := CfModFileInfo{
			Fingerprint: 999,
			Hashes:      []cfFileInfoHash{{Algorithm: hashAlgo(99), Value: "unknown"}},
		}
		hash, format := info.GetBestHash()
		assert.Equal(t, "999", hash)
		assert.Equal(t, "murmur2", format)
	})
}

func TestCfApiClient_makeGet(t *testing.T) {
	withTestCfApiKey(t)

	t.Run("non-200 status returns wrapped error", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		client := NewCfApiClient(httpClient, core.PrintLogger{})

		_, err := client.makeGet("/v1/mods/1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid response status")
	})

	t.Run("sends api key and accept headers", func(t *testing.T) {
		var gotApiKey, gotAccept string
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotApiKey = r.Header.Get("X-API-Key")
			gotAccept = r.Header.Get("Accept")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{}}`))
		}))
		client := NewCfApiClient(httpClient, core.PrintLogger{})

		_, err := client.makeGet("/v1/mods/1")
		require.NoError(t, err)
		assert.Equal(t, "test-key", gotApiKey)
		assert.Equal(t, "application/json", gotAccept)
	})
}

func TestCfApiClient_GetModInfo(t *testing.T) {
	withTestCfApiKey(t)

	t.Run("happy path decodes mod info", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/mods/42", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"id":42,"name":"Test Mod","slug":"test-mod"}}`))
		}))
		client := NewCfApiClient(httpClient, core.PrintLogger{})

		info, err := client.GetModInfo(42)
		require.NoError(t, err)
		assert.Equal(t, uint32(42), info.ID)
		assert.Equal(t, "Test Mod", info.Name)
		assert.Equal(t, "test-mod", info.Slug)
	})

	t.Run("unexpected id in response is an error", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"id":99}}`))
		}))
		client := NewCfApiClient(httpClient, core.PrintLogger{})

		_, err := client.GetModInfo(42)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected project ID")
	})
}

func TestCfApiClient_GetFileInfo(t *testing.T) {
	withTestCfApiKey(t)

	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/mods/42/files/7", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":7,"modId":42,"fileName":"test.jar"}}`))
	}))
	client := NewCfApiClient(httpClient, core.PrintLogger{})

	info, err := client.GetFileInfo(42, 7)
	require.NoError(t, err)
	assert.Equal(t, uint32(7), info.ID)
	assert.Equal(t, "test.jar", info.FileName)
}

func TestCfApiClient_GetFingerprintInfo(t *testing.T) {
	withTestCfApiKey(t)

	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/fingerprints", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"exactFingerprints":[%d]}}`, 12345)))
	}))
	client := NewCfApiClient(httpClient, core.PrintLogger{})

	resp, err := client.GetFingerprintInfo([]uint32{12345})
	require.NoError(t, err)
	assert.Equal(t, []uint32{12345}, resp.ExactFingerprints)
}
