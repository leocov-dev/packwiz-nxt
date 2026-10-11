package jarmeta

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fabricJSON = `{
  "schemaVersion": 1,
  "id": "examplemod",
  "version": "1.2.3",
  "name": "Example",
  "environment": "client",
  "provides": ["example-alias"],
  "entrypoints": {"main": ["a.B"]},
  "somethingUnknown": {"x": 1},
  "depends": {
    "minecraft": [">=1.20 <1.20.5", "1.19.4"],
    "fabricloader": ">=0.15.0",
    "fabric-api": "*",
    "java": ">=17"
  },
  "recommends": {"modmenu": "*"},
  "suggests": {"sodium": ">=0.5"},
  "breaks": {"badmod": "<2.0"},
  "conflicts": {"meh": ["1.0", "1.1"]}
}`

const quiltJSON = `{
  "schema_version": 1,
  "quilt_loader": {
    "group": "org.example",
    "id": "qmod",
    "version": "0.9.0",
    "metadata": {"name": "Q Mod"},
    "provides": ["alias", {"id": "alias2", "version": "1"}],
    "depends": [
      "plain_dep",
      {"id": "minecraft", "versions": ">=1.20"},
      {"id": "opt", "versions": ["1.0", "2.0"], "optional": true},
      {"id": "quilt_loader", "versions": {"all": [">=0.20", "<0.30"]}},
      {"id": "x", "versions": {"any": ["1", "2"]}},
      [{"id": "alt1"}, {"id": "alt2"}]
    ],
    "breaks": [{"id": "evil", "versions": "*"}]
  },
  "minecraft": {"environment": "dedicated_server"}
}`

const forgeTOML = `
modLoader = "javafml"
loaderVersion = "[47,)"
license = "MIT"

[[mods]]
modId = "forgemod"
version = "${file.jarVersion}"
displayName = "Forge Mod"

[[dependencies.forgemod]]
modId = "forge"
mandatory = true
versionRange = "[47.1.0,)"
side = "BOTH"

[[dependencies.forgemod]]
modId = "minecraft"
mandatory = true
versionRange = "[1.20.1,1.21)"
side = "BOTH"

[[dependencies.forgemod]]
modId = "jei"
mandatory = false
versionRange = "[15,)"
side = "CLIENT"
`

const neoforgeTOML = `
modLoader = "javafml"
loaderVersion = "[2,)"

[[mods]]
modId = "neomod"
version = "2.0.0"

[[mods]]
modId = "neomod_api"
version = "2.0.1"

[[dependencies.neomod]]
modId = "neoforge"
type = "required"
versionRange = "[21.1,)"
side = "BOTH"

[[dependencies.neomod]]
modId = "minecraft"
type = "required"
versionRange = "[1.21,1.21.2)"

[[dependencies.neomod]]
modId = "opt"
type = "optional"
versionRange = "[1,)"
side = "SERVER"

[[dependencies.neomod]]
modId = "bad"
type = "incompatible"
versionRange = "[1,)"

[[dependencies.neomod]]
modId = "meh"
type = "discouraged"
`

type entry struct {
	name string
	data string
}

func buildJar(t *testing.T, entries ...entry) *bytes.Reader {
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
	return bytes.NewReader(buf.Bytes())
}

func parseJar(t *testing.T, entries ...entry) ([]LoaderMeta, error) {
	t.Helper()
	r := buildJar(t, entries...)
	return Parse(r, r.Size())
}

func findDep(t *testing.T, m LoaderMeta, id string, kind DependencyKind) Dependency {
	t.Helper()
	d, ok := m.FindDependency(id, kind)
	require.Truef(t, ok, "dependency %s/%s not found", id, kind)
	return d
}

func TestParseFabric(t *testing.T) {
	metas, err := parseJar(t, entry{"fabric.mod.json", fabricJSON}, entry{"a/B.class", "x"})
	require.NoError(t, err)
	require.Len(t, metas, 1)
	m := metas[0]

	assert.Equal(t, LoaderFabric, m.Loader)
	assert.Equal(t, "examplemod", m.ModID)
	assert.Equal(t, "1.2.3", m.Version)
	assert.False(t, m.VersionIsPlaceholder)
	assert.Equal(t, EnvClient, m.Environment)
	assert.Equal(t, []string{"example-alias"}, m.Provides)
	assert.Equal(t, []Mod{{ID: "examplemod", Version: "1.2.3"}}, m.Mods)

	mc := findDep(t, m, "minecraft", KindDepends)
	assert.Equal(t, []string{">=1.20 <1.20.5", "1.19.4"}, mc.VersionRanges)
	assert.True(t, mc.Mandatory)
	assert.Equal(t, "examplemod", mc.Owner)
	ranges, ok := m.MinecraftRanges()
	assert.True(t, ok)
	assert.Equal(t, mc.VersionRanges, ranges)

	ld, ok := m.LoaderDependency()
	require.True(t, ok)
	assert.Equal(t, []string{">=0.15.0"}, ld.VersionRanges)

	assert.Equal(t, []string{"*"}, findDep(t, m, "fabric-api", KindDepends).VersionRanges)
	assert.Equal(t, []string{">=17"}, findDep(t, m, "java", KindDepends).VersionRanges)
	assert.False(t, findDep(t, m, "modmenu", KindRecommends).Mandatory)
	findDep(t, m, "sodium", KindSuggests)
	findDep(t, m, "badmod", KindBreaks)
	assert.Equal(t, []string{"1.0", "1.1"}, findDep(t, m, "meh", KindConflicts).VersionRanges)
}

func TestParseQuilt(t *testing.T) {
	metas, err := parseJar(t, entry{"quilt.mod.json", quiltJSON})
	require.NoError(t, err)
	require.Len(t, metas, 1)
	m := metas[0]

	assert.Equal(t, LoaderQuilt, m.Loader)
	assert.Equal(t, "qmod", m.ModID)
	assert.Equal(t, "Q Mod", m.Name)
	assert.Equal(t, EnvServer, m.Environment)
	assert.Equal(t, []string{"alias", "alias2"}, m.Provides)

	plain := findDep(t, m, "plain_dep", KindDepends)
	assert.Empty(t, plain.VersionRanges)
	assert.True(t, plain.Mandatory)

	opt := findDep(t, m, "opt", KindDepends)
	assert.False(t, opt.Mandatory)
	assert.Equal(t, []string{"1.0", "2.0"}, opt.VersionRanges)

	ql := findDep(t, m, "quilt_loader", KindDepends)
	assert.Equal(t, []string{">=0.20", "<0.30"}, ql.VersionRangesAll)
	assert.Empty(t, ql.VersionRanges)
	got, ok := m.LoaderDependency()
	assert.True(t, ok)
	assert.Equal(t, ql, got)

	assert.Equal(t, []string{"1", "2"}, findDep(t, m, "x", KindDepends).VersionRanges)
	findDep(t, m, "evil", KindBreaks)
	_, ok = m.FindDependency("alt1", KindDepends)
	assert.False(t, ok, "alternative groups are not matched by FindDependency")

	var group Dependency
	for _, d := range m.Dependencies {
		if len(d.AnyOf) > 0 {
			group = d
		}
	}
	require.Len(t, group.AnyOf, 2)
	assert.Empty(t, group.ID)
	assert.Equal(t, KindDepends, group.Kind)
	assert.Equal(t, "qmod", group.Owner)
	assert.True(t, group.Mandatory)
	assert.Equal(t, "alt1", group.AnyOf[0].ID)
	assert.Equal(t, "alt2", group.AnyOf[1].ID)
	assert.Equal(t, KindDepends, group.AnyOf[1].Kind)
}

func TestParseQuiltBreaksGroupAndEmptyGroup(t *testing.T) {
	metas, err := parseJar(t, entry{"quilt.mod.json",
		`{"quilt_loader":{"id":"q","depends":[[]],"breaks":[["a",{"id":"b","versions":"<1"}]]}}`})
	require.NoError(t, err)
	m := metas[0]
	require.Len(t, m.Dependencies, 1, "empty group dropped")
	g := m.Dependencies[0]
	assert.Equal(t, KindBreaks, g.Kind)
	require.Len(t, g.AnyOf, 2)
	assert.Equal(t, "a", g.AnyOf[0].ID)
	assert.Equal(t, []string{"<1"}, g.AnyOf[1].VersionRanges)
}

func TestParseForge(t *testing.T) {
	metas, err := parseJar(t, entry{"META-INF/mods.toml", forgeTOML})
	require.NoError(t, err)
	require.Len(t, metas, 1)
	m := metas[0]

	assert.Equal(t, LoaderForge, m.Loader)
	assert.Equal(t, "forgemod", m.ModID)
	assert.Equal(t, "${file.jarVersion}", m.Version)
	assert.True(t, m.VersionIsPlaceholder)
	assert.True(t, m.Mods[0].VersionIsPlaceholder)
	assert.Equal(t, EnvBoth, m.Environment)
	assert.Equal(t, "javafml", m.ModLoader)
	assert.Equal(t, "[47,)", m.LoaderVersionRange)

	mc, ok := m.MinecraftDependency()
	require.True(t, ok)
	assert.Equal(t, []string{"[1.20.1,1.21)"}, mc.VersionRanges)
	assert.True(t, mc.Mandatory)
	assert.Equal(t, "forgemod", mc.Owner)

	ld, ok := m.LoaderDependency()
	require.True(t, ok)
	assert.Equal(t, "forge", ld.ID)

	jei := findDep(t, m, "jei", KindDepends)
	assert.False(t, jei.Mandatory)
	assert.Equal(t, SideClient, jei.Side)
}

func TestParseNeoForge(t *testing.T) {
	metas, err := parseJar(t, entry{"META-INF/neoforge.mods.toml", neoforgeTOML})
	require.NoError(t, err)
	require.Len(t, metas, 1)
	m := metas[0]

	assert.Equal(t, LoaderNeoForge, m.Loader)
	assert.Equal(t, "neomod", m.ModID)
	require.Len(t, m.Mods, 2)
	assert.Equal(t, Mod{ID: "neomod_api", Version: "2.0.1"}, m.Mods[1])

	ld, ok := m.LoaderDependency()
	require.True(t, ok)
	assert.Equal(t, "neoforge", ld.ID)
	assert.True(t, ld.Mandatory)

	opt := findDep(t, m, "opt", KindDepends)
	assert.False(t, opt.Mandatory)
	assert.Equal(t, SideServer, opt.Side)
	findDep(t, m, "bad", KindBreaks)
	meh := findDep(t, m, "meh", KindConflicts)
	assert.Empty(t, meh.VersionRanges)
	assert.Equal(t, SideBoth, meh.Side)
}

func TestParseMultiLoader(t *testing.T) {
	metas, err := parseJar(t,
		entry{"META-INF/neoforge.mods.toml", neoforgeTOML},
		entry{"fabric.mod.json", fabricJSON},
	)
	require.NoError(t, err)
	require.Len(t, metas, 2)
	loaders := []Loader{metas[0].Loader, metas[1].Loader}
	assert.ElementsMatch(t, []Loader{LoaderFabric, LoaderNeoForge}, loaders)
}

func TestParseMultiLoaderPartialFailure(t *testing.T) {
	metas, err := parseJar(t,
		entry{"fabric.mod.json", fabricJSON},
		entry{"quilt.mod.json", "{not json"},
		entry{"META-INF/mods.toml", "[[mods"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformed)
	assert.False(t, errors.Is(err, ErrNoMetadata))
	assert.Contains(t, err.Error(), "quilt.mod.json")
	assert.Contains(t, err.Error(), "META-INF/mods.toml")
	require.Len(t, metas, 1)
	assert.Equal(t, LoaderFabric, metas[0].Loader)
}

func TestParseForgeTomlWithNeoForgeDependency(t *testing.T) {
	const neo = "[[mods]]\nmodId = \"m\"\n[[dependencies.m]]\nmodId = \"neoforge\"\nmandatory = true\nversionRange = \"[47,)\"\n"
	const both = neo + "[[dependencies.m]]\nmodId = \"forge\"\nmandatory = true\n"

	metas, err := parseJar(t, entry{"META-INF/mods.toml", neo})
	require.NoError(t, err)
	assert.Equal(t, LoaderNeoForge, metas[0].Loader)
	ld, ok := metas[0].LoaderDependency()
	require.True(t, ok)
	assert.Equal(t, "neoforge", ld.ID)

	metas, err = parseJar(t, entry{"META-INF/mods.toml", both})
	require.NoError(t, err)
	assert.Equal(t, LoaderForge, metas[0].Loader)

	metas, err = parseJar(t, entry{"META-INF/mods.toml", forgeTOML})
	require.NoError(t, err)
	assert.Equal(t, LoaderForge, metas[0].Loader)
}

func TestParseNoMetadata(t *testing.T) {
	tests := []struct {
		name    string
		entries []entry
	}{
		{"empty jar", nil},
		{"unrelated files", []entry{{"META-INF/MANIFEST.MF", "x"}}},
		{"nested fabric.mod.json", []entry{{"sub/fabric.mod.json", fabricJSON}}},
		{"nested mods.toml", []entry{{"foo/META-INF/mods.toml", forgeTOML}}},
		{"wrong case", []entry{{"Fabric.mod.json", fabricJSON}}},
		{"quilt in subdir", []entry{{"META-INF/quilt.mod.json", quiltJSON}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseJar(t, tc.entries...)
			assert.ErrorIs(t, err, ErrNoMetadata)
		})
	}
}

func TestParseMalformed(t *testing.T) {
	tests := []struct {
		name  string
		entry entry
	}{
		{"bad json", entry{"fabric.mod.json", "{not json"}},
		{"fabric no id", entry{"fabric.mod.json", `{"version":"1"}`}},
		{"fabric bad depends type", entry{"fabric.mod.json", `{"id":"a","depends":{"x":5}}`}},
		{"quilt no id", entry{"quilt.mod.json", `{"quilt_loader":{"version":"1"}}`}},
		{"bad toml", entry{"META-INF/mods.toml", "[[mods"}},
		{"forge no mods", entry{"META-INF/mods.toml", `modLoader = "javafml"`}},
		{"forge mod without id", entry{"META-INF/neoforge.mods.toml", "[[mods]]\nversion = \"1\"\n"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseJar(t, tc.entry)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrMalformed)
			assert.False(t, errors.Is(err, ErrNoMetadata))
		})
	}
}

func TestParseNotAZip(t *testing.T) {
	data := []byte("definitely not a zip file")
	_, err := Parse(bytes.NewReader(data), int64(len(data)))
	assert.ErrorIs(t, err, ErrMalformed)
}

func TestParseEntryTooLarge(t *testing.T) {
	big := `{"id":"a","pad":"` + strings.Repeat("x", MaxEntrySize) + `"}`
	_, err := parseJar(t, entry{"fabric.mod.json", big})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEntryTooLarge)
	assert.ErrorIs(t, err, ErrMalformed)

	// exactly at the cap is fine
	pad := MaxEntrySize - len(`{"id":"a","pad":""}`)
	ok := `{"id":"a","pad":"` + strings.Repeat("x", pad) + `"}`
	require.Len(t, ok, MaxEntrySize)
	_, err = parseJar(t, entry{"fabric.mod.json", ok})
	assert.NoError(t, err)
}

func TestReadCappedEnforcesLimit(t *testing.T) {
	// The reader yields more than MaxEntrySize regardless of any declared size.
	_, err := readCapped(strings.NewReader(strings.Repeat("x", MaxEntrySize+1)))
	assert.ErrorIs(t, err, ErrEntryTooLarge)

	data, err := readCapped(strings.NewReader(strings.Repeat("x", MaxEntrySize)))
	require.NoError(t, err)
	assert.Len(t, data, MaxEntrySize)
}

// The standard library already rejects a deflate entry whose header lies about
// its size, so such a jar must fail as malformed before reaching the limiter.
func TestParseEntryDeclaredSizeLiesDeflate(t *testing.T) {
	payload := `{"id":"a","pad":"` + strings.Repeat("x", MaxEntrySize+100) + `"}`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestSpeed)
	})
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "fabric.mod.json", Method: zip.Deflate})
	require.NoError(t, err)
	_, err = w.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	patched := patchUncompressedSize(t, buf.Bytes(), 10)
	zr, err := zip.NewReader(bytes.NewReader(patched), int64(len(patched)))
	require.NoError(t, err)
	require.Equal(t, uint64(10), zr.File[0].UncompressedSize64)

	_, err = Parse(bytes.NewReader(patched), int64(len(patched)))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformed)
}

func TestParseUnknownFieldsIgnored(t *testing.T) {
	metas, err := parseJar(t,
		entry{"fabric.mod.json", `{"id":"a","version":"1","weird":[1,2,{"q":null}]}`},
		entry{"META-INF/mods.toml", "futureKey = 1\n[[mods]]\nmodId = \"b\"\nnewThing = \"x\"\n"},
	)
	require.NoError(t, err)
	require.Len(t, metas, 2)
}

func TestPlaceholderAndMissingOptional(t *testing.T) {
	metas, err := parseJar(t, entry{"fabric.mod.json", `{"id":"a","version":"${version}"}`})
	require.NoError(t, err)
	m := metas[0]
	assert.True(t, m.VersionIsPlaceholder)
	assert.Equal(t, "${version}", m.Version)
	assert.Empty(t, m.Dependencies)
	assert.Equal(t, EnvBoth, m.Environment)
	_, ok := m.MinecraftRanges()
	assert.False(t, ok)
	_, ok = m.LoaderDependency()
	assert.False(t, ok)
}

// patchUncompressedSize rewrites the uncompressed-size field of the first
// central directory header so the declared size disagrees with the data.
func patchUncompressedSize(t *testing.T, raw []byte, size uint32) []byte {
	t.Helper()
	out := append([]byte(nil), raw...)
	sig := []byte{0x50, 0x4b, 0x01, 0x02}
	i := bytes.Index(out, sig)
	require.GreaterOrEqual(t, i, 0)
	binary.LittleEndian.PutUint32(out[i+24:], size)
	return out
}
