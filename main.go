package main

import (
	"github.com/leocov-dev/packwiz-nxt/cmd"
	"github.com/leocov-dev/packwiz-nxt/config"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/cmdcurseforge"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/cmdgithub"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/cmdmigrate"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/cmdmodrinth"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/cmdsettings"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/cmdurl"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/cmdutils"
)

var Version string
var CfApiKey string

func main() {
	config.SetVersion(Version)
	config.SetCurseforgeApiKey(CfApiKey)
	cmd.Execute()
}
