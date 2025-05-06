package cmdmodrinth

import (
	"github.com/leocov-dev/packwiz-nxt/cmd"
	"github.com/spf13/cobra"
)

var modrinthCmd = &cobra.Command{
	Use:     "modrinth",
	Aliases: []string{"mr"},
	Short:   "Manage modrinth-based mods",
}

func init() {
	cmd.Add(modrinthCmd)
}
