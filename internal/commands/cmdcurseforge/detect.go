package cmdcurseforge

import (
	"fmt"
	"github.com/aviddiviner/go-murmur"
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/internal/shared"
	"github.com/leocov-dev/packwiz-nxt/sources"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

// TODO: make all of this less bad and hardcoded

// detectCmd represents the detect command
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect .jar files in the mods folder (experimental)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		packFile, packDir, err := shared.GetPackPaths()
		if err != nil {
			shared.Exitln(err)
		}

		pack, err := fileio.LoadAll(packFile)
		if err != nil {
			shared.Exitln(err)
		}

		// Walk files in the mods folder
		var hashes []uint32
		modPaths := make(map[uint32]string)
		err = filepath.Walk("mods", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".jar") && !strings.HasSuffix(path, ".litemod") {
				// TODO: make this less bad
				return nil
			}
			fmt.Println("Hashing " + path)
			bytes, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash := getByteArrayHash(bytes)
			hashes = append(hashes, hash)
			modPaths[hash] = path
			return nil
		})
		if err != nil {
			shared.Exitln(err)
		}
		fmt.Printf("Found %d files, submitting...\n", len(hashes))

		res, err := sources.GetCurseforgeClient().GetFingerprintInfo(hashes)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Successfully matched %d files\n", len(res.ExactFingerprints))
		if len(res.PartialMatches) > 0 {
			fmt.Println("The following fingerprints were partial and I don't know what to do!!!")
			for _, v := range res.PartialMatches {
				fmt.Printf("%s (%d)", modPaths[v], v)
			}
		}
		if len(res.UnmatchedFingerprints) > 0 {
			fmt.Printf("Failed to match the following %d files:\n", len(res.UnmatchedFingerprints))
			for _, v := range res.UnmatchedFingerprints {
				fmt.Printf("%s (%d)\n", modPaths[v], v)
			}
		}

		fmt.Println("Retrieving metadata...")
		ids := make([]uint32, len(res.ExactMatches))
		for i, v := range res.ExactMatches {
			ids[i] = v.ID
		}
		modInfos, err := sources.GetCurseforgeClient().GetModInfoMultiple(ids)
		if err != nil {
			shared.Exitf("Failed to retrieve metadata: %v", err)
		}
		modInfosMap := make(map[uint32]sources.CfModInfo)
		for _, v := range modInfos {
			modInfosMap[v.ID] = v
		}

		fmt.Println("Creating metadata files...")
		for _, v := range res.ExactMatches {
			mod, err := sources.CurseforgeNewMod(modInfosMap[v.ID], v.File, false)
			if err != nil {
				shared.Exitln(err)
			}

			pack.SetMod(mod)
		}
		fmt.Println("Detection complete!")

		err = fileio.WriteAll(*pack, packDir)
		if err != nil {
			shared.Exitln(err)
		}
	},
}

func init() {
	curseforgeCmd.AddCommand(detectCmd)
}

func getByteArrayHash(bytes []byte) uint32 {
	return murmur.MurmurHash2(computeNormalizedArray(bytes), 1)
}

func computeNormalizedArray(bytes []byte) []byte {
	var newArray []byte
	for _, b := range bytes {
		if !isWhitespaceCharacter(b) {
			newArray = append(newArray, b)
		}
	}
	return newArray
}

func isWhitespaceCharacter(b byte) bool {
	return b == 9 || b == 10 || b == 13 || b == 32
}
