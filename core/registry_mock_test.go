package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/core/mocks"
)

// TestRegistry_MockUpdater_CheckUpdate is the reference example for using the
// mockery-generated core/mocks in place of a hand-written fake: it registers a
// MockUpdater on a Registry and asserts CheckUpdate is invoked exactly once with the
// expected mod slice, which a hand-written fake can't verify without extra bookkeeping.
func TestRegistry_MockUpdater_CheckUpdate(t *testing.T) {
	reg := core.NewRegistry()
	mods := []*core.Mod{{Name: "test-mod"}}
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	mockUpdater := mocks.NewMockUpdater(t)
	mockUpdater.EXPECT().GetName().Return("mock-source")
	mockUpdater.EXPECT().CheckUpdate(mods, pack).Return([]core.UpdateCheck{{UpdateAvailable: true}}, nil).Once()

	reg.AddUpdater(mockUpdater)

	updater, ok := reg.GetUpdater("mock-source")
	require.True(t, ok)

	results, err := updater.CheckUpdate(mods, pack)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].UpdateAvailable)
}
