package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurseforgeFileHash(t *testing.T) {
	t.Run("deterministic for the same input", func(t *testing.T) {
		data := []byte("mod file contents")
		assert.Equal(t, curseforgeFileHash(data), curseforgeFileHash(data))
	})

	t.Run("different input produces a different hash", func(t *testing.T) {
		assert.NotEqual(t, curseforgeFileHash([]byte("mod file contents")), curseforgeFileHash([]byte("different contents")))
	})
}
