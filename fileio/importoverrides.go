package fileio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ImportOverrideFile is a minimal handle to a file provided by an external
// pack import source. It is defined structurally (rather than imported)
// so that packages providing pack-import metadata (e.g. packinterop) don't
// need to depend on fileio, avoiding an import cycle.
type ImportOverrideFile interface {
	Name() string
	Open() (io.ReadCloser, error)
}

// CopyImportOverrides copies override files sourced from an external pack
// import into packDir, skipping any file whose resolved absolute path is in
// skipPaths (e.g. because it is already covered by resolved metadata) or
// whose name is in skipNames (e.g. the pack's own metadata files). It
// returns the number of files successfully copied or skipped.
func CopyImportOverrides(files []ImportOverrideFile, packDir string, skipPaths []string, skipNames []string) int {
	successes := 0
	for _, v := range files {
		filePath := filepath.Join(packDir, filepath.FromSlash(v.Name()))
		filePathAbs, err := filepath.Abs(filePath)
		if err == nil {
			found := false
			for _, sp := range skipPaths {
				if sp == filePathAbs {
					found = true
					break
				}
			}
			if found {
				fmt.Printf("Ignored file \"%s\" (referenced by metadata)\n", filePath)
				successes++
				continue
			}

			skip := false
			for _, sn := range skipNames {
				if v.Name() == sn {
					skip = true
					break
				}
			}
			if skip {
				fmt.Printf("Ignored file \"%s\"\n", v.Name())
				successes++
				continue
			}
		}

		f, err := os.Create(filePath)
		if err != nil {
			// Attempt to create the containing directory
			err2 := os.MkdirAll(filepath.Dir(filePath), 0755)
			if err2 == nil {
				f, err = os.Create(filePath)
			}
			if err != nil {
				fmt.Printf("Failed to write file \"%s\": %s\n", filePath, err)
				if err2 != nil {
					fmt.Printf("Failed to create directories: %s\n", err)
				}
				continue
			}
		}

		src, err := v.Open()
		if err != nil {
			fmt.Printf("Failed to read file \"%s\": %s\n", filePath, err)
			f.Close()
			continue
		}
		_, err = io.Copy(f, src)
		if err != nil {
			fmt.Printf("Failed to copy file \"%s\": %s\n", filePath, err)
			f.Close()
			src.Close()
			continue
		}

		fmt.Printf("Copied file \"%s\" successfully!\n", filePath)
		f.Close()
		src.Close()
		successes++
	}
	return successes
}
