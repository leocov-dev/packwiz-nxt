package fileio

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func TestModWriter_Write(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "balm.pw.toml")
	mod := core.ModToml{Name: "Balm", FileName: "balm.jar"}
	mod.SetMetaPath(modPath)

	hashFormat, hash, err := NewModWriter().Write(&mod)
	require.NoError(t, err)
	assert.NotEmpty(t, hashFormat)
	assert.NotEmpty(t, hash)
	assert.FileExists(t, modPath)
}

func TestWriteModAndUpdateIndex(t *testing.T) {
	resetViper(t)
	dir := t.TempDir()
	pack := testPack(t)
	require.NoError(t, WriteAll(pack, dir))

	packPath := filepath.Join(dir, "pack.toml")
	packToml, err := LoadPackFile(packPath)
	require.NoError(t, err)

	viper.Set("meta-folder-base", dir)

	modMeta := core.ModToml{
		Name:     "NewMod",
		FileName: "newmod.jar",
		Side:     core.UniversalSide,
		Download: core.ModDownload{URL: "https://example.com/newmod.jar", HashFormat: "sha256", Hash: "deadbeef"},
	}
	destPath := modMeta.SetMetaPath(filepath.Join(dir, "mods", "newmod.pw.toml"))

	require.NoError(t, WriteModAndUpdateIndex(&packToml, &modMeta, destPath))

	assert.FileExists(t, destPath)

	// Reloading confirms the index was updated to include the new mod alongside the
	// original fixture mod ("balm"), and that pack.toml's index hash was refreshed.
	loaded, err := LoadAll(packPath)
	require.NoError(t, err)
	assert.Contains(t, loaded.Mods, "newmod")
	assert.Contains(t, loaded.Mods, "balm")
}
