package fileio

import (
	"github.com/pelletier/go-toml/v2"
	"os"
)

type PackWriter struct {
}

func NewPackWriter() PackWriter {
	return PackWriter{}
}

func (p PackWriter) Write(writable Writable) error {
	metaFile := writable.GetFilePath()

	f, err := os.Create(metaFile)
	if err != nil {
		return err
	}

	enc := toml.NewEncoder(f)
	// Disable indentation
	enc.SetIndentSymbol("")
	err = enc.Encode(writable)
	if err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}
