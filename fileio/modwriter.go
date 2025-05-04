package fileio

type ModWriter struct {
}

func NewModWriter() ModWriter {
	return ModWriter{}
}

func (m ModWriter) Write(writable Writable) (string, string, error) {
	metaFile := writable.GetFilePath()

	f, err := CreateFile(metaFile)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	result, err := writable.Marshal()
	if err != nil {
		return "", "", err
	}

	if _, err := f.Write(result.Value); err != nil {
		return "", "", err
	}

	return result.HashFormat, result.Hash, nil
}
