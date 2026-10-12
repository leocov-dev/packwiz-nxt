package jarmeta

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const libTOML = `
modLoader = "javafml"
loaderVersion = "[2,)"

[[mods]]
modId = "xaerolib"
version = "1.2.0"
`

func zipBytes(t *testing.T, entries ...entry) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		require.NoError(t, err)
		_, err = w.Write([]byte(e.data))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.String()
}

func TestParseJarJar(t *testing.T) {
	lib := zipBytes(t, entry{"META-INF/neoforge.mods.toml", libTOML})
	metas, err := parseJar(t,
		entry{"META-INF/neoforge.mods.toml", neoforgeTOML},
		entry{jarjarPath, `{"jars":[{"path":"META-INF/jarjar/lib.jar"},{"path":"META-INF/jarjar/gone.jar"}]}`},
		entry{"META-INF/jarjar/lib.jar", lib},
	)
	require.NoError(t, err)
	require.Len(t, metas, 1)
	m := metas[0]
	assert.Equal(t, []string{"META-INF/jarjar/lib.jar", "META-INF/jarjar/gone.jar"}, m.NestedJars)
	assert.Equal(t, 1, m.NestedOpened)
	assert.Equal(t, []Mod{{ID: "xaerolib", Version: "1.2.0"}}, m.Bundled)
}

func TestParseJarJarNotRecursive(t *testing.T) {
	inner := zipBytes(t, entry{"META-INF/neoforge.mods.toml", libTOML})
	middle := zipBytes(t,
		entry{"META-INF/neoforge.mods.toml", neoforgeTOML},
		entry{jarjarPath, `{"jars":[{"path":"META-INF/jarjar/inner.jar"}]}`},
		entry{"META-INF/jarjar/inner.jar", inner},
	)
	metas, err := parseJar(t,
		entry{"META-INF/neoforge.mods.toml", neoforgeTOML},
		entry{jarjarPath, `{"jars":[{"path":"META-INF/jarjar/middle.jar"}]}`},
		entry{"META-INF/jarjar/middle.jar", middle},
	)
	require.NoError(t, err)
	var ids []string
	for _, mod := range metas[0].Bundled {
		ids = append(ids, mod.ID)
	}
	assert.Equal(t, []string{"neomod", "neomod_api"}, ids)
}

func TestParseJarJarUnreadableNested(t *testing.T) {
	metas, err := parseJar(t,
		entry{"META-INF/neoforge.mods.toml", neoforgeTOML},
		entry{jarjarPath, `{"jars":[{"path":"META-INF/jarjar/bad.jar"}]}`},
		entry{"META-INF/jarjar/bad.jar", "not a zip"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"META-INF/jarjar/bad.jar"}, metas[0].NestedJars)
	assert.Zero(t, metas[0].NestedOpened)
	assert.Empty(t, metas[0].Bundled)
}

func TestParseJarJarMalformed(t *testing.T) {
	metas, err := parseJar(t,
		entry{"META-INF/neoforge.mods.toml", neoforgeTOML},
		entry{jarjarPath, `{nope`},
	)
	require.ErrorIs(t, err, ErrMalformed)
	require.Len(t, metas, 1)
}

func TestParseFabricNestedOpened(t *testing.T) {
	lib := zipBytes(t, entry{"fabric.mod.json", `{"id":"nestedlib","version":"3"}`})
	metas, err := parseJar(t,
		entry{"fabric.mod.json", `{"id":"outer","version":"1","jars":[{"file":"META-INF/jars/lib.jar"}]}`},
		entry{"META-INF/jars/lib.jar", lib},
	)
	require.NoError(t, err)
	assert.Equal(t, []Mod{{ID: "nestedlib", Version: "3"}}, metas[0].Bundled)
	assert.Equal(t, 1, metas[0].NestedOpened)
}
