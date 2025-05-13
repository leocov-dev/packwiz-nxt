package cmdcurseforge

import (
	"fmt"
	"strconv"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
)

// openCmd represents the open command
var openCmd = &cobra.Command{
	Use:     "open [name]",
	Short:   "Open the project page for a CurseForge file in your browser",
	Aliases: []string{"doc"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		packFile, _, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		resolvedMod, ok := pack.Mods[args[0]]
		if !ok {
			// TODO: should this auto-refresh?
			shared.Exitln("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
		}

		var cfUpdateData sources.CfUpdateData
		err = resolvedMod.DecodeNamedModSourceData("curseforge", &cfUpdateData)

		fmt.Println("Opening browser...")
		url := "https://www.curseforge.com/projects/" + strconv.FormatUint(uint64(cfUpdateData.ProjectID), 10)
		err = open.Start(url)
		if err != nil {
			fmt.Println("Opening page failed, direct link:")
			fmt.Println(url)
		}
	},
}

func init() {
	curseforgeCmd.AddCommand(openCmd)
}
