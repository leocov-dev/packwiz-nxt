package fileio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func TestInitIndexFile(t *testing.T) {
	t.Run("creates the file when missing", func(t *testing.T) {
		dir := t.TempDir()
		indexPath := filepath.Join(dir, "index.toml")
		pack := core.PackToml{Index: core.PackTomlIndex{File: indexPath}}

		created, err := InitIndexFile(pack)
		require.NoError(t, err)
		assert.True(t, created)
		assert.FileExists(t, indexPath)
	})

	t.Run("leaves an existing file alone", func(t *testing.T) {
		dir := t.TempDir()
		indexPath := filepath.Join(dir, "index.toml")
		require.NoError(t, os.WriteFile(indexPath, []byte("existing content"), 0644))
		pack := core.PackToml{Index: core.PackTomlIndex{File: indexPath}}

		created, err := InitIndexFile(pack)
		require.NoError(t, err)
		assert.False(t, created)

		content, err := os.ReadFile(indexPath)
		require.NoError(t, err)
		assert.Equal(t, "existing content", string(content))
	})
}

func TestIndexWriter_Write(t *testing.T) {
	resetViper(t)
	dir := t.TempDir()
	pack := testPack(t)
	require.NoError(t, WriteAll(pack, dir))

	indexPath := filepath.Join(dir, "index.toml")
	index, err := LoadIndex(indexPath)
	require.NoError(t, err)

	repr, err := index.ToWritable()
	require.NoError(t, err)

	require.NoError(t, NewIndexWriter().Write(&repr))
	assert.FileExists(t, indexPath)
}
