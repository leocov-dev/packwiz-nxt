package core

import (
	"github.com/bradleyjkemp/cupaloy"
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
			false, false,
			ModUpdate{"curseforge": map[string]interface{}{"file-id": 6459015, "project-id": 531761}},
			ModDownload{HashFormat: "sha1", Hash: "5694a7bdfd508cf23bb4f2ab2fca7d45a517def7", Mode: "metadata:curseforge"},
			nil,
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

	idx, err := newMod("21.5.14+fabric").toIndexEntry()
	assert.NoError(t, err)
	assert.Equal(t, baseHash, idx.Hash)
}
