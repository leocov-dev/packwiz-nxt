package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func TestRegisterAll(t *testing.T) {
	reg := core.NewRegistry()
	RegisterAll(reg)

	for _, name := range []string{"curseforge", "github", "modrinth"} {
		updater, ok := reg.GetUpdater(name)
		if assert.True(t, ok, "expected updater %q to be registered", name) {
			assert.Equal(t, name, updater.GetName())
		}
	}
}

func TestRegisterCurseforge(t *testing.T) {
	reg := core.NewRegistry()
	RegisterCurseforge(reg)

	updater, ok := reg.GetUpdater("curseforge")
	assert.True(t, ok)
	assert.Equal(t, "curseforge", updater.GetName())

	_, ok = reg.GetUpdater("github")
	assert.False(t, ok)
}

func TestRegisterGithub(t *testing.T) {
	reg := core.NewRegistry()
	RegisterGithub(reg)

	updater, ok := reg.GetUpdater("github")
	assert.True(t, ok)
	assert.Equal(t, "github", updater.GetName())
}

func TestRegisterModrinth(t *testing.T) {
	reg := core.NewRegistry()
	RegisterModrinth(reg)

	updater, ok := reg.GetUpdater("modrinth")
	assert.True(t, ok)
	assert.Equal(t, "modrinth", updater.GetName())
}
