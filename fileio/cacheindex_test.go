package fileio

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func sha1Hex(content string) string {
	sum := sha1.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

func newTestCacheIndex(t *testing.T) *CacheIndex {
	t.Helper()
	cachePath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cachePath, "temp"), 0755))
	return &CacheIndex{
		Version:   cacheLatestVersion,
		Hashes:    map[string][]string{cacheHashFormat: {}},
		cachePath: cachePath,
	}
}

func writeThroughHandle(t *testing.T, index *CacheIndex, content string) *CacheIndexHandle {
	t.Helper()
	hash := sha256Hex(content)
	handle, exists := index.NewHandleFromHashes(map[string]string{cacheHashFormat: hash})
	require.False(t, exists)

	tempFile, err := os.CreateTemp(filepath.Join(index.cachePath, "temp"), "download-tmp")
	require.NoError(t, err)
	_, err = tempFile.WriteString(content)
	require.NoError(t, err)

	_, err = handle.CreateFromTemp(tempFile)
	require.NoError(t, err)
	handle.UpdateIndex()

	return handle
}

func TestCacheIndex_NewHandleFromHashesAndOpen(t *testing.T) {
	index := newTestCacheIndex(t)
	handle := writeThroughHandle(t, index, "hello world")

	f, err := handle.Open()
	require.NoError(t, err)
	defer f.Close()

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))

	hash := sha256Hex("hello world")
	assert.Equal(t, filepath.Join(index.cachePath, hash[:2], hash[2:]), handle.Path())
}

func TestCacheIndex_GetHandleFromHash(t *testing.T) {
	index := newTestCacheIndex(t)
	writeThroughHandle(t, index, "content a")

	t.Run("found by matching hash", func(t *testing.T) {
		hash := sha256Hex("content a")
		handle := index.GetHandleFromHash(cacheHashFormat, hash)
		require.NotNil(t, handle)
		assert.Equal(t, hash, handle.Hashes[cacheHashFormat])
	})

	t.Run("not found for unknown hash", func(t *testing.T) {
		handle := index.GetHandleFromHash(cacheHashFormat, sha256Hex("nonexistent"))
		assert.Nil(t, handle)
	})
}

func TestCacheIndex_GetHandleFromHashForce(t *testing.T) {
	index := newTestCacheIndex(t)
	writeThroughHandle(t, index, "content b")

	t.Run("rehashes to find a match in an unindexed format", func(t *testing.T) {
		handle, err := index.GetHandleFromHashForce("sha1", sha1Hex("content b"))
		require.NoError(t, err)
		require.NotNil(t, handle)
	})

	t.Run("no match returns nil, nil", func(t *testing.T) {
		handle, err := index.GetHandleFromHashForce("sha1", sha1Hex("no such content"))
		require.NoError(t, err)
		assert.Nil(t, handle)
	})
}

func TestCacheIndex_MoveImportFiles(t *testing.T) {
	index := newTestCacheIndex(t)
	importDir := filepath.Join(index.cachePath, DownloadCacheImportFolder)
	require.NoError(t, os.MkdirAll(importDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(importDir, "somefile.jar"), []byte("imported content"), 0644))

	require.NoError(t, index.MoveImportFiles())

	_, err := os.Stat(filepath.Join(importDir, "somefile.jar"))
	assert.True(t, os.IsNotExist(err), "import file should have been moved out of the import folder")

	hash := sha256Hex("imported content")
	handle := index.GetHandleFromHash(cacheHashFormat, hash)
	require.NotNil(t, handle)
}

func TestCacheIndexHandle_UpdateIndex_WarnsOnInconsistentHash(t *testing.T) {
	index := newTestCacheIndex(t)
	handle := writeThroughHandle(t, index, "original content")

	// Simulate discovering a different value for an already-recorded hash format.
	handle.Hashes[cacheHashFormat] = "0000000000000000000000000000000000000000000000000000000000000000"
	warnings := handle.UpdateIndex()
	assert.NotEmpty(t, warnings)
}

func TestCacheIndexHandle_Remove(t *testing.T) {
	index := newTestCacheIndex(t)
	handle := writeThroughHandle(t, index, "removable content")

	hash := sha256Hex("removable content")
	require.NotNil(t, index.GetHandleFromHash(cacheHashFormat, hash))

	handle.Remove()

	assert.Nil(t, index.GetHandleFromHash(cacheHashFormat, hash))
}

func TestCacheIndexHandle_GetRemainingHashes(t *testing.T) {
	handle := &CacheIndexHandle{Hashes: map[string]string{"sha256": "abc"}}
	remaining := handle.GetRemainingHashes([]string{"sha256", "sha1"})
	assert.Equal(t, []string{"sha1"}, remaining)
}
