package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeUpdater is a minimal core.Updater used only to prove Registry isolation - its
// methods beyond GetName are never exercised by this test.
type fakeUpdater struct{ name string }

func (f fakeUpdater) GetName() string                         { return f.name }
func (f fakeUpdater) ParseUpdate(map[string]any) (any, error) { return nil, nil }
func (f fakeUpdater) CheckUpdate([]*Mod, Pack) ([]UpdateCheck, error) {
	return nil, nil
}
func (f fakeUpdater) DoUpdate([]*Mod, []any) error { return nil }

// TestRegistryIsolation confirms that Mod.GetUpdater (and, by construction,
// ModToml.GetUpdater, which shares the same updaterFor helper) resolves against the
// *Registry explicitly passed in, not a global default - two independently-constructed
// Registries never see each other's registered Updaters.
func TestRegistryIsolation(t *testing.T) {
	regA := NewRegistry()
	regB := NewRegistry()
	regA.AddUpdater(fakeUpdater{name: "fake-source"})

	mod := &Mod{
		Name:   "test-mod",
		Update: ModUpdate{"fake-source": map[string]interface{}{}},
	}

	updaterA, err := mod.GetUpdater(regA)
	assert.NoError(t, err)
	assert.Equal(t, "fake-source", updaterA.GetName())

	_, err = mod.GetUpdater(regB)
	assert.Error(t, err, "regB never had fake-source registered, so this must fail")
}
