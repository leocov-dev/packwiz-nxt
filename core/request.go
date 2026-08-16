package core

import (
	"net/http"
	"time"
)

const UserAgent = "packwiz/packwiz"

// DefaultHTTPTimeout is applied to the default HTTP clients used by core and the
// sources providers, so a hung remote server doesn't block forever.
const DefaultHTTPTimeout = 30 * time.Second

func GetWithUA(url string, contentType string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", contentType)
	return http.DefaultClient.Do(req)
}
