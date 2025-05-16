package packwiz

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/sources"
)

type Pack core.Pack
type Mod core.Mod

var (
	NewPack         = core.NewPack
	NewMod          = core.NewMod
	UpdateSingleMod = core.UpdateSingleMod
	UpdateAllMods   = core.UpdateAllMods

	GithubNewMod = sources.GitHubNewMod

	ModrinthNewMod                  = sources.ModrinthNewMod
	ModrinthFindMissingDependencies = sources.ModrinthFindMissingDependencies
	ModrinthParseUrl                = sources.ParseAsModrinthSlug
	ModrinthSearchForProjects       = sources.ModrinthSearchForProjects
	ModrinthProjectFromVersionID    = sources.ModrinthProjectFromVersionID
	ModrinthGetLatestVersion        = sources.ModrinthGetLatestVersion

	CurseforgeNewMod                  = sources.CurseforgeNewMod
	CurseforgeFindMissingDependencies = sources.CurseforgeFindMissingDependencies
	CurseforgeParseUrl                = sources.CurseforgeParseUrl
	CurseforgeModInfoFromSlug         = sources.CurseforgeModInfoFromSlug
	CurseforgeModInfo                 = sources.CurseforgeModInfoFromID
)
