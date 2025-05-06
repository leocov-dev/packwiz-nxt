package cmdgithub

import (
	"github.com/leocov-dev/packwiz-nxt/cmd"
	"github.com/spf13/cobra"
)

var githubCmd = &cobra.Command{
	Use:     "github",
	Aliases: []string{"gh"},
	Short:   "Manage projects released on GitHub",
}

func init() {
	cmd.Add(githubCmd)
}
