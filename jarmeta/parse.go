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
// Result contract: the returned slice and error are NOT mutually exclusive.
//   - ErrNoMetadata (and a nil slice) when no supported metadata file exists.
//   - An error wrapping ErrMalformed and a nil slice when the archive itself
//     cannot be opened.
//   - Otherwise every metadata file that parsed successfully is returned, and
//     if any file failed (unreadable, oversized, invalid) the error is an
//     errors.Join of one ErrMalformed-wrapping error per failed file. A
//     multi-loader jar with one broken file therefore still yields the other
//     loaders' metadata. Callers must inspect the slice even when err != nil.
func Parse(r io.ReaderAt, size int64) ([]LoaderMeta, error) {
	return parseWith(r, size, defaultParsers())
}

func parseWith(r io.ReaderAt, size int64, parsers []formatParser) ([]LoaderMeta, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: opening jar: %w", ErrMalformed, err)
	}
	out, err := parseZip(zr, parsers)
	if len(out) > 0 {
		err = errors.Join(err, attachNested(zr, parsers, out))
	}
	if len(out) == 0 && err == nil {
		return nil, ErrNoMetadata
	}
	return out, err
}

// parseZip reads the loader metadata files of an open archive. Nested jars
// are not followed.
func parseZip(zr *zip.Reader, parsers []formatParser) ([]LoaderMeta, error) {
	var (
		out  []LoaderMeta
		errs []error
	)
	for _, p := range parsers {
		f := findEntry(zr, p.Path())
		if f == nil {
			continue
		}
		data, err := readEntry(f)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: reading %s: %w", ErrMalformed, p.Path(), err))
			continue
		}
		meta, err := p.Parse(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: parsing %s: %w", ErrMalformed, p.Path(), err))
			continue
		}
		out = append(out, meta)
	}
	return out, errors.Join(errs...)
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
	return readCapped(rc)
}

// readCapped reads r fully but fails with ErrEntryTooLarge once more than
// MaxEntrySize bytes are produced.
func readCapped(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxEntrySize+1))
	if err != nil {
		return nil, fmt.Errorf("reading entry: %w", err)
	}
	if len(data) > MaxEntrySize {
		return nil, ErrEntryTooLarge
	}
	return data, nil
}
