package utils

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"

	"github.com/leocov-dev/fork.packwiz/internal/cmdshared"
)

// markdownCmd represents the markdown command
var markdownCmd = &cobra.Command{
	Use:     "markdown",
	Short:   "Generate markdown documentation (that you might be reading right now!!)",
	Aliases: []string{"md"},
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		outDir := viper.GetString("utils.markdown.dir")
		err := os.MkdirAll(outDir, os.ModePerm)
		if err != nil {
			cmdshared.Exitf("Error creating directory: %s\n", err)
		}
		disableTag(cmd.Root())
		err = doc.GenMarkdownTree(cmd.Root(), outDir)
		if err != nil {
			cmdshared.Exitf("Error generating markdown: %s\n", err)
		}
		fmt.Println("Generated markdown successfully!")
	},
}

func disableTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, v := range cmd.Commands() {
		disableTag(v)
	}
}

func init() {
	utilsCmd.AddCommand(markdownCmd)

	markdownCmd.Flags().String("dir", ".", "The destination directory to save docs in")
	_ = viper.BindPFlag("utils.markdown.dir", markdownCmd.Flags().Lookup("dir"))
}
