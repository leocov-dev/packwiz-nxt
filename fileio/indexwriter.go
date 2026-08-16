package fileio

import (
	"github.com/leocov-dev/packwiz-nxt/core"
	"os"
)

type IndexWriter struct {
}

func NewIndexWriter() IndexWriter {
	return IndexWriter{}
}

func (m IndexWriter) Write(writable Writable) error {
	_, err := writeMarshalled(writable)
	return err
}

// InitIndexFile creates the index file for pack if it does not already exist.
// It returns true if a new file was created, false if the file already existed.
func InitIndexFile(pack core.PackToml) (bool, error) {
	indexFilePath := pack.Index.File
	_, err := os.Stat(indexFilePath)
	if os.IsNotExist(err) {
		err = os.WriteFile(indexFilePath, []byte{}, 0644)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, err
}
