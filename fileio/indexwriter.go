package fileio

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/pelletier/go-toml/v2"
	"io"
	"os"
	"path/filepath"
)

type IndexWriter struct {
}

func NewIndexWriter() IndexWriter {
	return IndexWriter{}
}

func (m IndexWriter) Write(writable Writable) (string, string, error) {
	hashFormat := "sha256"
	metaFile := writable.GetFilePath()

	f, err := os.Create(metaFile)
	if err != nil {
		// Attempt to create the containing directory
		err2 := os.MkdirAll(filepath.Dir(metaFile), os.ModePerm)
		if err2 == nil {
			f, err = os.Create(metaFile)
		}
		if err != nil {
			return "", "", err
		}
	}

	h, err := core.GetHashImpl(hashFormat)
	if err != nil {
		_ = f.Close()
		return "", "", err
	}
	w := io.MultiWriter(h, f)

	enc := toml.NewEncoder(w)
	// Disable indentation
	enc.SetIndentSymbol("")
	err = enc.Encode(writable)
	hashString := h.String()

	writable.UpdateHash(hashFormat, hashString)
	if err != nil {
		_ = f.Close()
		return hashFormat, hashString, err
	}
	return hashFormat, hashString, f.Close()
}
