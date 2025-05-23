package core

import (
	"golang.org/x/exp/slices"
	"path"
)

// IndexFiles are stored as a map of path -> (indexFile or alias -> indexFile)
// The latter is used for multiple copies with the same path but different alias
type IndexFiles map[string]IndexPathHolder

type IndexPathHolder interface {
	updateHash(hash string, format string)
	markFound()
	markMetaFile()
	MarkedFound() bool
	IsMetaFile() bool
}

// IndexFile is a file in the index
type IndexFile struct {
	// Files are stored in forward-slash format relative to the index file
	File       string `toml:"file"`
	Hash       string `toml:"hash,omitempty"`
	HashFormat string `toml:"hash-format,omitempty"`
	Alias      string `toml:"alias,omitempty"`
	MetaFile   bool   `toml:"metafile,omitempty"` // True when it is a .toml metadata file
	Preserve   bool   `toml:"preserve,omitempty"` // Don't overwrite the file when updating
	fileFound  bool
}

func (i *IndexFile) updateHash(hash string, format string) {
	i.Hash = hash
	i.HashFormat = format
}

func (i *IndexFile) markFound() {
	i.fileFound = true
}

func (i *IndexFile) markMetaFile() {
	i.MetaFile = true
}

func (i *IndexFile) MarkedFound() bool {
	return i.fileFound
}

func (i *IndexFile) IsMetaFile() bool {
	return i.MetaFile
}

type indexFileMultipleAlias map[string]IndexFile

func (i *indexFileMultipleAlias) updateHash(hash string, format string) {
	for k, v := range *i {
		v.updateHash(hash, format)
		(*i)[k] = v // Can't mutate map value in place
	}
}

// (indexFileMultipleAlias == map[string]indexFile)
func (i *indexFileMultipleAlias) markFound() {
	for k, v := range *i {
		v.markFound()
		(*i)[k] = v // Can't mutate map value in place
	}
}

func (i *indexFileMultipleAlias) markMetaFile() {
	for k, v := range *i {
		v.markMetaFile()
		(*i)[k] = v // Can't mutate map value in place
	}
}

func (i *indexFileMultipleAlias) MarkedFound() bool {
	for _, v := range *i {
		return v.MarkedFound()
	}
	panic("No entries in indexFileMultipleAlias")
}

func (i *indexFileMultipleAlias) IsMetaFile() bool {
	for _, v := range *i {
		return v.MetaFile
	}
	panic("No entries in indexFileMultipleAlias")
}

// updateFileEntry updates the hash of a file and marks as found; adding it if it doesn't exist
// This also sets metafile if markAsMetaFile is set
// This updates all existing aliassed variants of a file, but doesn't create new ones
func (f *IndexFiles) updateFileEntry(path string, format string, hash string, markAsMetaFile bool) {
	// Ensure map is non-nil
	if *f == nil {
		*f = make(IndexFiles)
	}
	// Fetch existing entry
	file, found := (*f)[path]
	if found {
		// Exists: update hash/format/metafile
		file.markFound()
		file.updateHash(hash, format)
		if markAsMetaFile {
			file.markMetaFile()
		}
		// (don't do anything if markAsMetaFile is false - don't reset metafile status of existing metafiles)
	} else {
		// Doesn't exist: create new file data
		newFile := IndexFile{
			File:       path,
			Hash:       hash,
			HashFormat: format,
			MetaFile:   markAsMetaFile,
			fileFound:  true,
		}
		(*f)[path] = &newFile
	}
}

type IndexFilesTomlRepresentation []IndexFile

// toMemoryRep converts the TOML representation of IndexFiles to that used in memory
// These silly converter functions are necessary because the TOML libraries don't support custom non-primitive serializers
func (rep IndexFilesTomlRepresentation) toMemoryRep() IndexFiles {
	out := make(IndexFiles)

	// Add entries to map
	for _, v := range rep {
		v := v // Narrow scope of loop variable
		v.File = path.Clean(v.File)
		v.Alias = path.Clean(v.Alias)
		// path.Clean converts "" into "." - undo this for Alias as we use omitempty
		if v.Alias == "." {
			v.Alias = ""
		}
		if existing, ok := out[v.File]; ok {
			if existingFile, ok := existing.(*IndexFile); ok {
				// Is this the same as the existing file?
				if v.Alias == existingFile.Alias {
					// Yes: overwrite
					out[v.File] = &v
				} else {
					// No: convert to new map
					m := make(indexFileMultipleAlias)
					m[existingFile.Alias] = *existingFile
					m[v.Alias] = v
					out[v.File] = &m
				}
			} else if existingMap, ok := existing.(*indexFileMultipleAlias); ok {
				// Add to alias map
				(*existingMap)[v.Alias] = v
			} else {
				panic("Unknown type in IndexFiles")
			}
		} else {
			out[v.File] = &v
		}
	}

	return out
}

// toTomlRep converts the in-memory representation of IndexFiles to that used in TOML
// These silly converter functions are necessary because the TOML libraries don't support custom non-primitive serializers
func (f *IndexFiles) toTomlRep() IndexFilesTomlRepresentation {
	// Turn internal representation into TOML representation
	rep := make(IndexFilesTomlRepresentation, 0, len(*f))
	for _, v := range *f {
		if file, ok := v.(*IndexFile); ok {
			rep = append(rep, *file)
		} else if file, ok := v.(*indexFileMultipleAlias); ok {
			for _, alias := range *file {
				rep = append(rep, alias)
			}
		} else {
			panic("Unknown type in IndexFiles")
		}
	}

	slices.SortFunc(rep, func(a IndexFile, b IndexFile) int {
		if a.File == b.File {
			if a.Alias == b.Alias {
				return 0
			} else if a.Alias < b.Alias {
				return -1
			} else {
				return 1
			}
		} else {
			if a.File < b.File {
				return -1
			} else {
				return 1
			}
		}
	})

	return rep
}
