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

var GithubRegex = regexp.MustCompile(`^https?://(?:www\.)?github\.com/([^/]+/[^/]+)`)

func fetchRepo(slug string) (Repo, error) {
	var repo Repo

	res, err := ghDefaultClient.getRepo(slug)
	if err != nil {
		return repo, err
	}

	defer res.Body.Close()

	repoBody, err := io.ReadAll(res.Body)
	if err != nil {
		return repo, err
	}

	err = json.Unmarshal(repoBody, &repo)
	if err != nil {
		return repo, err
	}

	if repo.FullName == "" {
		return repo, errors.New("invalid json while fetching project: " + slug)
	}

	return repo, nil
}

func GitHubNewMod(slugOrUrl, branch, regex, modType string) (*core.Mod, error) {
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
		return nil, err
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

	mod, err := installMod(repo, branch, regex, modType)
	if err != nil {
		return nil, err
	}

	return mod, nil
}

func installMod(repo Repo, branch, regex, modType string) (*core.Mod, error) {
	latestRelease, err := getLatestRelease(repo.FullName, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %v", err)
	}

	return installRelease(repo, latestRelease, regex, modType)
}

func getLatestRelease(slug string, branch string) (Release, error) {
	var releases []Release

	resp, err := ghDefaultClient.getReleases(slug)
	if err != nil {
		return Release{}, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Release{}, err
	}

	err = json.Unmarshal(body, &releases)
	if err != nil {
		return Release{}, err
	}

	if branch != "" {
		for _, r := range releases {
			if r.TargetCommitish == branch {
				return r, nil
			}
		}
		return Release{}, fmt.Errorf("failed to find release for branch %v", branch)
	}

	if len(releases) < 1 {
		return Release{}, fmt.Errorf("no releases for %s", slug)
	}

	return releases[0], nil
}

// selectReleaseAsset finds the single asset in assets whose name matches regex.
// It returns an error if no assets are present, if none match the regex, or if
// more than one asset matches the regex.
func selectReleaseAsset(assets []Asset, regex string) (Asset, error) {
	if len(assets) == 0 {
		return Asset{}, errors.New("release doesn't have any assets attached")
	}

	expr := regexp2.MustCompile(regex, 0)

	var files []Asset

	for _, v := range assets {
		bl, _ := expr.MatchString(v.Name)
		if bl {
			files = append(files, v)
		}
	}

	if len(files) == 0 {
		return Asset{}, errors.New("release doesn't have any assets matching regex")
	}

	if len(files) > 1 {
		// TODO: also print file names
		return Asset{}, errors.New("release has more than one asset matching regex")
	}

	return files[0], nil
}

func installRelease(
	repo Repo,
	release Release,
	regex string,
	modType string,
) (*core.Mod, error) {
	file, err := selectReleaseAsset(release.Assets, regex)
	if err != nil {
		return nil, err
	}

	// Install the file
	GetGithubClient().logger.Infof("Installing %s from release %s\n", file.Name, release.TagName)

	updateMap := make(core.ModUpdate)

	updateMap["github"], err = ghUpdateData{
		Slug:   repo.FullName,
		Tag:    release.TagName,
		Branch: release.TargetCommitish, // TODO: if no branch is specified by the user, we shouldn't record it - in order to remain branch-agnostic in getLatestRelease()
		Regex:  regex,                   // TODO: ditto!
	}.ToMap()
	if err != nil {
		return nil, err
	}

	hash, err := file.getSha256()
	if err != nil {
		return nil, err
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
		false,
		updateMap,
		download,
		nil,
	)

	return mod, nil
}
