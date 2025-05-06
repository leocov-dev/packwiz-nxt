package packwiz

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/sources"
)

func AddGitHubMod(slugOrUrl, branch, regex string) (core.ModToml, sources.Repo, sources.Asset, error) {
	return sources.AddGitHubMod(slugOrUrl, branch, regex)
}
