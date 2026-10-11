package jarmeta

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
)

// MaxEntrySize is the hard cap, in bytes, on any metadata entry read.
const MaxEntrySize = 1 << 20

var (
	// ErrNoMetadata means the jar holds none of the supported metadata files.
	ErrNoMetadata = errors.New("no mod loader metadata found")
	// ErrMalformed means the jar or one of its metadata files is unreadable
	// or lacks required identity.
	ErrMalformed = errors.New("malformed jar or metadata")
	// ErrEntryTooLarge means a metadata entry exceeds MaxEntrySize. It is
	// always returned wrapped together with ErrMalformed.
	ErrEntryTooLarge = errors.New("metadata entry too large")
)

// formatParser parses one loader metadata file found at a fixed zip path.
// Supporting a new loader means adding an implementation to defaultParsers.
type formatParser interface {
	// Path is the exact zip entry name this parser reads.
	Path() string
	// Parse converts the raw file content into LoaderMeta.
	Parse(data []byte) (LoaderMeta, error)
}

func defaultParsers() []formatParser {
	return []formatParser{
		fabricParser{},
		quiltParser{},
		forgeParser{loader: LoaderNeoForge, path: "META-INF/neoforge.mods.toml"},
		forgeParser{loader: LoaderForge, path: "META-INF/mods.toml"},
	}
}

// Parse reads the loader metadata files of the jar in r and returns one
// LoaderMeta per loader found. Only the known metadata entries (exact paths)
// are read, each capped at MaxEntrySize.
//
// It returns ErrNoMetadata when none exist, and an error wrapping ErrMalformed
// when the archive or any metadata file is invalid.
func Parse(r io.ReaderAt, size int64) ([]LoaderMeta, error) {
	return parseWith(r, size, defaultParsers())
}

func parseWith(r io.ReaderAt, size int64, parsers []formatParser) ([]LoaderMeta, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: opening jar: %w", ErrMalformed, err)
	}

	var out []LoaderMeta
	for _, p := range parsers {
		f := findEntry(zr, p.Path())
		if f == nil {
			continue
		}
		data, err := readEntry(f)
		if err != nil {
			return nil, fmt.Errorf("%w: reading %s: %w", ErrMalformed, p.Path(), err)
		}
		meta, err := p.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%w: parsing %s: %w", ErrMalformed, p.Path(), err)
		}
		out = append(out, meta)
	}
	if len(out) == 0 {
		return nil, ErrNoMetadata
	}
	return out, nil
}

func findEntry(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// readEntry reads a zip entry, enforcing MaxEntrySize on both the declared
// size and the bytes actually decompressed (declared sizes can lie).
func readEntry(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > MaxEntrySize {
		return nil, ErrEntryTooLarge
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("opening entry: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, MaxEntrySize+1))
	if err != nil {
		return nil, fmt.Errorf("reading entry: %w", err)
	}
	if len(data) > MaxEntrySize {
		return nil, ErrEntryTooLarge
	}
	return data, nil
}
