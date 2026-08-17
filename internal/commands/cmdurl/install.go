package cmdurl

import (
	"context"
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

var installCmd = &cobra.Command{
	Use:     "add [name] [url]",
	Short:   "Add an external file from a direct download link, for sites that are not directly supported by packwiz",
	Aliases: []string{"install", "get"},
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := fileio.LoadPackFile(viper.GetString("pack-file"))
		if err != nil {
			shared.Exitln(err)
		}

		dl, err := url.Parse(args[1])
		if err != nil {
			shared.Exitln("Failed to parse URL:", err)
		}
		if dl.Scheme != "https" && dl.Scheme != "http" {
			shared.Exitln("Unsupported URL scheme:", dl.Scheme)
		}

		// TODO: consider using colors for these warnings but those can have issues on windows
		force, err := cmd.Flags().GetBool("force")
		if !force && err == nil {
			var msg string
			// TODO: update when github command is added
			// TODO: make this generic?
			//if dl.Host == "www.github.com" || dl.Host == "github.com" {
			//	msg = "github add " + args[1]
			//}
			if strings.HasSuffix(dl.Host, "modrinth.com") {
				msg = "modrinth add " + args[1]
			}
			if strings.HasSuffix(dl.Host, "curseforge.com") || strings.HasSuffix(dl.Host, "forgecdn.net") {
				msg = "curseforge add " + args[1]
			}
			if msg != "" {
				shared.Exitln("Consider using packwiz", msg, "instead; if you know what you are doing use --force to add this file without update metadata.")
			}
		}

		hash, err := getHash(cmd.Context(), args[1])
		if err != nil {
			shared.Exitln("Failed to retrieve SHA256 hash for file", err)
		}

		filename := path.Base(dl.Path)
		modMeta := core.ModToml{
			Name:     args[0],
			FileName: filename,
			Side:     core.UniversalSide,
			Download: core.ModDownload{
				URL:        args[1],
				HashFormat: "sha256",
				Hash:       hash,
			},
		}

		folder := viper.GetString("meta-folder")
		if folder == "" {
			folder = "mods"
		}
		destPathName, err := cmd.Flags().GetString("meta-name")
		if err != nil {
			shared.Exitln(err)
		}
		if destPathName == "" {
			destPathName = core.SlugifyName(args[0])
		}
		destPath := modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder,
			destPathName+core.MetaExtension))

		err = fileio.WriteModAndUpdateIndex(&pack, &modMeta, destPath)
		if err != nil {
			shared.Exitln(err)
		}
		fmt.Printf("Successfully added %s (%s) from: %s\n", args[0], destPath, args[1])
	}}

// getHash retrieves the SHA256 hash of the file at the given URL by downloading it through the
// shared fileio download/cache machinery (the same DownloadSession used by the other providers),
// rather than hand-rolling an HTTP fetch-and-hash.
func getHash(ctx context.Context, url string) (string, error) {
	dlMod := &core.Mod{
		Download: core.ModDownload{
			URL:  url,
			Mode: core.ModeURL,
		},
	}

	session, err := fileio.CreateDownloadSession(nil, []*core.Mod{dlMod}, []string{"sha256"})
	if err != nil {
		return "", err
	}

	var hash string
	for dl := range session.StartDownloads(ctx) {
		if dl.File != nil {
			_ = dl.File.Close()
		}
		if dl.Error != nil {
			return "", dl.Error
		}
		hash = dl.Hashes["sha256"]
	}

	if err := session.SaveIndex(); err != nil {
		return "", err
	}

	return hash, nil
}

func init() {
	urlCmd.AddCommand(installCmd)

	installCmd.Flags().Bool("force", false, "Add a file even if the download URL is supported by packwiz in an alternative command (which may support dependencies and updates)")
	installCmd.Flags().String("meta-name", "", "Filename to use for the created metadata file (defaults to a name generated from the name you supply)")
}
