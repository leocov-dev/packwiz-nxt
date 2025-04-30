package fileio

type Writable interface {
	GetFilePath() string
	UpdateHash(format, hash string)
}
