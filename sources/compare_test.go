package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareChain(t *testing.T) {
	t.Run("empty comparator list returns 0", func(t *testing.T) {
		assert.Equal(t, int32(0), compareChain())
	})

	t.Run("single comparator non-zero returns that value", func(t *testing.T) {
		assert.Equal(t, int32(5), compareChain(func() int32 { return 5 }))
	})

	t.Run("first comparator non-zero short-circuits", func(t *testing.T) {
		secondCalled := false
		result := compareChain(
			func() int32 { return -3 },
			func() int32 {
				secondCalled = true
				return 1
			},
		)
		assert.Equal(t, int32(-3), result)
		assert.False(t, secondCalled, "second comparator should not be called once an earlier one is non-zero")
	})

	t.Run("first ties, second breaks tie", func(t *testing.T) {
		result := compareChain(
			func() int32 { return 0 },
			func() int32 { return 7 },
		)
		assert.Equal(t, int32(7), result)
	})

	t.Run("all comparators tie returns 0", func(t *testing.T) {
		result := compareChain(
			func() int32 { return 0 },
			func() int32 { return 0 },
			func() int32 { return 0 },
		)
		assert.Equal(t, int32(0), result)
	})

	t.Run("negative result propagates", func(t *testing.T) {
		result := compareChain(
			func() int32 { return 0 },
			func() int32 { return -42 },
		)
		assert.Equal(t, int32(-42), result)
	})

	t.Run("positive result propagates", func(t *testing.T) {
		result := compareChain(
			func() int32 { return 0 },
			func() int32 { return 42 },
		)
		assert.Equal(t, int32(42), result)
	})
}
