package sources

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/config"
	"net/http"
	"strconv"

	"github.com/leocov-dev/packwiz-nxt/core"
)

const ghApiServer = "api.github.com"

type ghApiClient struct {
	httpClient *http.Client
	logger     core.Logger
}

var ghDefaultClient = ghApiClient{&http.Client{Timeout: core.DefaultHTTPTimeout}, core.PrintLogger{}}

// GetGithubClient returns the default GitHub API client, mirroring
// GetCurseforgeClient/GetModrinthClient.
func GetGithubClient() *ghApiClient {
	return &ghDefaultClient
}

// SetLogger overrides the client's logger, used to report non-fatal warnings/progress.
func (c *ghApiClient) SetLogger(l core.Logger) {
	c.logger = l
}

func (c *ghApiClient) makeGet(url string) (*http.Response, error) {
	ghApiToken := config.GetGhApiKey()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", core.UserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	if ghApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+ghApiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO: there is likely a better way to do this
	ratelimit := 999

	ratelimit_header := resp.Header.Get("x-ratelimit-remaining")
	if ratelimit_header != "" {
		ratelimit, err = strconv.Atoi(ratelimit_header)
		if err != nil {
			return nil, err
		}
	}

	if resp.StatusCode == 403 && ratelimit == 0 {
		return nil, fmt.Errorf("GitHub API ratelimit exceeded; time of reset: %v", resp.Header.Get("x-ratelimit-reset"))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response status: %v", resp.Status)
	}

	if ratelimit < 10 {
		c.logger.Warnf("Warning: GitHub API allows %v more requests before ratelimiting\n", ratelimit)
		c.logger.Warnf("Specifying a token is recommended; see documentation\n")
	}

	return resp, nil
}

func (c *ghApiClient) getRepo(slug string) (*http.Response, error) {
	resp, err := c.makeGet("https://" + ghApiServer + "/repos/" + slug)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

func (c *ghApiClient) getReleases(slug string) (*http.Response, error) {
	resp, err := c.getRepo(slug + "/releases")
	if err != nil {
		return resp, err
	}

	return resp, nil
}
