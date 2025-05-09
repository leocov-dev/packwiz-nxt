package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/dlclark/regexp2"
	"github.com/leocov-dev/packwiz-nxt/core"
)

func init() {
	core.AddUpdater(ghUpdater{})
}

var GithubRegex = regexp.MustCompile(`^https?://(?:www\.)?github\.com/([^/]+/[^/]+)`)

func AddGitHubMod(slugOrUrl, branch, regex, modType string) (*core.Mod, Repo, Asset, error) {
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
		return nil, repo, Asset{}, err
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

	mod, file, err := installMod(repo, branch, regex, modType)
	if err != nil {
		return nil, repo, file, err
	}

	return mod, repo, file, nil
}

func installMod(repo Repo, branch, regex, modType string) (*core.Mod, Asset, error) {
	latestRelease, err := getLatestRelease(repo.FullName, branch)
	if err != nil {
		return nil, Asset{}, fmt.Errorf("failed to get latest release: %v", err)
	}

	return installRelease(repo, latestRelease, regex, modType)
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
	modType string,
) (*core.Mod, Asset, error) {
	expr := regexp2.MustCompile(regex, 0)

	var file Asset

	if len(release.Assets) == 0 {
		return nil, file, errors.New("release doesn't have any assets attached")
	}

	var files []Asset

	for _, v := range release.Assets {
		bl, _ := expr.MatchString(v.Name)
		if bl {
			files = append(files, v)
		}
	}

	if len(files) == 0 {
		return nil, file, errors.New("release doesn't have any assets matching regex")
	}

	if len(files) > 1 {
		// TODO: also print file names
		return nil, file, errors.New("release has more than one asset matching regex")
	}

	file = files[0]

	// Install the file
	fmt.Printf("Installing %s from release %s\n", file.Name, release.TagName)

	updateMap := make(core.ModUpdate)

	var err error

	updateMap["github"], err = ghUpdateData{
		Slug:   repo.FullName,
		Tag:    release.TagName,
		Branch: release.TargetCommitish, // TODO: if no branch is specified by the user, we shouldn't record it - in order to remain branch-agnostic in getLatestRelease()
		Regex:  regex,                   // TODO: ditto!
	}.ToMap()
	if err != nil {
		return nil, file, err
	}

	hash, err := file.getSha256()
	if err != nil {
		return nil, file, err
	}

	download := core.ModDownload{
		URL:        file.BrowserDownloadURL,
		HashFormat: "sha256",
		Hash:       hash,
	}

	mod := core.NewMod(
		core.SlugifyName(repo.Name),
		repo.Name,
		file.Name,
		core.UniversalSide,
		modType,
		"",
		false,
		true,
		false,
		updateMap,
		download,
		nil,
	)

	return mod, file, nil
}
