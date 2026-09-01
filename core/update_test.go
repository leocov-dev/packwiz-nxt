package core_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/core/mocks"
)

func modWithUpdate(slug, source string, pin bool) *core.Mod {
	return &core.Mod{
		Name:   slug,
		Slug:   slug,
		Pin:    pin,
		Update: core.ModUpdate{source: map[string]interface{}{}},
	}
}

// TestCheckAllMods_MixedAvailability confirms CheckAllMods reports each mod's
// own UpdateAvailable/UpdateString independently.
func TestCheckAllMods_MixedAvailability(t *testing.T) {
	reg := core.NewRegistry()
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	modA := modWithUpdate("mod-a", "mock-source", false)
	modB := modWithUpdate("mod-b", "mock-source", false)
	pack.SetMod(modA)
	pack.SetMod(modB)

	updater := mocks.NewMockUpdater(t)
	updater.EXPECT().GetName().Return("mock-source")
	updater.EXPECT().
		CheckUpdate(mockModsContaining(modA, modB), pack).
		Return([]core.UpdateCheck{
			{UpdateAvailable: true, UpdateString: "a-1.0.0 -> a-1.0.1"},
			{UpdateAvailable: false},
		}, nil).
		Once()
	reg.AddUpdater(updater)

	results, err := core.CheckAllMods(reg, pack)
	require.NoError(t, err)
	require.Len(t, results, 2)

	bySlug := indexBySlug(results)
	assert.True(t, bySlug["mod-a"].UpdateAvailable)
	assert.Equal(t, "a-1.0.0 -> a-1.0.1", bySlug["mod-a"].UpdateString)
	assert.False(t, bySlug["mod-b"].UpdateAvailable)
}

// TestCheckAllMods_PerModError confirms one mod's per-check Error doesn't
// discard the results for other mods checked in the same source batch - this
// is the behavior GetUpdatableMods lacks (it aborts the whole batch instead).
func TestCheckAllMods_PerModError(t *testing.T) {
	reg := core.NewRegistry()
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	modA := modWithUpdate("mod-a", "mock-source", false)
	modB := modWithUpdate("mod-b", "mock-source", false)
	pack.SetMod(modA)
	pack.SetMod(modB)

	checkErr := errors.New("no compatible version found")

	updater := mocks.NewMockUpdater(t)
	updater.EXPECT().GetName().Return("mock-source")
	updater.EXPECT().
		CheckUpdate(mockModsContaining(modA, modB), pack).
		Return([]core.UpdateCheck{
			{UpdateAvailable: false, Error: checkErr},
			{UpdateAvailable: true},
		}, nil).
		Once()
	reg.AddUpdater(updater)

	results, err := core.CheckAllMods(reg, pack)
	require.NoError(t, err)
	require.Len(t, results, 2)

	bySlug := indexBySlug(results)
	require.Error(t, bySlug["mod-a"].Err)
	assert.ErrorIs(t, bySlug["mod-a"].Err, checkErr)
	require.NoError(t, bySlug["mod-b"].Err)
	assert.True(t, bySlug["mod-b"].UpdateAvailable)
}

// TestCheckAllMods_SourceBatchError confirms a whole-source CheckUpdate
// failure (e.g. a network/auth error) is recorded against every mod in that
// source's batch, without affecting other sources' results.
func TestCheckAllMods_SourceBatchError(t *testing.T) {
	reg := core.NewRegistry()
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	brokenMod := modWithUpdate("broken-mod", "broken-source", false)
	okMod := modWithUpdate("ok-mod", "ok-source", false)
	pack.SetMod(brokenMod)
	pack.SetMod(okMod)

	batchErr := errors.New("upstream unavailable")

	brokenUpdater := mocks.NewMockUpdater(t)
	brokenUpdater.EXPECT().GetName().Return("broken-source")
	brokenUpdater.EXPECT().
		CheckUpdate(mockModsContaining(brokenMod), pack).
		Return(nil, batchErr).
		Once()
	reg.AddUpdater(brokenUpdater)

	okUpdater := mocks.NewMockUpdater(t)
	okUpdater.EXPECT().GetName().Return("ok-source")
	okUpdater.EXPECT().
		CheckUpdate(mockModsContaining(okMod), pack).
		Return([]core.UpdateCheck{{UpdateAvailable: true}}, nil).
		Once()
	reg.AddUpdater(okUpdater)

	results, err := core.CheckAllMods(reg, pack)
	require.NoError(t, err)
	require.Len(t, results, 2)

	bySlug := indexBySlug(results)
	require.Error(t, bySlug["broken-mod"].Err)
	assert.ErrorIs(t, bySlug["broken-mod"].Err, batchErr)
	require.NoError(t, bySlug["ok-mod"].Err)
	assert.True(t, bySlug["ok-mod"].UpdateAvailable)
}

// TestCheckAllMods_PinnedModIncluded confirms CheckAllMods does not filter
// out pinned mods (unlike GetUpdatableMods), so a caller can report "pinned,
// update available but would be skipped" distinctly from "up to date".
func TestCheckAllMods_PinnedModIncluded(t *testing.T) {
	reg := core.NewRegistry()
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	pinnedMod := modWithUpdate("pinned-mod", "mock-source", true)
	pack.SetMod(pinnedMod)

	updater := mocks.NewMockUpdater(t)
	updater.EXPECT().GetName().Return("mock-source")
	updater.EXPECT().
		CheckUpdate(mockModsContaining(pinnedMod), pack).
		Return([]core.UpdateCheck{{UpdateAvailable: true}}, nil).
		Once()
	reg.AddUpdater(updater)

	results, err := core.CheckAllMods(reg, pack)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].UpdateAvailable)
	assert.True(t, results[0].Mod.Pin)
}

// TestGetUpdatableMods_FiltersPinnedButNotErrors confirms GetUpdatableMods,
// built on top of CheckAllMods, still filters pinned mods out of its applied
// result, and still aborts the whole call on a per-mod error (existing,
// unchanged behavior preserved through the refactor).
func TestGetUpdatableMods_FiltersPinnedButNotErrors(t *testing.T) {
	t.Run("filters pinned mods", func(t *testing.T) {
		reg := core.NewRegistry()
		pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

		pinnedMod := modWithUpdate("pinned-mod", "mock-source", true)
		unpinnedMod := modWithUpdate("unpinned-mod", "mock-source", false)
		pack.SetMod(pinnedMod)
		pack.SetMod(unpinnedMod)

		updater := mocks.NewMockUpdater(t)
		updater.EXPECT().GetName().Return("mock-source")
		updater.EXPECT().
			CheckUpdate(mockModsContaining(pinnedMod, unpinnedMod), pack).
			Return([]core.UpdateCheck{
				{UpdateAvailable: true},
				{UpdateAvailable: true},
			}, nil).
			Once()
		reg.AddUpdater(updater)

		updatable, err := core.GetUpdatableMods(reg, pack)
		require.NoError(t, err)

		data, ok := updatable["mock-source"]
		require.True(t, ok)
		require.Len(t, data.Mods, 1)
		assert.Equal(t, "unpinned-mod", data.Mods[0].Slug)
	})

	t.Run("aborts on a per-mod error", func(t *testing.T) {
		reg := core.NewRegistry()
		pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

		mod := modWithUpdate("broken-mod", "mock-source", false)
		pack.SetMod(mod)

		updater := mocks.NewMockUpdater(t)
		updater.EXPECT().GetName().Return("mock-source")
		updater.EXPECT().
			CheckUpdate(mockModsContaining(mod), pack).
			Return([]core.UpdateCheck{{Error: errors.New("boom")}}, nil).
			Once()
		reg.AddUpdater(updater)

		_, err := core.GetUpdatableMods(reg, pack)
		assert.Error(t, err)
	})
}

func indexBySlug(results []core.UpdateCheckResult) map[string]core.UpdateCheckResult {
	out := make(map[string]core.UpdateCheckResult, len(results))
	for _, r := range results {
		out[r.Mod.Slug] = r
	}
	return out
}

// mockModsContaining matches a []*core.Mod argument containing exactly the
// given mods, regardless of order - BuildUpdateMap iterates a map, so the
// slice order CheckUpdate receives is not guaranteed.
func mockModsContaining(mods ...*core.Mod) interface{} {
	return mock.MatchedBy(func(actual []*core.Mod) bool {
		if len(actual) != len(mods) {
			return false
		}
		for _, want := range mods {
			found := false
			for _, got := range actual {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	})
}
