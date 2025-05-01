package core

import (
	"path"
	"path/filepath"
	"strings"
)

// Index is a representation of the index.toml file for referencing all the files in a pack.
type Index struct {
	DefaultModHashFormat string
	Files                IndexFiles
	packRoot             string

	hashFormat string
	hash       string
}

func NewIndexFromTomlRepr(rep IndexTomlRepresentation) Index {
	return Index{
		DefaultModHashFormat: rep.DefaultModHashFormat,
		Files:                rep.Files.toMemoryRep(),
		packRoot:             filepath.Dir(rep.GetFilePath()),
	}
}

func (in *Index) GetFilePath() string {
	return filepath.Join(in.packRoot, "index.toml")
}

func (in *Index) GetPackRoot() string {
	return in.packRoot
}

// RemoveFile removes a file from the index, given a file path
func (in *Index) RemoveFile(path string) error {
	relPath, err := in.RelIndexPath(path)
	if err != nil {
		return err
	}
	delete(in.Files, relPath)
	return nil
}

func (in *Index) UpdateFileHashGiven(path, format, hash string, markAsMetaFile bool) error {
	// Remove format if equal to index hash format
	if in.DefaultModHashFormat == format {
		format = ""
	}

	// Find in index
	relPath, err := in.RelIndexPath(path)
	if err != nil {
		return err
	}
	in.Files.updateFileEntry(relPath, format, hash, markAsMetaFile)
	return nil
}

// ResolveIndexPath turns a path from the index into a file path on disk
func (in *Index) ResolveIndexPath(p string) string {
	return filepath.Join(in.packRoot, filepath.FromSlash(p))
}

// RelIndexPath turns a file path on disk into a path from the index
func (in *Index) RelIndexPath(p string) (string, error) {
	rel, err := filepath.Rel(in.packRoot, p)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (in *Index) ToWritable() IndexTomlRepresentation {
	return IndexTomlRepresentation{
		DefaultModHashFormat: in.DefaultModHashFormat,
		Files:                in.Files.toTomlRep(),
		index:                in,
	}
}

// FindMod finds a mod in the index and returns its path and whether it has been found
func (in *Index) FindMod(modName string) (string, bool) {
	for p, v := range in.Files {
		if v.IsMetaFile() {
			_, fileName := path.Split(p)
			fileTrimmed := strings.TrimSuffix(strings.TrimSuffix(fileName, MetaExtension), MetaExtensionOld)
			if fileTrimmed == modName {
				return in.ResolveIndexPath(p), true
			}
		}
	}
	return "", false
}

// GetAllMods finds paths to every metadata file (Mod) in the index
func (in *Index) GetAllMods() []string {
	var list []string
	for p, v := range in.Files {
		if v.IsMetaFile() {
			list = append(list, in.ResolveIndexPath(p))
		}
	}
	return list
}

// IndexTomlRepresentation is the TOML representation of Index (Files must be converted)
type IndexTomlRepresentation struct {
	DefaultModHashFormat string                       `toml:"hash-format"`
	Files                IndexFilesTomlRepresentation `toml:"files"`

	filePath string
	index    *Index
}

func (it *IndexTomlRepresentation) GetFilePath() string {
	return it.filePath
}

func (it *IndexTomlRepresentation) SetFilePath(path string) {
	it.filePath = path
}

func (it *IndexTomlRepresentation) UpdateHash(format, hash string) {
	it.index.hashFormat = format
	it.index.hash = hash
}
