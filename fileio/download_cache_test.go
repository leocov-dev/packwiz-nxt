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
