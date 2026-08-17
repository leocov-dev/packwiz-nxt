package fileio

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

// resetViper snapshots and restores viper's global config map around a test,
// since core.ValidatePack (called from LoadPackFile) merges pack options into
// the process-wide viper singleton as a side effect.
func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func testPack(t *testing.T) core.Pack {
	t.Helper()
	pack := core.NewPack("PackA", "dev", "1.0.0", "a nice pack", "1.20.1", map[string]string{"fabric": "0.15.0"})

	// No Update source is set (a URL-mode mod) since ReflectUpdateData (invoked by
	// LoadMod) resolves updaters against core.DefaultRegistry, which this package
	// never populates - fileio must not depend on sources per AGENTS.md.
	mod := core.NewMod(
		"balm", "Balm", "balm-fabric.jar", core.UniversalSide, "mods", "",
		false, false,
		core.ModUpdate{},
		core.ModDownload{URL: "https://example.com/balm.jar", HashFormat: "sha1", Hash: "abc123"},
		nil,
	)
	pack.SetMod(mod)

	return *pack
}

func TestWriteAllThenLoadAll_RoundTrip(t *testing.T) {
	resetViper(t)
	dir := t.TempDir()
	pack := testPack(t)

	require.NoError(t, WriteAll(pack, dir))

	loaded, err := LoadAll(filepath.Join(dir, "pack.toml"))
	require.NoError(t, err)

	assert.Equal(t, pack.Name, loaded.Name)
	assert.Equal(t, pack.Author, loaded.Author)
	assert.Equal(t, pack.Version, loaded.Version)
	assert.Equal(t, pack.Versions, loaded.Versions)
	require.Contains(t, loaded.Mods, "balm")
	assert.Equal(t, "Balm", loaded.Mods["balm"].Name)
	assert.Equal(t, "balm-fabric.jar", loaded.Mods["balm"].FileName)
}

func TestLoadPackFile(t *testing.T) {
	resetViper(t)

	t.Run("missing file is an error", func(t *testing.T) {
		_, err := LoadPackFile(filepath.Join(t.TempDir(), "does-not-exist.toml"))
		assert.Error(t, err)
	})

	t.Run("malformed TOML is an error", func(t *testing.T) {
		dir := t.TempDir()
		packPath := filepath.Join(dir, "pack.toml")
		require.NoError(t, writeFile("not valid = [toml", packPath))

		_, err := LoadPackFile(packPath)
		assert.Error(t, err)
	})

	t.Run("valid pack.toml loads and sets file path", func(t *testing.T) {
		dir := t.TempDir()
		pack := testPack(t)
		require.NoError(t, WritePackAndIndex(pack, dir))

		packPath := filepath.Join(dir, "pack.toml")
		loaded, err := LoadPackFile(packPath)
		require.NoError(t, err)
		assert.Equal(t, pack.Name, loaded.Name)
		assert.Equal(t, packPath, loaded.GetFilePath())
	})
}

func TestWritePackAndIndex(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(t)

	require.NoError(t, WritePackAndIndex(pack, dir))

	assert.FileExists(t, filepath.Join(dir, "pack.toml"))
	assert.FileExists(t, filepath.Join(dir, "index.toml"))
}
