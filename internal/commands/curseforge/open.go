package curseforge

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strconv"
)

// openCmd represents the open command
var openCmd = &cobra.Command{
	Use:     "open [name]",
	Short:   "Open the project page for a CurseForge file in your browser",
	Aliases: []string{"doc"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			cmdshared.Exitln(err)
		}
		index, err := fileio.LoadPackIndexFile(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}
		resolvedMod, ok := index.FindMod(args[0])
		if !ok {
			// TODO: should this auto-refresh?
			cmdshared.Exitln("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
		}
		modData, err := fileio.LoadMod(resolvedMod)
		if err != nil {
			cmdshared.Exitln(err)
		}
		updateData, ok := modData.GetParsedUpdateData("curseforge")
		if !ok {
			cmdshared.Exitln("Can't find CurseForge update metadata for this file")
		}
		cfUpdateData := updateData.(cfUpdateData)
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
