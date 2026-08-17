package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{"equal versions", "1.20.1", "1.20.1", 0},
		{"simple numeric ordering, less", "1.19", "1.20", -1},
		{"simple numeric ordering, greater", "1.20", "1.19", 1},
		{"shorter padded with 0 is equal", "1.20", "1.20.0", 0},
		{"shorter padded with 0 is still less", "1.20", "1.20.1", -1},
		{"numeric comparison, not lexical (1.9 < 1.10)", "1.9", "1.10", -1},
		{"numeric comparison, not lexical (1.10 > 1.9)", "1.10", "1.9", 1},
		{"non-numeric parts fall back to string comparison", "1.20-pre1", "1.20-pre2", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CompareVersions(tt.v1, tt.v2))
		})
	}
}

func TestSortDescending(t *testing.T) {
	t.Run("unsorted input sorted highest-first, numeric-aware", func(t *testing.T) {
		got := SortDescending([]string{"1.9", "1.20", "1.10"})
		assert.Equal(t, []string{"1.20", "1.10", "1.9"}, got)
	})

	t.Run("already-descending input unchanged", func(t *testing.T) {
		got := SortDescending([]string{"1.20", "1.10", "1.9"})
		assert.Equal(t, []string{"1.20", "1.10", "1.9"}, got)
	})

	t.Run("single-element slice", func(t *testing.T) {
		got := SortDescending([]string{"1.20"})
		assert.Equal(t, []string{"1.20"}, got)
	})

	t.Run("empty slice", func(t *testing.T) {
		got := SortDescending([]string{})
		assert.Equal(t, []string{}, got)
	})
}
