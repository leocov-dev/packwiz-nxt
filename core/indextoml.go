package core

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// IndexFS is a representation of the index.toml file for referencing all the files in a pack.
type IndexFS struct {
	DefaultModHashFormat string
	Files                IndexFiles
	packRoot             string

	hashFormat string
	hash       string
}

func NewIndexFromTomlRepr(rep IndexTomlRepresentation) IndexFS {
	return IndexFS{
		DefaultModHashFormat: rep.DefaultModHashFormat,
		Files:                rep.Files.toMemoryRep(),
		packRoot:             filepath.Dir(rep.GetFilePath()),
	}
}

func (in *IndexFS) GetFilePath() string {
	return filepath.Join(in.packRoot, "index.toml")
}

func (in *IndexFS) GetPackRoot() string {
	return in.packRoot
}

func (in *IndexFS) GetHashFormat() string {
	return in.hashFormat
}

func (in *IndexFS) GetHash() string {
	return in.hash
}

// RemoveFile removes a file from the index, given a file path
func (in *IndexFS) RemoveFile(path string) error {
	relPath, err := in.RelIndexPath(path)
	if err != nil {
		return err
	}
	delete(in.Files, relPath)
	return nil
}

func (in *IndexFS) UpdateFileHashGiven(path, format, hash string, markAsMetaFile bool) error {
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
func (in *IndexFS) ResolveIndexPath(p string) string {
	return filepath.Join(in.packRoot, filepath.FromSlash(p))
}

// RelIndexPath turns a file path on disk into a path from the index
func (in *IndexFS) RelIndexPath(p string) (string, error) {
	rel, err := filepath.Rel(in.packRoot, p)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (in *IndexFS) ToWritable() IndexTomlRepresentation {
	return IndexTomlRepresentation{
		DefaultModHashFormat: in.DefaultModHashFormat,
		Files:                in.Files.toTomlRep(),
		filePath:             in.GetFilePath(),
		hashFormat:           in.GetHashFormat(),
		hash:                 in.GetHash(),
	}
}

// FindMod finds a mod in the index and returns its path and whether it has been found
func (in *IndexFS) FindMod(modName string) (string, bool) {
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

// GetAllMods finds paths to every metadata file (ModToml) in the index
func (in *IndexFS) GetAllMods() []string {
	var list []string
	for p, v := range in.Files {
		if v.IsMetaFile() {
			list = append(list, in.ResolveIndexPath(p))
		}
	}
	return list
}

// IndexTomlRepresentation is the TOML representation of IndexFS (Files must be converted)
type IndexTomlRepresentation struct {
	DefaultModHashFormat string                       `toml:"hash-format"`
	Files                IndexFilesTomlRepresentation `toml:"files"`

	filePath   string
	hashFormat string
	hash       string
}

func (it *IndexTomlRepresentation) GetFilePath() string {
	return it.filePath
}

func (it *IndexTomlRepresentation) SetFilePath(path string) {
	it.filePath = path
}

func (it *IndexTomlRepresentation) UpdateHash(format, hash string) {
	it.hashFormat = format
	it.hash = hash
}

func (it *IndexTomlRepresentation) GetHashFormat() string {
	return "sha256"
}

func (it *IndexTomlRepresentation) Marshal() (MarshalResult, error) {
	result := MarshalResult{
		HashFormat: it.GetHashFormat(),
	}

	var err error

	result.Value, err = toml.Marshal(it)
	if err != nil {
		return result, err
	}

	stringer, err := GetHashImpl(result.HashFormat)
	if err != nil {
		return result, err
	}

	if _, err := stringer.Write(result.Value); err != nil {
		return result, err
	}

	result.Hash = stringer.String()

	it.UpdateHash(result.HashFormat, result.Hash)

	return result, nil
}
