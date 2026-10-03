package fileio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const remotePackToml = `name = "Remote"
author = "someone"
version = "1.2.3"
description = "desc"
pack-format = "packwiz:1.1.0"

[index]
file = "index.toml"
hash-format = "sha256"

[versions]
minecraft = "1.20.1"
fabric = "0.15.0"
`

const remoteIndexToml = `hash-format = "sha256"

[[files]]
file = "mods/sodium.pw.toml"
hash = "00"
metafile = true
preserve = true

[[files]]
file = "mods/gone.pw.toml"
metafile = true

[[files]]
file = "config/foo.json"
hash = "11"

[[files]]
file = "mods/bad.pw.toml"
metafile = true
`

const remoteSodiumToml = `name = "Sodium"
filename = "sodium.jar"
side = "client"

[download]
url = "https://example.com/sodium.jar"
hash-format = "sha1"
hash = "abc"

[update.modrinth]
mod-id = "AANobbMI"
version = "xyz"
`

func newRemoteServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	serve := func(p, body string) {
		mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	serve("/p/pack.toml", remotePackToml)
	serve("/p/index.toml", remoteIndexToml)
	serve("/p/mods/sodium.pw.toml", remoteSodiumToml)
	serve("/p/mods/bad.pw.toml", "not = [toml")
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestLoadRemotePack(t *testing.T) {
	srv := newRemoteServer(t)

	rp, err := LoadRemotePack(context.Background(), srv.URL+"/p/pack.toml", RemoteLoadOptions{})
	require.NoError(t, err)

	assert.Equal(t, "Remote", rp.Pack.Name)
	assert.Equal(t, "1.20.1", rp.Pack.Versions["minecraft"])
	require.Len(t, rp.Pack.Mods, 1, "%v", rp.Warnings)

	mod := rp.Pack.Mods["sodium"]
	require.NotNil(t, mod)
	assert.Equal(t, "Sodium", mod.Name)
	assert.Equal(t, "mods", mod.ModType)
	assert.True(t, mod.Preserve)
	assert.Equal(t, "sha256", mod.HashFormat)
	assert.Contains(t, mod.Update, "modrinth")

	assert.Equal(t, []string{"config/foo.json"}, rp.SkippedFiles)
	// gone (404), bad (parse), plus hash mismatch note for sodium
	assert.Len(t, rp.Warnings, 3)
}

func TestLoadRemotePack_Errors(t *testing.T) {
	srv := newRemoteServer(t)

	_, err := LoadRemotePack(context.Background(), "ftp://example.com/pack.toml", RemoteLoadOptions{})
	assert.Error(t, err)

	_, err = LoadRemotePack(context.Background(), srv.URL+"/missing/pack.toml", RemoteLoadOptions{})
	assert.Error(t, err)

	_, err = LoadRemotePack(context.Background(), srv.URL+"/p/pack.toml", RemoteLoadOptions{MaxFileSize: 10})
	assert.Error(t, err)
}

func TestLoadRemotePack_RejectsOtherHost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pack.toml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("name = \"x\"\n[index]\nfile = \"http://elsewhere.invalid/index.toml\"\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := LoadRemotePack(context.Background(), srv.URL+"/pack.toml", RemoteLoadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same host")
}

func TestUniqueSlug(t *testing.T) {
	mk := func(p string) *core.ModToml {
		m := &core.ModToml{}
		m.SetMetaPath(p)
		return m
	}
	taken := map[string]core.IndexFile{"foo": {}}

	assert.False(t, uniqueSlug(mk("mods/bar.pw.toml"), taken))

	m := mk("resourcepacks/foo.pw.toml")
	assert.True(t, uniqueSlug(m, taken))
	assert.Equal(t, "foo-resourcepacks", m.GetSlug())

	taken["foo-resourcepacks"] = core.IndexFile{}
	m = mk("resourcepacks/foo.pw.toml")
	assert.True(t, uniqueSlug(m, taken))
	assert.Equal(t, "foo-resourcepacks-2", m.GetSlug())
}
