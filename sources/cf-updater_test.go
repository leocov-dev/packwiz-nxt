package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/leocov-dev/packwiz-nxt/core"
)

func TestGetCurseforgeVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"release version unchanged", "1.20.1", "1.20.1"},
		{"pre-release dash form", "1.19-pre1", "1.19-Snapshot"},
		{"Pre-Release capitalized form", "1.19 Pre-Release 2", "1.19-Snapshot"},
		{"pre-release lowercase form", "1.19 Pre-release 2", "1.19-Snapshot"},
		{"release-candidate form", "1.19-rc1", "1.19-Snapshot"},
		{"snapshot table: 22w11a is the 1.19 boundary", "22w11a", "1.19-Snapshot"},
		{"snapshot table: 22w10a falls in the 1.18 bucket", "22w10a", "1.18-Snapshot"},
		{"snapshot table: oldest bucket 11w47a", "11w47a", "1.1-Snapshot"},
		{"snapshot-shaped but unparseable week digits ignored (regex requires digits)", "22wxxa", "22wxxa"},
		{"non-matching input returned unchanged", "not-a-version", "not-a-version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetCurseforgeVersion(tt.in))
		})
	}
}

func TestGetCurseforgeVersions(t *testing.T) {
	t.Run("maps each version, preserving order and length", func(t *testing.T) {
		got := GetCurseforgeVersions([]string{"1.20.1", "22w11a", "22w10a"})
		assert.Equal(t, []string{"1.20.1", "1.19-Snapshot", "1.18-Snapshot"}, got)
	})

	t.Run("empty input returns empty output", func(t *testing.T) {
		got := GetCurseforgeVersions([]string{})
		assert.Equal(t, []string{}, got)
	})
}

func TestCurseforgeParseUrl(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantCategory string
		wantSlug     string
		wantFileID   uint32
		wantErr      bool
	}{
		{"legacy project URL, no file", "https://minecraft.curseforge.com/projects/jei", "", "jei", 0, false},
		{"legacy project URL, with file", "https://minecraft.curseforge.com/projects/jei/files/12345", "", "jei", 12345, false},
		{"modern URL, no file", "https://www.curseforge.com/minecraft/mc-mods/jei", "mc-mods", "jei", 0, false},
		{"modern URL with beta prefix, with download id", "https://beta.curseforge.com/minecraft/mc-mods/jei/download/98765", "mc-mods", "jei", 98765, false},
		{"modern URL with legacy prefix", "https://legacy.curseforge.com/minecraft/mc-mods/jei", "mc-mods", "jei", 0, false},
		{"bare slug", "jei", "", "jei", 0, false},
		{"non-matching input", "JEI Mod!!", "", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, slug, fileID, err := CurseforgeParseUrl(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantCategory, category)
			assert.Equal(t, tt.wantSlug, slug)
			assert.Equal(t, tt.wantFileID, fileID)
		})
	}

	t.Run("fileID beyond uint32 range propagates a parse error", func(t *testing.T) {
		_, _, _, err := CurseforgeParseUrl("https://minecraft.curseforge.com/projects/jei/files/99999999999999999999")
		assert.Error(t, err)
	})
}

func TestGetCfModType(t *testing.T) {
	tests := []struct {
		name       string
		gameID     uint32
		classID    uint32
		categoryID uint32
		want       string
	}{
		{"known classID mods", minecraftGameId, 6, 0, "mods"},
		{"known classID resourcepacks", minecraftGameId, 12, 0, "resourcepacks"},
		{"unknown classID falls back to known categoryID", minecraftGameId, 999, 17, "saves"},
		{"unknown gameID", 1, 6, 0, "unknown"},
		{"known gameID, both classID/categoryID unknown", minecraftGameId, 999, 999, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetCfModType(tt.gameID, tt.classID, tt.categoryID))
		})
	}
}

func TestCfGetSearchLoaderType(t *testing.T) {
	tests := []struct {
		name string
		pack core.Pack
		want ModloaderType
	}{
		{"fabric only", core.Pack{Versions: map[string]string{"fabric": "0.1.0"}}, ModloaderTypeFabric},
		{"forge only", core.Pack{Versions: map[string]string{"forge": "1.0"}}, ModloaderTypeForge},
		{"fabric and quilt both present is ambiguous", core.Pack{Versions: map[string]string{"fabric": "0.1.0", "quilt": "0.2.0"}}, ModloaderTypeAny},
		{"no loaders", core.Pack{Versions: map[string]string{"minecraft": "1.20.1"}}, ModloaderTypeAny},
		{"neoforge alongside forge is ambiguous", core.Pack{Versions: map[string]string{"forge": "1.0", "neoforge": "1.0"}}, ModloaderTypeAny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CfGetSearchLoaderType(tt.pack))
		})
	}
}

func TestCfFilterLoaderTypeIndex(t *testing.T) {
	t.Run("empty packLoaders allows all", func(t *testing.T) {
		loaderType, ok := CfFilterLoaderTypeIndex(nil, ModloaderTypeFabric)
		assert.True(t, ok)
		assert.Equal(t, ModloaderTypeAny, loaderType)
	})

	t.Run("modLoaderType Any always passes", func(t *testing.T) {
		loaderType, ok := CfFilterLoaderTypeIndex([]string{"forge"}, ModloaderTypeAny)
		assert.True(t, ok)
		assert.Equal(t, ModloaderTypeAny, loaderType)
	})

	t.Run("loader present in packLoaders passes through", func(t *testing.T) {
		loaderType, ok := CfFilterLoaderTypeIndex([]string{"fabric"}, ModloaderTypeFabric)
		assert.True(t, ok)
		assert.Equal(t, ModloaderTypeFabric, loaderType)
	})

	t.Run("loader absent from packLoaders is unsupported", func(t *testing.T) {
		loaderType, ok := CfFilterLoaderTypeIndex([]string{"forge"}, ModloaderTypeFabric)
		assert.False(t, ok)
		assert.Equal(t, ModloaderTypeAny, loaderType)
	})
}

func TestCfFilterFileInfoLoaderIndex(t *testing.T) {
	t.Run("empty packLoaders allows all", func(t *testing.T) {
		loaderType, ok := CfFilterFileInfoLoaderIndex(nil, CfModFileInfo{GameVersions: []string{"Fabric"}})
		assert.True(t, ok)
		assert.Equal(t, ModloaderTypeAny, loaderType)
	})

	t.Run("picks the most-preferred of multiple matching loaders", func(t *testing.T) {
		loaderType, ok := CfFilterFileInfoLoaderIndex(
			[]string{"fabric", "quilt"},
			CfModFileInfo{GameVersions: []string{"Fabric", "Quilt"}},
		)
		assert.True(t, ok)
		assert.Equal(t, ModloaderTypeQuilt, loaderType)
	})

	t.Run("no matching loader is unsupported", func(t *testing.T) {
		loaderType, ok := CfFilterFileInfoLoaderIndex(
			[]string{"forge"},
			CfModFileInfo{GameVersions: []string{"Fabric"}},
		)
		assert.False(t, ok)
		assert.Equal(t, ModloaderTypeAny, loaderType)
	})
}

func TestCfIsBetterCandidate(t *testing.T) {
	tests := []struct {
		name           string
		mcVerIdx       int
		loaderIdx      ModloaderType
		loaderValid    bool
		candidateID    uint32
		bestMcVer      int
		bestLoaderType ModloaderType
		currentBestID  uint32
		want           bool
	}{
		{"negative mcVerIdx is never better", -1, ModloaderTypeFabric, true, 5, 0, ModloaderTypeAny, 1, false},
		{"invalid loader is never better", 0, ModloaderTypeFabric, false, 5, 0, ModloaderTypeAny, 1, false},
		{"higher mcVerIdx wins regardless of loader/ID", 1, ModloaderTypeAny, true, 1, 0, ModloaderTypeQuilt, 999, true},
		{"equal mcVerIdx, higher loaderIdx wins", 0, ModloaderTypeQuilt, true, 1, 0, ModloaderTypeFabric, 999, true},
		{"equal mcVerIdx, lower loaderIdx loses", 0, ModloaderTypeFabric, true, 999, 0, ModloaderTypeQuilt, 1, false},
		{"equal mcVerIdx and loaderIdx, higher ID wins", 0, ModloaderTypeFabric, true, 5, 0, ModloaderTypeFabric, 1, true},
		{"bestLoaderType Any is treated as neutral, falls through to ID", 0, ModloaderTypeFabric, true, 5, 0, ModloaderTypeAny, 1, true},
		{"loaderIdx Any is treated as neutral, falls through to ID", 0, ModloaderTypeAny, true, 1, 0, ModloaderTypeFabric, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfIsBetterCandidate(tt.mcVerIdx, tt.loaderIdx, tt.loaderValid, tt.candidateID, tt.bestMcVer, tt.bestLoaderType, tt.currentBestID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCfFindLatestFile(t *testing.T) {
	t.Run("picks the file matching the latest supported MC version", func(t *testing.T) {
		modInfo := CfModInfo{
			LatestFiles: []CfModFileInfo{
				{ID: 1, FileName: "old.jar", GameVersions: []string{"1.19"}},
				{ID: 2, FileName: "new.jar", GameVersions: []string{"1.20"}},
			},
		}
		fileID, fileInfoData, fileName := CfFindLatestFile(modInfo, []string{"1.19", "1.20"}, nil)
		assert.Equal(t, uint32(2), fileID)
		assert.Equal(t, "new.jar", fileName)
		if assert.NotNil(t, fileInfoData) {
			assert.Equal(t, uint32(2), fileInfoData.ID)
		}
	})

	t.Run("picks the file matching a preferred loader when MC versions tie", func(t *testing.T) {
		modInfo := CfModInfo{
			LatestFiles: []CfModFileInfo{
				{ID: 1, FileName: "fabric.jar", GameVersions: []string{"1.20", "Fabric"}},
				{ID: 2, FileName: "quilt.jar", GameVersions: []string{"1.20", "Quilt"}},
			},
		}
		fileID, _, fileName := CfFindLatestFile(modInfo, []string{"1.20"}, []string{"fabric", "quilt"})
		assert.Equal(t, uint32(2), fileID)
		assert.Equal(t, "quilt.jar", fileName)
	})

	t.Run("falls back to GameVersionLatestFiles when LatestFiles is empty", func(t *testing.T) {
		modInfo := CfModInfo{
			GameVersionLatestFiles: []struct {
				GameVersion string        `json:"gameVersion"`
				ID          uint32        `json:"fileId"`
				Name        string        `json:"filename"`
				FileType    fileType      `json:"releaseType"`
				Modloader   ModloaderType `json:"modLoader"`
			}{
				{ID: 3, Name: "fallback.jar", GameVersion: "1.20.1", Modloader: ModloaderTypeAny},
			},
		}
		fileID, fileInfoData, fileName := CfFindLatestFile(modInfo, []string{"1.20.1"}, nil)
		assert.Equal(t, uint32(3), fileID)
		assert.Equal(t, "fallback.jar", fileName)
		assert.Nil(t, fileInfoData)
	})

	t.Run("no candidate matches pack's MC versions/loaders", func(t *testing.T) {
		modInfo := CfModInfo{
			LatestFiles: []CfModFileInfo{
				{ID: 1, FileName: "old.jar", GameVersions: []string{"1.16"}},
			},
		}
		fileID, fileInfoData, fileName := CfFindLatestFile(modInfo, []string{"1.20"}, nil)
		assert.Equal(t, uint32(0), fileID)
		assert.Nil(t, fileInfoData)
		assert.Equal(t, "", fileName)
	})
}
