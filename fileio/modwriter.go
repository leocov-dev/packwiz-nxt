package fileio

import "github.com/leocov-dev/packwiz-nxt/core"

type ModWriter struct {
}

func NewModWriter() ModWriter {
	return ModWriter{}
}

func (m ModWriter) Write(writable Writable) (string, string, error) {
	result, err := writeMarshalled(writable)
	if err != nil {
		return "", "", err
	}

	return result.HashFormat, result.Hash, nil
}

// WriteModAndUpdateIndex writes a mod's metadata file, then updates and rewrites the pack index
// and pack.toml to record the new file's hash. This is the common write sequence used by commands
// that install a single new mod (e.g. `url add`, `github add`).
//
// modMeta must already have its meta path set (e.g. via ModToml.SetMetaPath), and metaPath must be
// that same path, as returned by SetMetaPath.
func WriteModAndUpdateIndex(pack *core.PackToml, modMeta *core.ModToml, metaPath string) error {
	modWriter := NewModWriter()
	format, hash, err := modWriter.Write(modMeta)
	if err != nil {
		return err
	}

	index, err := LoadPackIndexFile(pack)
	if err != nil {
		return err
	}

	err = index.UpdateFileHashGiven(metaPath, format, hash, true)
	if err != nil {
		return err
	}

	repr, err := index.ToWritable()
	if err != nil {
		return err
	}
	indexWriter := NewIndexWriter()
	err = indexWriter.Write(&repr)
	if err != nil {
		return err
	}

	pack.RefreshIndexHash(index)

	packWriter := NewPackWriter()
	err = packWriter.Write(pack)
	if err != nil {
		return err
	}

	return nil
}
