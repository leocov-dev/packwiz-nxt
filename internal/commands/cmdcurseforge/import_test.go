package cmdcurseforge

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveZipImportSource_MissingMetaFile confirms resolveZipImportSource (the helper
// shared between the local-zip and new HTTP-fetch import paths) errors clearly when a
// zip has neither manifest.json nor minecraftinstance.json, rather than panicking or
// silently returning a nil metadata source.
func TestResolveZipImportSource_MissingMetaFile(t *testing.T) {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	w, err := zw.Create("some-other-file.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("not a manifest"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	_, err = resolveZipImportSource(zr)
	assert.ErrorContains(t, err, "manifest.json")
}
