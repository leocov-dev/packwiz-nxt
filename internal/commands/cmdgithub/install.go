package cmdgithub

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"path/filepath"
)

var branchFlag string
var regexFlag string

func init() {
	githubCmd.AddCommand(installCmd)

	installCmd.Flags().StringVar(&branchFlag, "branch", "", "The GitHub repository branch to retrieve releases for")
	installCmd.Flags().StringVar(&regexFlag, "regex", "", "The regular expression to match releases against")
}

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "add [URL|slug]",
	Short:   "Add a project from a GitHub repository URL or slug",
	Aliases: []string{"install", "get"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			shared.Exitln(err)
		}

		if len(args) == 0 || len(args[0]) == 0 {
			shared.Exitln("You must specify a GitHub repository URL.")
		}

		modType := viper.GetString("meta-folder")
		if modType == "" {
			modType = "mods"
		}

		slugOrUrl := args[0]

		mod, err := sources.NewGitHubMod(
			args[0],
			branchFlag,
			regexFlag,
			modType,
		)
		if err != nil {
			shared.Exitf("Failed to add project: %s\n", err)
		}

		modMeta := mod.ToModMeta()

		var path string

		path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), modMeta.GetMetaRelativePath()))

		err = writeMod(pack, modMeta, path)
		if err != nil {
			shared.Exitf("Failed to add project: %s\n", err)
		}

		fmt.Printf("Project \"%s\" successfully added! (%s)\n", slugOrUrl, mod.FileName)
	},
}

func writeMod(pack core.PackToml, modMeta core.ModToml, path string) error {
	// If the file already exists, this will overwrite it!!!
	// TODO: Should this be improved?
	// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
	// VCS anyway.
	modWriter := fileio.NewModWriter()
	format, hash, err := modWriter.Write(&modMeta)
	if err != nil {
		return err
	}

	index, err := fileio.LoadPackIndexFile(&pack)
	if err != nil {
		return err
	}

	err = index.UpdateFileHashGiven(path, format, hash, true)
	if err != nil {
		return err
	}

	repr := index.ToWritable()
	writer := fileio.NewIndexWriter()
	err = writer.Write(&repr)
	if err != nil {
		return err
	}

	pack.RefreshIndexHash(index)

	packWriter := fileio.NewPackWriter()
	err = packWriter.Write(&pack)
	if err != nil {
		return err
	}

	return nil
}
