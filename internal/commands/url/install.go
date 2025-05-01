package url

import (
	"fmt"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/cmdshared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
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
			cmdshared.Exitln(err)
		}

		dl, err := url.Parse(args[1])
		if err != nil {
			cmdshared.Exitln("Failed to parse URL:", err)
		}
		if dl.Scheme != "https" && dl.Scheme != "http" {
			cmdshared.Exitln("Unsupported URL scheme:", dl.Scheme)
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
				cmdshared.Exitln("Consider using packwiz", msg, "instead; if you know what you are doing use --force to add this file without update metadata.")
			}
		}

		hash, err := getHash(args[1])
		if err != nil {
			cmdshared.Exitln("Failed to retrieve SHA256 hash for file", err)
		}

		index, err := fileio.LoadPackIndexFile(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}

		filename := path.Base(dl.Path)
		modMeta := core.Mod{
			Name:     args[0],
			FileName: filename,
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
			cmdshared.Exitln(err)
		}
		if destPathName == "" {
			destPathName = core.SlugifyName(args[0])
		}
		destPath := modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder,
			destPathName+core.MetaExtension))

		modWriter := fileio.NewModWriter()
		format, hash, err := modWriter.Write(&modMeta)
		if err != nil {
			cmdshared.Exitln(err)
		}

		err = index.UpdateFileHashGiven(destPath, format, hash, true)
		if err != nil {
			cmdshared.Exitln(err)
		}

		repr := index.ToWritable()
		writer := fileio.NewIndexWriter()
		format, hash, err = writer.Write(&repr)
		if err != nil {
			cmdshared.Exitln(err)
		}

		pack.RefreshIndexHash(format, hash)

		packWriter := fileio.NewPackWriter()
		err = packWriter.Write(&pack)
		if err != nil {
			cmdshared.Exitln(err)
		}
		fmt.Printf("Successfully added %s (%s) from: %s\n", args[0], destPath, args[1])
	}}

func getHash(url string) (string, error) {
	mainHasher, err := core.GetHashImpl("sha256")
	if err != nil {
		return "", err
	}
	resp, err := core.GetWithUA(url, "application/octet-stream")
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download: unexpected response status: %v", resp.Status)
	}

	_, err = io.Copy(mainHasher, resp.Body)
	if err != nil {
		return "", err
	}

	return mainHasher.String(), nil
}

func init() {
	urlCmd.AddCommand(installCmd)

	installCmd.Flags().Bool("force", false, "Add a file even if the download URL is supported by packwiz in an alternative command (which may support dependencies and updates)")
	installCmd.Flags().String("meta-name", "", "Filename to use for the created metadata file (defaults to a name generated from the name you supply)")
}
