package core

import (
	"context"
	"net/http"
	"time"
)

const UserAgent = "packwiz/packwiz"

// DefaultHTTPTimeout is applied to the default HTTP clients used by core and the
// sources providers, so a hung remote server doesn't block forever.
const DefaultHTTPTimeout = 30 * time.Second

var defaultRequestClient = &http.Client{Timeout: DefaultHTTPTimeout}

// GetWithUA performs a GET request with the packwiz User-Agent and given Accept header.
// It is not cancellable; use GetWithUAContext where a context.Context is available.
func GetWithUA(url string, contentType string) (resp *http.Response, err error) {
	return GetWithUAContext(context.Background(), url, contentType)
}

// GetWithUAContext is GetWithUA with an explicit, cancellable context.Context.
func GetWithUAContext(ctx context.Context, url string, contentType string) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", contentType)
	return defaultRequestClient.Do(req)
}
