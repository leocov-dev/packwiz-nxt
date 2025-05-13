package packwiz

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/sources"
)

type Pack core.Pack
type Mod core.Mod

var (
	NewPack          = core.NewPack
	NewMod           = core.NewMod
	UpdateSingleMod  = core.UpdateSingleMod
	UpdateAllMods    = core.UpdateAllMods
	NewGithubMod     = sources.NewGitHubMod
	NewModrinthMod   = sources.NewModrinthMod
	NewCurseforgeMod = sources.NewCurseforgeMod
)
