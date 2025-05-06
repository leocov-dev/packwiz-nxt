package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dlclark/regexp2"
	"github.com/leocov-dev/packwiz-nxt/core"
	"io"
	"regexp"
)

func init() {
	core.AddUpdater("github", ghUpdater{})
}

var GithubRegex = regexp.MustCompile(`^https?://(?:www\.)?github\.com/([^/]+/[^/]+)`)

func AddGitHubMod(slugOrUrl, branch, regex string) (core.ModToml, Repo, Asset, error) {
	var slug string

	// Check if the argument is a valid GitHub repository URL; if so, extract the slug from the URL.
	// Otherwise, interpret the argument as a slug directly.
	matches := GithubRegex.FindStringSubmatch(slugOrUrl)
	if len(matches) == 2 {
		slug = matches[1]
	} else {
		slug = slugOrUrl
	}

	repo, err := fetchRepo(slug)

	if err != nil {
		return core.ModToml{}, repo, Asset{}, err
	}

	if regex == "" {
		// Regex to match potential release assets against.
		// The default will match any asset with a name that does *not* end with:
		// - "-api.jar"
		// - "-dev.jar"
		// - "-dev-preshadow.jar"
		// - "-sources.jar"
		// In most cases, this will only match one asset.
		// TODO: Hopefully.
		regex = `^.+(?<!-api|-dev|-dev-preshadow|-sources)\.jar$`
	}

	modMeta, file, err := installMod(repo, branch, regex)
	if err != nil {
		return core.ModToml{}, repo, file, err
	}

	return modMeta, repo, file, nil
}

func installMod(repo Repo, branch string, regex string) (core.ModToml, Asset, error) {
	latestRelease, err := getLatestRelease(repo.FullName, branch)
	if err != nil {
		return core.ModToml{}, Asset{}, fmt.Errorf("failed to get latest release: %v", err)
	}

	return installRelease(repo, latestRelease, regex)
}

func getLatestRelease(slug string, branch string) (Release, error) {
	var releases []Release
	var release Release

	resp, err := ghDefaultClient.getReleases(slug)
	if err != nil {
		return release, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return release, err
	}

	err = json.Unmarshal(body, &releases)
	if err != nil {
		return release, err
	}

	if branch != "" {
		for _, r := range releases {
			if r.TargetCommitish == branch {
				return r, nil
			}
		}
		return release, fmt.Errorf("failed to find release for branch %v", branch)
	}

	return releases[0], nil
}

func installRelease(
	repo Repo,
	release Release,
	regex string,
) (core.ModToml, Asset, error) {
	expr := regexp2.MustCompile(regex, 0)

	var file Asset

	if len(release.Assets) == 0 {
		return core.ModToml{}, file, errors.New("release doesn't have any assets attached")
	}

	var files []Asset

	for _, v := range release.Assets {
		bl, _ := expr.MatchString(v.Name)
		if bl {
			files = append(files, v)
		}
	}

	if len(files) == 0 {
		return core.ModToml{}, file, errors.New("release doesn't have any assets matching regex")
	}

	if len(files) > 1 {
		// TODO: also print file names
		return core.ModToml{}, file, errors.New("release has more than one asset matching regex")
	}

	file = files[0]

	// Install the file
	fmt.Printf("Installing %s from release %s\n", file.Name, release.TagName)

	updateMap := make(map[string]map[string]interface{})

	var err error

	updateMap["github"], err = ghUpdateData{
		Slug:   repo.FullName,
		Tag:    release.TagName,
		Branch: release.TargetCommitish, // TODO: if no branch is specified by the user, we shouldn't record it - in order to remain branch-agnostic in getLatestRelease()
		Regex:  regex,                   // TODO: ditto!
	}.ToMap()
	if err != nil {
		return core.ModToml{}, file, err
	}

	hash, err := file.getSha256()
	if err != nil {
		return core.ModToml{}, file, err
	}

	modMeta := core.ModToml{
		Name:     repo.Name,
		FileName: file.Name,
		Side:     core.UniversalSide,
		Download: core.ModDownload{
			URL:        file.BrowserDownloadURL,
			HashFormat: "sha256",
			Hash:       hash,
		},
		Update: updateMap,
	}

	return modMeta, file, nil
}
