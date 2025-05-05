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
