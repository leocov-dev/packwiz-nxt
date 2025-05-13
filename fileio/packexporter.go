package fileio

import (
	"archive/zip"
	"github.com/leocov-dev/packwiz-nxt/core"
	"os"
	"path/filepath"
)

func ExportPack(pack core.Pack, fileName string, targetPath string) error {
	if fileName == "" {
		fileName = pack.GetExportName() + ".mrpack"
	}

	if targetPath != "" {
		targetPath = filepath.Join(targetPath, fileName)
	}

	expFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}

	zipWriter := zip.NewWriter(expFile)

	_, err = zipWriter.Create("overrides/")

	return nil
}
