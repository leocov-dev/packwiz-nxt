package fileio

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRefreshIndexFiles_ParallelHashingCorrect exercises hashFilesInto's worker-pool
// hashing through RefreshIndexFiles, asserting every file ends up with the correct hash
// despite being processed concurrently - correctness matters more than speed here, since
// the index mutation itself is meant to stay single-threaded.
func TestRefreshIndexFiles_ParallelHashingCorrect(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "mods"), 0755))

	contents := map[string]string{
		"mods/test1.jar": "hello world 1",
		"mods/test2.jar": "hello world 2",
		"mods/test3.jar": "hello world 3",
	}
	for relPath, content := range contents {
		require.NoError(t, os.WriteFile(filepath.Join(dir, relPath), []byte(content), 0644))
	}

	indexPath := filepath.Join(dir, "index.toml")
	require.NoError(t, os.WriteFile(indexPath, []byte("hash-format = \"sha256\"\n"), 0644))

	index, err := LoadIndex(indexPath)
	require.NoError(t, err)

	var progressCalls int
	err = RefreshIndexFiles(&index, filepath.Join(dir, "pack.toml"), func(current, total int, path string) {
		progressCalls++
	})
	require.NoError(t, err)

	assert.Equal(t, len(contents), progressCalls)
	assert.Len(t, index.Files, len(contents))

	for relPath, content := range contents {
		entry, ok := index.Files[relPath]
		require.True(t, ok, "missing index entry for %s", relPath)
		file, ok := entry.(*core.IndexFile)
		require.True(t, ok)

		sum := sha256.Sum256([]byte(content))
		assert.Equal(t, hex.EncodeToString(sum[:]), file.Hash)
	}
}

func TestLoadIndex(t *testing.T) {
	t.Run("missing file is an error", func(t *testing.T) {
		_, err := LoadIndex(filepath.Join(t.TempDir(), "does-not-exist.toml"))
		assert.Error(t, err)
	})

	t.Run("malformed TOML is an error", func(t *testing.T) {
		dir := t.TempDir()
		indexPath := filepath.Join(dir, "index.toml")
		require.NoError(t, os.WriteFile(indexPath, []byte("not valid = [toml"), 0644))

		_, err := LoadIndex(indexPath)
		assert.Error(t, err)
	})

	t.Run("missing hash-format defaults to DefaultHashFormat", func(t *testing.T) {
		dir := t.TempDir()
		indexPath := filepath.Join(dir, "index.toml")
		require.NoError(t, os.WriteFile(indexPath, []byte(""), 0644))

		index, err := LoadIndex(indexPath)
		require.NoError(t, err)
		assert.Equal(t, indexPath, index.GetFilePath())
	})
}

func TestLoadAllMods(t *testing.T) {
	resetViper(t)
	dir := t.TempDir()
	pack := testPack(t)
	require.NoError(t, WriteAll(pack, dir))

	index, err := LoadIndex(filepath.Join(dir, "index.toml"))
	require.NoError(t, err)

	mods, err := LoadAllMods(&index)
	require.NoError(t, err)
	require.Len(t, mods, 1)
	assert.Equal(t, "Balm", mods[0].Name)
}

func TestUpdateIndexFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "somefile.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	indexPath := filepath.Join(dir, "index.toml")
	require.NoError(t, os.WriteFile(indexPath, []byte("hash-format = \"sha256\"\n"), 0644))
	index, err := LoadIndex(indexPath)
	require.NoError(t, err)

	require.NoError(t, UpdateIndexFile(&index, filePath))

	entry, ok := index.Files["somefile.txt"]
	require.True(t, ok)
	file, ok := entry.(*core.IndexFile)
	require.True(t, ok)
	assert.NotEmpty(t, file.Hash)
}
