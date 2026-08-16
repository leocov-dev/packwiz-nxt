package fileio

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
