package fileio

import (
	"fmt"
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

func InitIndexFile(pack core.PackToml) error {
	indexFilePath := pack.Index.File
	_, err := os.Stat(indexFilePath)
	if os.IsNotExist(err) {
		err = os.WriteFile(indexFilePath, []byte{}, 0644)
		if err != nil {
			return err
		}
		fmt.Println(indexFilePath + " created!")
	}
	return err
}
