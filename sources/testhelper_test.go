package sources

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// redirectTransport rewrites the scheme+host of every outgoing request to
// target before delegating to the underlying RoundTripper, so package code
// with a hardcoded API host (cfApiServer/ghApiServer, or the vendored
// Modrinth client's internal base URL) can be redirected to an
// httptest.Server without touching production URL-building code.
type redirectTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	req.Host = t.target.Host
	return t.base.RoundTrip(req)
}

// newTestHTTPClient starts an httptest.Server backed by handler and returns
// an *http.Client whose requests are transparently redirected to it,
// regardless of the scheme/host the calling code targeted. The server is
// closed automatically via t.Cleanup.
func newTestHTTPClient(t *testing.T, handler http.Handler) *http.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse httptest server URL: %v", err)
	}

	return &http.Client{
		Transport: redirectTransport{target: target, base: http.DefaultTransport},
	}
}
