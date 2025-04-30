package fileio

type Writable interface {
	GetFilePath() string
	UpdateHash(format, hash string)
}

type Writer interface {
	Write(writable Writable) (string, string, error)
}
