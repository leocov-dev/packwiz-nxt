package fileio

import (
	"os"
	"path/filepath"
)

func CreateFile(path string) (*os.File, error) {
	f, err := os.Create(path)
	if err != nil {
		err2 := os.MkdirAll(filepath.Dir(path), os.ModePerm)
		if err2 == nil {
			f, err = os.Create(path)
		}
		if err != nil {
			return nil, err
		}
	}

	return f, nil
}
