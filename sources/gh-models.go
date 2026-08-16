package sources

import (
	"io"

	"github.com/leocov-dev/packwiz-nxt/core"
)

type Repo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`      // "hello_world"
	FullName string `json:"full_name"` // "owner/hello_world"
}

type Release struct {
	URL             string  `json:"url"`
	TagName         string  `json:"tag_name"`
	TargetCommitish string  `json:"target_commitish"` // The branch of the release
	Name            string  `json:"name"`
	CreatedAt       string  `json:"created_at"`
	Assets          []Asset `json:"assets"`
}

type Asset struct {
	URL                string `json:"url"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Name               string `json:"name"`
}

func (u Asset) getSha256() (string, error) {
	// TODO potentionally cache downloads to speed things up and avoid getting ratelimited by github!
	mainHasher, err := core.GetHashImpl("sha256")
	if err != nil {
		return "", err
	}

	resp, err := ghDefaultClient.makeGet(u.BrowserDownloadURL)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	mainHasher.Write(body)

	return mainHasher.String(), nil
}
