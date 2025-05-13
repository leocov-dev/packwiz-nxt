package fileio

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"path/filepath"
)

type PackWriter struct {
}

func NewPackWriter() PackWriter {
	return PackWriter{}
}

func (p PackWriter) Write(writable Writable) error {
	metaFile := writable.GetFilePath()

	f, err := CreateFile(metaFile)
	if err != nil {
		return err
	}
	defer f.Close()

	result, err := writable.Marshal()
	if err != nil {
		return err
	}

	if _, err := f.Write(result.Value); err != nil {
		return err
	}

	return nil
}

func writeFile(text string, targetPath string) error {
	f, err := CreateFile(targetPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.Write([]byte(text)); err != nil {
		return err
	}

	return nil
}

func WritePackAndIndex(pack core.Pack, targetDir string) error {
	packTarget := filepath.Join(targetDir, "pack.toml")
	indexTarget := filepath.Join(targetDir, "index.toml")

	packToml, err := pack.AsPackToml()
	if err != nil {
		return err
	}
	if err = writeFile(packToml, packTarget); err != nil {
		return err
	}

	indexToml, _, err := pack.AsIndexToml()
	if err != nil {
		return err
	}
	if err = writeFile(indexToml, indexTarget); err != nil {
		return err
	}

	return nil
}

func WriteAll(pack core.Pack, targetDir string) error {
	if err := WritePackAndIndex(pack, targetDir); err != nil {
		return err
	}

	for _, mod := range pack.Mods {
		modToml, _, err := mod.AsModToml()
		if err != nil {
			return err
		}
		modTarget := filepath.Join(targetDir, mod.GetRelMetaPath())
		if err = writeFile(modToml, modTarget); err != nil {
			return err
		}
	}

	return nil
}
