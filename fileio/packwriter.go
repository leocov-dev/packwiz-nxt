package fileio

type PackWriter struct {
}

func NewPackWriter() PackWriter {
	return PackWriter{}
}

func (p PackWriter) Write(writable Writable) error {
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
