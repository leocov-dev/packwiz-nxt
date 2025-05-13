package cmdcurseforge

import (
	"github.com/spf13/cobra"

	"github.com/leocov-dev/packwiz-nxt/cmd"
)

var curseforgeCmd = &cobra.Command{
	Use:     "curseforge",
	Aliases: []string{"cf", "curse"},
	Short:   "Manage curseforge-based mods",
}

func init() {
	cmd.Add(curseforgeCmd)
}
