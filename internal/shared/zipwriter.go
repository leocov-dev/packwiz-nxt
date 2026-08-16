package shared

import (
	"archive/zip"
	"fmt"
	"os"
)

// WithZipWriter creates the zip file at path, invokes fn with a *zip.Writer
// to populate it, then closes the writer and the underlying file. It centralizes
// the create/close lifecycle so callers (e.g. the modrinth/curseforge export
// commands) don't need to repeat manual close-on-error cleanup at every
// error-return point.
//
// If fn returns an error, WithZipWriter still attempts to close the zip
// writer and file (best-effort, ignoring their errors) before returning fn's
// error. If fn succeeds, any error from closing the zip writer or file is
// returned instead.
func WithZipWriter(path string, fn func(*zip.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create zip: %w", err)
	}

	zw := zip.NewWriter(f)

	fnErr := fn(zw)
	if fnErr != nil {
		_ = zw.Close()
		_ = f.Close()
		return fnErr
	}

	if err := zw.Close(); err != nil {
		_ = f.Close()
		return fmt.Errorf("error writing export file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("error writing export file: %w", err)
	}

	return nil
}
