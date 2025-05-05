package packwiz

import (
	"github.com/bradleyjkemp/cupaloy"
	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPackStruct(t *testing.T) {

	pack := NewPack(
		"PackA",
		"dev",
		"1.0.0",
		"a nice pack",
		"1.21.5",
		map[string]string{
			"quilt": "0.29.0-beta.6",
		},
	)

	download := core.ModDownload{
		URL:        "",
		HashFormat: "sha1",
		Hash:       "5694a7bdfd508cf23bb4f2ab2fca7d45a517def7",
		Mode:       "metadata:curseforge",
	}

	update := core.ModUpdate{
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

	pack.SetMod("balm", mod)

	packText, err := pack.AsPackToml()
	assert.NoError(t, err)

	err = cupaloy.SnapshotMulti("pack", packText)
	assert.NoError(t, err)

	indexText, hash, err := pack.AsIndexToml()
	assert.NoError(t, err)

	err = cupaloy.SnapshotMulti("index", indexText, hash)
	assert.NoError(t, err)
}
