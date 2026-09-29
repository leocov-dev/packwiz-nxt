package core

import (
	"github.com/bradleyjkemp/cupaloy"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestModStruct(t *testing.T) {
	download := ModDownload{
		URL:        "",
		HashFormat: "sha1",
		Hash:       "5694a7bdfd508cf23bb4f2ab2fca7d45a517def7",
		Mode:       "metadata:curseforge",
	}

	update := ModUpdate{
		"curseforge": map[string]interface{}{
			"file-id":    6459015,
			"project-id": 531761,
		},
	}

	mod := NewMod(
		"balm",
		"Balm",
		"balm-fabric-1.21.5-21.5.14.jar",
		"both",
		"mods",
		"",
		false,
		false,
		update,
		download,
		nil,
	)

	text, hash, err := mod.AsModToml()

	assert.NoError(t, err)

	cupaloy.SnapshotT(t, text, hash)
}

// TestModVersionNotSerialized guards that the in-memory Version field never
// leaks into the marshalled .pw.toml or the index hash.
func TestModVersionNotSerialized(t *testing.T) {
	newMod := func(version string) *Mod {
		m := NewMod(
			"balm", "Balm", "balm-fabric-1.21.5-21.5.14.jar", "both", "mods", "",
			true, false,
			ModUpdate{
				"curseforge": map[string]interface{}{"file-id": 6459015, "project-id": 531761},
				"modrinth":   map[string]interface{}{"mod-id": "abc", "version": "def"},
			},
			ModDownload{HashFormat: "sha1", Hash: "5694a7bdfd508cf23bb4f2ab2fca7d45a517def7", Mode: "metadata:curseforge"},
			&ModOption{Optional: true, Description: "an option", Default: true},
		)
		m.Version = version
		return m
	}

	baseText, baseHash, err := newMod("").AsModToml()
	assert.NoError(t, err)

	text, hash, err := newMod("21.5.14+fabric").AsModToml()
	assert.NoError(t, err)

	assert.Equal(t, baseText, text)
	assert.Equal(t, baseHash, hash)
	assert.NotContains(t, text, "21.5.14+fabric")
	assert.Contains(t, text, "pin = true")
	assert.Contains(t, text, "an option")

	idx, err := newMod("21.5.14+fabric").toIndexEntry()
	assert.NoError(t, err)
	assert.Equal(t, baseHash, idx.Hash)

	// Round trip: marshal -> unmarshal -> FromModMeta -> set Version -> re-marshal.
	var meta ModToml
	assert.NoError(t, toml.Unmarshal([]byte(baseText), &meta))
	loaded := FromModMeta(meta)
	assert.Empty(t, loaded.Version)
	loaded.Slug = "balm"
	loaded.ModType = "mods"
	loaded.Version = "21.5.14+fabric"

	text2, hash2, err := loaded.AsModToml()
	assert.NoError(t, err)
	assert.Equal(t, baseText, text2)
	assert.Equal(t, baseHash, hash2)
}
