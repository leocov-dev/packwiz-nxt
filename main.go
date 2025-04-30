package main

import (
	"github.com/leocov-dev/fork.packwiz/cmd"
	"github.com/leocov-dev/fork.packwiz/config"
	_ "github.com/leocov-dev/fork.packwiz/internal/commands/curseforge"
	_ "github.com/leocov-dev/fork.packwiz/internal/commands/github"
	_ "github.com/leocov-dev/fork.packwiz/internal/commands/migrate"
	_ "github.com/leocov-dev/fork.packwiz/internal/commands/modrinth"
	_ "github.com/leocov-dev/fork.packwiz/internal/commands/settings"
	_ "github.com/leocov-dev/fork.packwiz/internal/commands/url"
	_ "github.com/leocov-dev/fork.packwiz/internal/commands/utils"
)

var Version string
var CfApiKey string

func main() {
	config.SetConfig(
		Version,
		CfApiKey,
	)
	cmd.Execute()
}
