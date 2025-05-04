package fileio

import "github.com/leocov-dev/packwiz-nxt/core"

type Writable interface {
	core.HashableObject
	GetFilePath() string
	UpdateHash(format, hash string)
}
