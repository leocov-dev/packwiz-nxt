package fileio

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPackwizLocalStore(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Run("XDG_DATA_HOME set takes precedence on linux", func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", "/custom/data/home")
			got, err := GetPackwizLocalStore()
			require.NoError(t, err)
			assert.Equal(t, filepath.Join("/custom/data/home", "packwiz"), got)
		})
	}

	t.Run("falls back to os.UserConfigDir when unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		got, err := GetPackwizLocalStore()
		require.NoError(t, err)
		assert.Equal(t, "packwiz", filepath.Base(got))
	})
}

func TestGetPackwizLocalCache(t *testing.T) {
	got, err := GetPackwizLocalCache()
	require.NoError(t, err)
	assert.Equal(t, "packwiz", filepath.Base(got))
}

func TestGetPackwizInstallBinPath(t *testing.T) {
	got, err := GetPackwizInstallBinPath()
	require.NoError(t, err)
	assert.Equal(t, "bin", filepath.Base(got))
}

func TestGetPackwizInstallBinFile(t *testing.T) {
	got, err := GetPackwizInstallBinFile()
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Equal(t, "packwiz.exe", filepath.Base(got))
	} else {
		assert.Equal(t, "packwiz", filepath.Base(got))
	}
}

func TestGetPackwizCache(t *testing.T) {
	t.Run("explicit configured dir is used as-is", func(t *testing.T) {
		got, err := GetPackwizCache("/explicit/cache/dir")
		require.NoError(t, err)
		assert.Equal(t, "/explicit/cache/dir", got)
	})

	t.Run("empty configured dir falls back to local cache store", func(t *testing.T) {
		got, err := GetPackwizCache("")
		require.NoError(t, err)
		assert.Equal(t, "cache", filepath.Base(got))
	})
}
