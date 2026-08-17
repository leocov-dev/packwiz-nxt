package sources

import (
	"testing"
	"time"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func strPtr(s string) *string { return &s }

func TestModrinthUrlParsing(t *testing.T) {
	t.Run("ParseModrinthSlugOrUrl", func(t *testing.T) {
		t.Run("web URL with mod category", func(t *testing.T) {
			var slug, version, versionID, filename string
			parsed, err := ParseModrinthSlugOrUrl("https://modrinth.com/mod/jei", &slug, &version, &versionID, &filename)
			require.NoError(t, err)
			assert.False(t, parsed)
			assert.Equal(t, "jei", slug)
			assert.Equal(t, "", version)
		})

		t.Run("web URL with version and www prefix", func(t *testing.T) {
			var slug, version, versionID, filename string
			parsed, err := ParseModrinthSlugOrUrl("https://www.modrinth.com/mod/jei/version/1.2.3", &slug, &version, &versionID, &filename)
			require.NoError(t, err)
			assert.False(t, parsed)
			assert.Equal(t, "jei", slug)
			assert.Equal(t, "1.2.3", version)
		})

		t.Run("unknown category segment errors", func(t *testing.T) {
			var slug, version, versionID, filename string
			_, err := ParseModrinthSlugOrUrl("https://modrinth.com/widget/jei", &slug, &version, &versionID, &filename)
			assert.Error(t, err)
		})

		t.Run("CDN URL with escaped filename", func(t *testing.T) {
			var slug, version, versionID, filename string
			parsed, err := ParseModrinthSlugOrUrl("https://cdn.modrinth.com/data/AANobbMI/versions/abc123/jei%201.20.jar", &slug, &version, &versionID, &filename)
			require.NoError(t, err)
			assert.False(t, parsed)
			assert.Equal(t, "AANobbMI", slug)
			assert.Equal(t, "abc123", versionID)
			assert.Equal(t, "jei 1.20.jar", filename)
		})

		t.Run("bare slug sets parsedSlug true", func(t *testing.T) {
			var slug, version, versionID, filename string
			parsed, err := ParseModrinthSlugOrUrl("jei", &slug, &version, &versionID, &filename)
			require.NoError(t, err)
			assert.True(t, parsed)
			assert.Equal(t, "jei", slug)
		})

		t.Run("non-matching input leaves out-params untouched", func(t *testing.T) {
			var slug, version, versionID, filename string
			parsed, err := ParseModrinthSlugOrUrl("", &slug, &version, &versionID, &filename)
			require.NoError(t, err)
			assert.False(t, parsed)
			assert.Equal(t, "", slug)
		})
	})

	t.Run("ParseAsModrinthSlug", func(t *testing.T) {
		assert.Equal(t, "jei", ParseAsModrinthSlug("https://modrinth.com/mod/jei"))
		assert.Equal(t, "jei", ParseAsModrinthSlug("jei"))
		assert.Equal(t, "", ParseAsModrinthSlug("https://modrinth.com/widget/jei"))
	})

	t.Run("ParseAsModrinthVersion", func(t *testing.T) {
		assert.Equal(t, "1.2.3", ParseAsModrinthVersion("https://modrinth.com/mod/jei/version/1.2.3"))
		assert.Equal(t, "", ParseAsModrinthVersion("jei"))
	})

	t.Run("ParseAsModrinthVersionID", func(t *testing.T) {
		assert.Equal(t, "abc123", ParseAsModrinthVersionID("https://cdn.modrinth.com/data/AANobbMI/versions/abc123/jei.jar"))
		assert.Equal(t, "", ParseAsModrinthVersionID("jei"))
	})

	t.Run("ParseAsModrinthFilename", func(t *testing.T) {
		assert.Equal(t, "jei 1.20.jar", ParseAsModrinthFilename("https://cdn.modrinth.com/data/AANobbMI/versions/abc123/jei%201.20.jar"))
		assert.Equal(t, "", ParseAsModrinthFilename("jei"))
	})
}

func TestMrCompareLoaderLists(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want int32
	}{
		{"identical loader lists tie", []string{"fabric"}, []string{"fabric"}, 0},
		{"a has a more-preferred loader", []string{"quilt"}, []string{"fabric"}, -1},
		{"b has a more-preferred loader", []string{"fabric"}, []string{"quilt"}, 1},
		{"compat group excludes quilt when both share fabric", []string{"fabric", "quilt"}, []string{"fabric"}, 0},
		{"loaders absent from preference list are ignored", []string{"unknown-loader"}, []string{"unknown-loader"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mrCompareLoaderLists(tt.a, tt.b))
		})
	}
}

func TestMrFindLatestVersion(t *testing.T) {
	mkVersion := func(number string, gameVersions []string, loaders []string, published time.Time) *modrinthApi.Version {
		return &modrinthApi.Version{
			VersionNumber: strPtr(number),
			GameVersions:  gameVersions,
			Loaders:       loaders,
			DatePublished: &published,
		}
	}

	base := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("single element slice returned unchanged", func(t *testing.T) {
		v := mkVersion("1.0.0", []string{"1.20"}, []string{"fabric"}, base)
		got := mrFindLatestVersion([]*modrinthApi.Version{v}, []string{"1.20"}, true)
		assert.Same(t, v, got)
	})

	t.Run("useFlexVer true: higher version number wins over an earlier date", func(t *testing.T) {
		older := mkVersion("2.0.0", []string{"1.20"}, []string{"fabric"}, base)
		newer := mkVersion("1.0.0", []string{"1.20"}, []string{"fabric"}, base.Add(24*time.Hour))
		got := mrFindLatestVersion([]*modrinthApi.Version{older, newer}, []string{"1.20"}, true)
		assert.Same(t, older, got)
	})

	t.Run("useFlexVer false: later DatePublished wins regardless of version number", func(t *testing.T) {
		earlierPublished := mkVersion("2.0.0", []string{"1.20"}, []string{"fabric"}, base)
		laterPublished := mkVersion("1.0.0", []string{"1.20"}, []string{"fabric"}, base.Add(24*time.Hour))
		got := mrFindLatestVersion([]*modrinthApi.Version{earlierPublished, laterPublished}, []string{"1.20"}, false)
		assert.Same(t, laterPublished, got)
	})

	t.Run("higher-index (later specified) game version preferred on tie", func(t *testing.T) {
		matchesFirst := mkVersion("1.0.0", []string{"1.19"}, []string{"fabric"}, base)
		matchesSecond := mkVersion("1.0.0", []string{"1.20"}, []string{"fabric"}, base)
		got := mrFindLatestVersion([]*modrinthApi.Version{matchesFirst, matchesSecond}, []string{"1.19", "1.20"}, false)
		assert.Same(t, matchesSecond, got)
	})
}

func TestMrGetProjectTypeFolder(t *testing.T) {
	t.Run("modpack errors", func(t *testing.T) {
		_, err := mrGetProjectTypeFolder("modpack", nil, nil)
		assert.Error(t, err)
	})

	t.Run("resourcepack", func(t *testing.T) {
		folder, err := mrGetProjectTypeFolder("resourcepack", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "resourcepacks", folder)
	})

	t.Run("shader picks best loader folder from fileLoaders alone", func(t *testing.T) {
		folder, err := mrGetProjectTypeFolder("shader", []string{"optifine", "iris"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "shaderpacks", folder)
	})

	t.Run("shader falls back to shaderpacks when no known loader", func(t *testing.T) {
		folder, err := mrGetProjectTypeFolder("shader", []string{"unknown"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "shaderpacks", folder)
	})

	t.Run("mod requires loader present in both fileLoaders and packLoaders", func(t *testing.T) {
		folder, err := mrGetProjectTypeFolder("mod", []string{"quilt", "fabric"}, []string{"fabric"})
		require.NoError(t, err)
		assert.Equal(t, "mods", folder)
	})

	t.Run("mod with no shared loader falls back to mods", func(t *testing.T) {
		folder, err := mrGetProjectTypeFolder("mod", []string{"forge"}, []string{"fabric"})
		require.NoError(t, err)
		assert.Equal(t, "mods", folder)
	})

	t.Run("mod datapack loader errors", func(t *testing.T) {
		_, err := mrGetProjectTypeFolder("mod", []string{"datapack"}, []string{"fabric"})
		assert.Error(t, err)
	})

	t.Run("unknown project type errors", func(t *testing.T) {
		_, err := mrGetProjectTypeFolder("plugin-but-not-really", nil, nil)
		assert.Error(t, err)
	})
}

func TestMrGetSide(t *testing.T) {
	tests := []struct {
		name   string
		server string
		client string
		want   core.ModSide
	}{
		{"both required is universal", "required", "required", core.UniversalSide},
		{"server required, client unsupported is server-only", "required", "unsupported", core.ServerSide},
		{"client optional, server unsupported is client-only", "unsupported", "optional", core.ClientSide},
		{"both unsupported is empty", "unsupported", "unsupported", core.EmptySide},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &modrinthApi.Project{
				ServerSide: strPtr(tt.server),
				ClientSide: strPtr(tt.client),
			}
			assert.Equal(t, tt.want, mrGetSide(project))
		})
	}
}

func TestMrGetBestHash(t *testing.T) {
	tests := []struct {
		name          string
		hashes        map[string]string
		wantAlgorithm string
		wantHash      string
	}{
		{"prefers sha512 over all others", map[string]string{"sha512": "h512", "sha256": "h256", "sha1": "h1"}, "sha512", "h512"},
		{"prefers sha256 over sha1/murmur2", map[string]string{"sha256": "h256", "sha1": "h1", "murmur2": "hm2"}, "sha256", "h256"},
		{"prefers sha1 over murmur2", map[string]string{"sha1": "h1", "murmur2": "hm2"}, "sha1", "h1"},
		{"falls back to murmur2", map[string]string{"murmur2": "hm2"}, "murmur2", "hm2"},
		{"empty map returns empty strings", map[string]string{}, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &modrinthApi.File{Hashes: tt.hashes}
			algorithm, hash := mrGetBestHash(file)
			assert.Equal(t, tt.wantAlgorithm, algorithm)
			assert.Equal(t, tt.wantHash, hash)
		})
	}

	t.Run("only unrecognized hash present still returns it as a fallback", func(t *testing.T) {
		file := &modrinthApi.File{Hashes: map[string]string{"crc32": "abc"}}
		algorithm, hash := mrGetBestHash(file)
		assert.Equal(t, "crc32", algorithm)
		assert.Equal(t, "abc", hash)
	})
}

func TestMrGetInstalledProjectIDs(t *testing.T) {
	newModrinthMod := func(projectID string) *core.Mod {
		update := make(core.ModUpdate)
		update["modrinth"] = map[string]interface{}{"mod-id": projectID}
		return core.NewMod("slug", "name", "file.jar", core.UniversalSide, "mods", "", false, false, update, core.ModDownload{}, nil)
	}

	t.Run("collects installed project IDs from modrinth-sourced mods", func(t *testing.T) {
		mods := []*core.Mod{newModrinthMod("AANobbMI"), newModrinthMod("P7dR8mSH")}
		got := mrGetInstalledProjectIDs(mods)
		assert.Equal(t, []string{"AANobbMI", "P7dR8mSH"}, got)
	})

	t.Run("skips mods without modrinth source data", func(t *testing.T) {
		update := make(core.ModUpdate)
		update["curseforge"] = map[string]interface{}{"project-id": 12345}
		cfMod := core.NewMod("slug", "name", "file.jar", core.UniversalSide, "mods", "", false, false, update, core.ModDownload{}, nil)

		got := mrGetInstalledProjectIDs([]*core.Mod{cfMod})
		assert.Empty(t, got)
	})

	t.Run("skips mods with an empty project ID", func(t *testing.T) {
		got := mrGetInstalledProjectIDs([]*core.Mod{newModrinthMod("")})
		assert.Empty(t, got)
	})
}
