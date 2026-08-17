package sources

import (
	"net/http"
	"testing"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func TestGetModrinthVersionPrimaryFile(t *testing.T) {
	t.Run("no primary marked, no filename match: first file returned", func(t *testing.T) {
		// Primary is a *bool with no omitempty exemption in real API responses (every file
		// always has it set), but GetModrinthVersionPrimaryFile dereferences it unconditionally
		// (mr-ops.go:180) - a nil Primary here would panic, so this fixture mirrors real data.
		first := &modrinthApi.File{Filename: strPtr("a.jar"), Primary: boolPtrMr(false)}
		second := &modrinthApi.File{Filename: strPtr("b.jar"), Primary: boolPtrMr(false)}
		version := &modrinthApi.Version{Files: []*modrinthApi.File{first, second}}

		got := GetModrinthVersionPrimaryFile(version, "")
		assert.Same(t, first, got)
	})

	t.Run("file marked primary is returned even if not first", func(t *testing.T) {
		first := &modrinthApi.File{Filename: strPtr("a.jar"), Primary: boolPtrMr(false)}
		primary := &modrinthApi.File{Filename: strPtr("b.jar"), Primary: boolPtrMr(true)}
		version := &modrinthApi.Version{Files: []*modrinthApi.File{first, primary}}

		got := GetModrinthVersionPrimaryFile(version, "")
		assert.Same(t, primary, got)
	})

	t.Run("filename match overrides primary marking", func(t *testing.T) {
		primary := &modrinthApi.File{Filename: strPtr("a.jar"), Primary: boolPtrMr(true)}
		match := &modrinthApi.File{Filename: strPtr("b.jar"), Primary: boolPtrMr(false)}
		version := &modrinthApi.Version{Files: []*modrinthApi.File{primary, match}}

		got := GetModrinthVersionPrimaryFile(version, "b.jar")
		assert.Same(t, match, got)
	})
}

func boolPtrMr(b bool) *bool { return &b }

func TestGetModrinthProjectSlug(t *testing.T) {
	t.Run("slug set is returned as-is", func(t *testing.T) {
		project := &modrinthApi.Project{Slug: strPtr("jei"), Title: strPtr("Just Enough Items")}
		assert.Equal(t, "jei", getModrinthProjectSlug(project))
	})

	t.Run("nil slug falls back to slugified title", func(t *testing.T) {
		project := &modrinthApi.Project{Title: strPtr("Just Enough Items")}
		assert.Equal(t, core.SlugifyName("Just Enough Items"), getModrinthProjectSlug(project))
	})
}

func TestModrinthNewMod(t *testing.T) {
	newProject := func(server, client string) *modrinthApi.Project {
		return &modrinthApi.Project{
			ID:          strPtr("AANobbMI"),
			Slug:        strPtr("jei"),
			Title:       strPtr("Just Enough Items"),
			ProjectType: strPtr("mod"),
			ServerSide:  strPtr(server),
			ClientSide:  strPtr(client),
		}
	}

	newVersion := func(hashes map[string]string) *modrinthApi.Version {
		return &modrinthApi.Version{
			ID:      strPtr("version123"),
			Loaders: []string{"fabric"},
			Files: []*modrinthApi.File{
				{
					Filename: strPtr("jei-1.20.1.jar"),
					URL:      strPtr("https://cdn.modrinth.com/data/AANobbMI/versions/version123/jei-1.20.1.jar"),
					Primary:  boolPtrMr(true),
					Hashes:   hashes,
				},
			},
		}
	}

	t.Run("happy path builds expected mod fields", func(t *testing.T) {
		project := newProject("required", "required")
		version := newVersion(map[string]string{"sha512": "abc512"})

		mod, err := ModrinthNewMod(project, version, "", []string{"fabric"}, "")
		require.NoError(t, err)

		assert.Equal(t, "jei", mod.Slug)
		assert.Equal(t, "Just Enough Items", mod.Name)
		assert.Equal(t, "jei-1.20.1.jar", mod.FileName)
		assert.Equal(t, core.UniversalSide, mod.Side)
		assert.Equal(t, "mods", mod.ModType)
		assert.Equal(t, "sha512", mod.Download.HashFormat)
		assert.Equal(t, "abc512", mod.Download.Hash)
	})

	t.Run("unsupported both sides falls back to universal", func(t *testing.T) {
		project := newProject("unsupported", "unsupported")
		version := newVersion(map[string]string{"sha512": "abc512"})

		mod, err := ModrinthNewMod(project, version, "", []string{"fabric"}, "")
		require.NoError(t, err)
		assert.Equal(t, core.UniversalSide, mod.Side)
	})

	t.Run("file with no recognized hash errors", func(t *testing.T) {
		project := newProject("required", "required")
		version := newVersion(map[string]string{})

		_, err := ModrinthNewMod(project, version, "", []string{"fabric"}, "")
		assert.Error(t, err)
	})
}

func TestModrinthFindMissingDependencies(t *testing.T) {
	pack := core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

	depType := "required"
	depProjectID := "dep-project"
	rootVersion := &modrinthApi.Version{
		Dependencies: []*modrinthApi.Dependency{
			{ProjectID: &depProjectID, DependencyType: &depType},
		},
	}

	t.Run("resolves a single required project dependency", func(t *testing.T) {
		withMrClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			switch r.URL.Path {
			case "/projects":
				_, _ = w.Write([]byte(`[{"id":"dep-project","title":"Dep Mod","project_type":"mod","client_side":"required","server_side":"required"}]`))
			case "/project/dep-project/version":
				_, _ = w.Write([]byte(`[{"id":"dep-version","project_id":"dep-project","version_number":"1.0","game_versions":["1.20.1"],"date_published":"2024-01-01T00:00:00Z","files":[{"filename":"dep.jar","primary":true,"url":"https://example.com/dep.jar","hashes":{"sha512":"deadbeef"}}]}]`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))

		mods, err := ModrinthFindMissingDependencies(rootVersion, pack, "")
		require.NoError(t, err)
		require.Len(t, mods, 1)
		assert.Equal(t, "Dep Mod", mods[0].Name)
		assert.Equal(t, "dep.jar", mods[0].FileName)
	})

	t.Run("no required dependencies returns no mods", func(t *testing.T) {
		version := &modrinthApi.Version{}
		mods, err := ModrinthFindMissingDependencies(version, pack, "")
		require.NoError(t, err)
		assert.Empty(t, mods)
	})
}
