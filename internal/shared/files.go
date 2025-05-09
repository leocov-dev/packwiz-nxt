package shared

import (
	"github.com/spf13/viper"
	"path/filepath"
)

func GetPackPaths() (string, string, error) {
	packFile, err := filepath.Abs(viper.GetString("pack-file"))
	if err != nil {
		return "", "", err
	}

	packDir := filepath.Dir(packFile)

	return packFile, packDir, nil
}
