package main

import (
	"github.com/leocov-dev/packwiz-nxt/cmd"
	"github.com/leocov-dev/packwiz-nxt/config"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/curseforge"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/github"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/migrate"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/modrinth"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/settings"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/url"
	_ "github.com/leocov-dev/packwiz-nxt/internal/commands/utils"
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
