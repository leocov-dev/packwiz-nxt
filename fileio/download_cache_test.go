package fileio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCacheIndex_PruneOrphaned confirms that entries whose backing file is missing or
// zero-byte are dropped, while an entry with real content is left alone - the scenario
// this guards against is a cache corrupted by a crash mid-download or manual deletion.
func TestCacheIndex_PruneOrphaned(t *testing.T) {
	cachePath := t.TempDir()

	writeCacheFile := func(hash string, content []byte) {
		dir := filepath.Join(cachePath, hash[:2])
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, hash[2:]), content, 0644))
	}

	const (
		healthyHash  = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
		zeroByteHash = "1111111111111111111111111111111111111111111111111111111111111111"
		missingHash  = "2222222222222222222222222222222222222222222222222222222222222222"
	)
	writeCacheFile(healthyHash, []byte("real content"))
	writeCacheFile(zeroByteHash, nil)
	// missingHash intentionally has no backing file at all.

	index := CacheIndex{
		cachePath: cachePath,
		Hashes: map[string][]string{
			cacheHashFormat: {healthyHash, zeroByteHash, missingHash},
		},
	}

	removed := index.PruneOrphaned()

	assert.Equal(t, 2, removed)
	assert.Equal(t, []string{healthyHash}, index.Hashes[cacheHashFormat])
}

// TestCacheIndex_EvictLRU confirms eviction removes the least-recently-used entries
// first, stopping as soon as the remaining total size is at or under the budget - the
// scenario this guards against is an unbounded cache that never shrinks.
func TestCacheIndex_EvictLRU(t *testing.T) {
	cachePath := t.TempDir()

	writeCacheFile := func(hash string, content []byte) {
		dir := filepath.Join(cachePath, hash[:2])
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, hash[2:]), content, 0644))
	}

	const (
		oldestHash = "aa11111111111111111111111111111111111111111111111111111111111111"
		middleHash = "bb22222222222222222222222222222222222222222222222222222222222222"
		newestHash = "cc33333333333333333333333333333333333333333333333333333333333333"
	)
	writeCacheFile(oldestHash, make([]byte, 10))
	writeCacheFile(middleHash, make([]byte, 10))
	writeCacheFile(newestHash, make([]byte, 10))

	index := CacheIndex{
		cachePath: cachePath,
		Hashes: map[string][]string{
			cacheHashFormat: {oldestHash, middleHash, newestHash},
		},
		LastAccess: []int64{100, 200, 300},
	}

	removed, freed, err := index.EvictLRU(15)

	require.NoError(t, err)
	assert.Equal(t, 2, removed)
	assert.Equal(t, int64(20), freed)
	assert.Equal(t, []string{newestHash}, index.Hashes[cacheHashFormat])
	assert.Equal(t, []int64{300}, index.LastAccess)

	assert.NoFileExists(t, filepath.Join(cachePath, oldestHash[:2], oldestHash[2:]))
	assert.NoFileExists(t, filepath.Join(cachePath, middleHash[:2], middleHash[2:]))
	assert.FileExists(t, filepath.Join(cachePath, newestHash[:2], newestHash[2:]))
}

// TestCacheIndex_EvictLRU_UnderBudget confirms eviction is a no-op when the cache is
// already within budget - nothing should be removed or touched on disk.
func TestCacheIndex_EvictLRU_UnderBudget(t *testing.T) {
	cachePath := t.TempDir()

	const hash = "dd44444444444444444444444444444444444444444444444444444444444444"
	dir := filepath.Join(cachePath, hash[:2])
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, hash[2:]), make([]byte, 10), 0644))

	index := CacheIndex{
		cachePath: cachePath,
		Hashes: map[string][]string{
			cacheHashFormat: {hash},
		},
		LastAccess: []int64{100},
	}

	removed, freed, err := index.EvictLRU(1000)

	require.NoError(t, err)
	assert.Equal(t, 0, removed)
	assert.Equal(t, int64(0), freed)
	assert.Equal(t, []string{hash}, index.Hashes[cacheHashFormat])
	assert.FileExists(t, filepath.Join(cachePath, hash[:2], hash[2:]))
}
