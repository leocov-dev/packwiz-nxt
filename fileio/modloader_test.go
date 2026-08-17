package fileio

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func TestLoadMod(t *testing.T) {
	t.Run("round trip through write and load", func(t *testing.T) {
		dir := t.TempDir()
		modPath := filepath.Join(dir, "balm.pw.toml")

		mod := core.ModToml{
			Name:     "Balm",
			FileName: "balm.jar",
			Side:     core.UniversalSide,
			Download: core.ModDownload{URL: "https://example.com/balm.jar", HashFormat: "sha256", Hash: "abc123"},
		}
		mod.SetMetaPath(modPath)

		_, _, err := NewModWriter().Write(&mod)
		require.NoError(t, err)

		loaded, err := LoadMod(modPath)
		require.NoError(t, err)
		assert.Equal(t, "Balm", loaded.Name)
		assert.Equal(t, "balm.jar", loaded.FileName)
		assert.Equal(t, modPath, loaded.GetFilePath())
	})

	t.Run("missing file is an error", func(t *testing.T) {
		_, err := LoadMod(filepath.Join(t.TempDir(), "does-not-exist.pw.toml"))
		assert.Error(t, err)
	})

	t.Run("malformed TOML is an error", func(t *testing.T) {
		dir := t.TempDir()
		modPath := filepath.Join(dir, "bad.pw.toml")
		require.NoError(t, writeFile("not valid = [toml", modPath))

		_, err := LoadMod(modPath)
		assert.Error(t, err)
	})

	t.Run("unregistered update plugin is an error", func(t *testing.T) {
		dir := t.TempDir()
		modPath := filepath.Join(dir, "unknown.pw.toml")
		mod := core.ModToml{
			Name:     "Unknown",
			FileName: "unknown.jar",
			Update:   core.ModUpdate{"not-a-real-updater": core.ModSourceData{}},
		}
		mod.SetMetaPath(modPath)
		_, _, err := NewModWriter().Write(&mod)
		require.NoError(t, err)

		_, err = LoadMod(modPath)
		assert.Error(t, err)
	})
}
