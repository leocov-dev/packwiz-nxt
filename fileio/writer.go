package fileio

import (
	"errors"

	"github.com/leocov-dev/packwiz-nxt/core"
)

type Writable interface {
	core.HashableObject
	GetFilePath() string
}

// writeMarshalled writes the given Writable to disk: it creates the file at
// writable.GetFilePath(), marshals writable to get the bytes to write, writes
// those bytes, and closes the file. It returns the MarshalResult so callers
// can extract whatever they need (e.g. hash format/value) from it.
//
// If the write succeeds but closing the file fails (e.g. a flush error), that
// close error is still surfaced rather than silently dropped, since the file
// on disk may be incomplete. If both the write and the close fail, both
// errors are combined.
func writeMarshalled(writable Writable) (core.MarshalResult, error) {
	metaFile := writable.GetFilePath()

	f, err := CreateFile(metaFile)
	if err != nil {
		return core.MarshalResult{}, err
	}

	result, marshalErr := writable.Marshal()
	var writeErr error
	if marshalErr == nil {
		_, writeErr = f.Write(result.Value)
	}

	closeErr := f.Close()

	if marshalErr != nil {
		return result, marshalErr
	}
	if writeErr != nil {
		if closeErr != nil {
			return result, errors.Join(writeErr, closeErr)
		}
		return result, writeErr
	}
	if closeErr != nil {
		return result, closeErr
	}

	return result, nil
}
