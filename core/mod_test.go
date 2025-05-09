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
		true,
		false,
		update,
		download,
		nil,
	)

	text, hash, err := mod.AsModToml()

	assert.NoError(t, err)

	cupaloy.SnapshotT(t, text, hash)
}
