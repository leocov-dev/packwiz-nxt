package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func TestCanBeIncludedDirectly(t *testing.T) {
	newMod := func(mode, url string) *core.Mod {
		return &core.Mod{Download: core.ModDownload{Mode: mode, URL: url}}
	}

	t.Run("URL mode, no domain restriction", func(t *testing.T) {
		mod := newMod(core.ModeURL, "https://example.com/file.jar")
		assert.True(t, CanBeIncludedDirectly(mod, false))
	})

	t.Run("empty mode defaults to URL mode", func(t *testing.T) {
		mod := newMod("", "https://example.com/file.jar")
		assert.True(t, CanBeIncludedDirectly(mod, false))
	})

	t.Run("curseforge mode is never included directly", func(t *testing.T) {
		mod := newMod(core.ModeCF, "https://example.com/file.jar")
		assert.False(t, CanBeIncludedDirectly(mod, true))
		assert.False(t, CanBeIncludedDirectly(mod, false))
	})

	t.Run("restricted domains: whitelisted host allowed", func(t *testing.T) {
		mod := newMod(core.ModeURL, "https://cdn.modrinth.com/data/AANobbMI/file.jar")
		assert.True(t, CanBeIncludedDirectly(mod, true))
	})

	t.Run("restricted domains: github host allowed", func(t *testing.T) {
		mod := newMod(core.ModeURL, "https://github.com/owner/repo/releases/download/v1/file.jar")
		assert.True(t, CanBeIncludedDirectly(mod, true))
	})

	t.Run("restricted domains: non-whitelisted host rejected", func(t *testing.T) {
		mod := newMod(core.ModeURL, "https://example.com/file.jar")
		assert.False(t, CanBeIncludedDirectly(mod, true))
	})

	t.Run("restricted domains: unparseable URL rejected, no panic", func(t *testing.T) {
		mod := newMod(core.ModeURL, "://not a valid url")
		assert.False(t, CanBeIncludedDirectly(mod, true))
	})
}
