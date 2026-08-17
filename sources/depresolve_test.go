package sources

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunDependencyResolution(t *testing.T) {
	t.Run("prepareNext on initial set returns empty stops immediately", func(t *testing.T) {
		fetchCalls := 0
		err := runDependencyResolution(
			[]int{1, 2},
			DefaultMaxDependencyCycles,
			func(newIDs []int) ([]int, error) { return nil, nil },
			func(pending []int) ([]int, error) {
				fetchCalls++
				return nil, nil
			},
		)
		require.NoError(t, err)
		assert.Equal(t, 0, fetchCalls)
	})

	t.Run("normal case progressively shrinks to empty", func(t *testing.T) {
		var prepareCalls [][]int
		var fetchCalls [][]int

		// prepareNext: [1,2] -> [1,2]; [3] -> [3]; [] -> []
		prepareNext := func(newIDs []int) ([]int, error) {
			prepareCalls = append(prepareCalls, append([]int(nil), newIDs...))
			return newIDs, nil
		}
		// fetchAndExpand: [1,2] -> [3]; [3] -> []
		fetchAndExpand := func(pending []int) ([]int, error) {
			fetchCalls = append(fetchCalls, append([]int(nil), pending...))
			if len(fetchCalls) == 1 {
				return []int{3}, nil
			}
			return nil, nil
		}

		err := runDependencyResolution([]int{1, 2}, DefaultMaxDependencyCycles, prepareNext, fetchAndExpand)
		require.NoError(t, err)

		assert.Equal(t, [][]int{{1, 2}, {3}, nil}, prepareCalls)
		assert.Equal(t, [][]int{{1, 2}, {3}}, fetchCalls)
	})

	t.Run("cycle limit hit returns error mentioning the limit", func(t *testing.T) {
		const maxCycles = 3
		fetchCalls := 0

		err := runDependencyResolution(
			[]int{1},
			maxCycles,
			func(newIDs []int) ([]int, error) { return []int{1}, nil }, // never empties
			func(pending []int) ([]int, error) {
				fetchCalls++
				return []int{1}, nil
			},
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "3")
		assert.Equal(t, maxCycles, fetchCalls, "fetchAndExpand should be called exactly maxCycles times before erroring")
	})

	t.Run("maxCycles zero errors on first non-empty pending set", func(t *testing.T) {
		fetchCalls := 0
		err := runDependencyResolution(
			[]int{1},
			0,
			func(newIDs []int) ([]int, error) { return newIDs, nil },
			func(pending []int) ([]int, error) {
				fetchCalls++
				return nil, nil
			},
		)
		require.Error(t, err)
		assert.Equal(t, 0, fetchCalls)
	})

	t.Run("prepareNext error on initial call propagates immediately", func(t *testing.T) {
		wantErr := errors.New("boom")
		fetchCalls := 0
		err := runDependencyResolution(
			[]int{1},
			DefaultMaxDependencyCycles,
			func(newIDs []int) ([]int, error) { return nil, wantErr },
			func(pending []int) ([]int, error) {
				fetchCalls++
				return nil, nil
			},
		)
		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, 0, fetchCalls)
	})

	t.Run("prepareNext error mid-loop propagates and stops fetchAndExpand", func(t *testing.T) {
		wantErr := errors.New("boom")
		fetchCalls := 0

		err := runDependencyResolution(
			[]int{1},
			DefaultMaxDependencyCycles,
			func(newIDs []int) ([]int, error) {
				if len(newIDs) == 0 {
					return nil, nil
				}
				if fetchCalls == 1 {
					return nil, wantErr
				}
				return newIDs, nil
			},
			func(pending []int) ([]int, error) {
				fetchCalls++
				return []int{2}, nil
			},
		)

		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, 1, fetchCalls, "fetchAndExpand should not be called again after prepareNext errors")
	})

	t.Run("fetchAndExpand error mid-loop propagates and stops prepareNext", func(t *testing.T) {
		wantErr := errors.New("boom")
		prepareCalls := 0

		err := runDependencyResolution(
			[]int{1},
			DefaultMaxDependencyCycles,
			func(newIDs []int) ([]int, error) {
				prepareCalls++
				return newIDs, nil
			},
			func(pending []int) ([]int, error) {
				return nil, wantErr
			},
		)

		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, 1, prepareCalls, "prepareNext should not be called again after fetchAndExpand errors")
	})
}
