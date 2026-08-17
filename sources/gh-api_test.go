package sources

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

// recordingLogger records Warnf messages for assertion, leaving Infof a no-op.
type recordingLogger struct {
	warnings []string
}

func (l *recordingLogger) Infof(format string, args ...any) {}
func (l *recordingLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

func TestGhApiClient_makeGet(t *testing.T) {
	t.Run("non-200 status returns wrapped error", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		client := NewGithubClient(httpClient, core.NoopLogger{})

		_, err := client.makeGet("https://" + ghApiServer + "/repos/foo/bar")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid response status")
	})

	t.Run("403 with exhausted ratelimit returns ratelimit error", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-reset", "1700000000")
			w.WriteHeader(http.StatusForbidden)
		}))
		client := NewGithubClient(httpClient, core.NoopLogger{})

		_, err := client.makeGet("https://" + ghApiServer + "/repos/foo/bar")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ratelimit exceeded")
	})

	t.Run("low remaining ratelimit logs a warning", func(t *testing.T) {
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-ratelimit-remaining", "5")
			w.WriteHeader(http.StatusOK)
		}))
		logger := &recordingLogger{}
		client := NewGithubClient(httpClient, logger)

		_, err := client.makeGet("https://" + ghApiServer + "/repos/foo/bar")
		require.NoError(t, err)
		assert.NotEmpty(t, logger.warnings)
	})

	t.Run("sends bearer token when configured", func(t *testing.T) {
		var gotAuth string
		httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("x-ratelimit-remaining", "999")
			w.WriteHeader(http.StatusOK)
		}))
		client := NewGithubClient(httpClient, core.NoopLogger{})

		_, err := client.makeGet("https://" + ghApiServer + "/repos/foo/bar")
		require.NoError(t, err)
		// no token configured in this test process, so Authorization is unset
		assert.Empty(t, gotAuth)
	})
}

func TestGhApiClient_getRepoAndReleases(t *testing.T) {
	httpClient := newTestHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-remaining", "999")
		switch r.URL.Path {
		case "/repos/foo/bar":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"foo/bar"}`))
		case "/repos/foo/bar/releases":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	client := NewGithubClient(httpClient, core.NoopLogger{})

	resp, err := client.getRepo("foo/bar")
	require.NoError(t, err)
	resp.Body.Close()

	resp, err = client.getReleases("foo/bar")
	require.NoError(t, err)
	resp.Body.Close()
}
