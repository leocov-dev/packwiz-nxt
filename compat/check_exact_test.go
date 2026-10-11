package compat

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

func quiltMeta(id, version string, deps ...jarmeta.Dependency) jarmeta.LoaderMeta {
	m := fabricMeta(id, version, deps...)
	m.Loader = jarmeta.LoaderQuilt
	return m
}

func withProvides(m jarmeta.LoaderMeta, versions map[string]string, ids ...string) jarmeta.LoaderMeta {
	m.Provides = ids
	m.ProvidesVersions = versions
	return m
}

func withNested(m jarmeta.LoaderMeta, jars ...string) jarmeta.LoaderMeta {
	m.NestedJars = jars
	return m
}

func forgeMeta(id string, deps ...jarmeta.Dependency) jarmeta.LoaderMeta {
	return jarmeta.LoaderMeta{
		Loader: jarmeta.LoaderForge, ModID: id, Mods: []jarmeta.Mod{{ID: id}},
		Dependencies: deps, ModLoader: "javafml", Environment: jarmeta.EnvBoth,
	}
}

var (
	quiltTarget = Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderQuilt}
	forgeTarget = Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderForge, LoaderVersion: "47.2.0"}
)

// TestCheckExactFindings compares complete findings, at least one case per code.
func TestCheckExactFindings(t *testing.T) {
	tests := []struct {
		name    string
		target  Target
		entries []Entry
		want    []Finding
	}{
		{
			name:   "wrong_loader",
			target: fabricTarget,
			entries: []Entry{entry("m", jarmeta.LoaderMeta{
				Loader: jarmeta.LoaderForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
			})},
			want: []Finding{{
				Severity: SeverityError, Code: CodeWrongLoader, Slug: "m",
				Message: "mod has no fabric metadata (jar supports: forge)",
				Detail:  map[string]string{"pack_loader": "fabric", "jar_loaders": "forge"},
			}},
		},
		{
			name:    "mc_version_mismatch",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", "1.19.x")))},
			want: []Finding{{
				Severity: SeverityError, Code: CodeMCVersionMismatch, Slug: "m",
				Message: "mod requires Minecraft 1.19.x but the pack uses 1.20.1",
				Detail:  map[string]string{"pack_version": "1.20.1", "required": "1.19.x"},
			}},
		},
		{
			name:    "loader_version_mismatch",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "fabricloader", ">=0.16")))},
			want: []Finding{{
				Severity: SeverityError, Code: CodeLoaderVersionMismatch, Slug: "m",
				Message: "mod requires fabric >=0.16 but the pack uses 0.15.11",
				Detail:  map[string]string{"loader": "fabric", "pack_version": "0.15.11", "required": ">=0.16"},
			}},
		},
		{
			name:    "missing_dependency",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=0.90")))},
			want: []Finding{{
				Severity: SeverityError, Code: CodeMissingDependency, Slug: "m",
				Message: `required mod "lib" (>=0.90) is not in the pack`,
				Detail:  map[string]string{"dependency": "lib", "required": ">=0.90"},
			}},
		},
		{
			name:   "dependency_version",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=2.0"))),
				entry("lib", fabricMeta("lib", "1.5.0")),
			},
			want: []Finding{{
				Severity: SeverityError, Code: CodeDependencyVersion, Slug: "m",
				Message: `required mod "lib" needs version >=2.0 but the pack has 1.5.0`,
				Detail:  map[string]string{"dependency": "lib", "required": ">=2.0", "found": "1.5.0"},
			}},
		},
		{
			name:   "breaks",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "b", "<2.0"), jarmeta.KindBreaks))),
				entry("b", fabricMeta("b", "1.5")),
			},
			want: []Finding{{
				Severity: SeverityError, Code: CodeBreaks, Slug: "a",
				Message: `mod breaks "b" (<2.0) from "b"`,
				Detail:  map[string]string{"mod_id": "b", "declared": "<2.0", "other_slug": "b", "other_version": "1.5"},
			}},
		},
		{
			name:   "conflicts",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "b", "*"), jarmeta.KindConflicts))),
				entry("b", fabricMeta("b", "1.0")),
			},
			want: []Finding{{
				Severity: SeverityWarning, Code: CodeConflicts, Slug: "a",
				Message: `mod conflicts with "b" (*) from "b"`,
				Detail:  map[string]string{"mod_id": "b", "declared": "*", "other_slug": "b", "other_version": "1.0"},
			}},
		},
		{
			name:   "duplicate_mod_id",
			target: fabricTarget,
			entries: []Entry{
				entry("two", fabricMeta("same", "1.1")),
				entry("one", fabricMeta("same", "1.0")),
			},
			want: []Finding{
				{
					Severity: SeverityError, Code: CodeDuplicateModID, Slug: "one",
					Message: `mod id "same" is provided by multiple mods: one, two`,
					Detail:  map[string]string{"mod_id": "same", "slugs": "one,two"},
				},
				{
					Severity: SeverityError, Code: CodeDuplicateModID, Slug: "two",
					Message: `mod id "same" is provided by multiple mods: one, two`,
					Detail:  map[string]string{"mod_id": "same", "slugs": "one,two"},
				},
			},
		},
		{
			name:    "no_metadata",
			target:  fabricTarget,
			entries: []Entry{{Slug: "lib", ParseErr: jarmeta.ErrNoMetadata}},
			want: []Finding{{
				Severity: SeverityInfo, Code: CodeNoMetadata, Slug: "lib",
				Message: "mod jar has no loader metadata (common for libraries)",
			}},
		},
		{
			name:    "parse_error",
			target:  fabricTarget,
			entries: []Entry{{Slug: "bad", ParseErr: errors.New("boom")}},
			want: []Finding{{
				Severity: SeverityWarning, Code: CodeParseError, Slug: "bad",
				Message: "mod jar could not be fully read: boom",
				Detail:  map[string]string{"error": "boom"},
			}},
		},
		{
			name:   "side_mismatch",
			target: fabricTarget,
			entries: []Entry{{Slug: "c", Side: core.ServerSide, Metas: []jarmeta.LoaderMeta{func() jarmeta.LoaderMeta {
				m := fabricMeta("c", "1.0")
				m.Environment = jarmeta.EnvClient
				return m
			}()}}},
			want: []Finding{{
				Severity: SeverityInfo, Code: CodeSideMismatch, Slug: "c",
				Message: `mod declares environment "client" but is set to side "server" in the pack`,
				Detail:  map[string]string{"environment": "client", "side": "server"},
			}},
		},
		{
			name:    "unevaluable_range",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", ">=zzz")))},
			want: []Finding{{
				Severity: SeverityInfo, Code: CodeUnevaluableRange, Slug: "m",
				Message: `version range for "minecraft" could not be evaluated: invalid version range: ">=zzz"`,
				Detail:  map[string]string{"id": "minecraft", "error": `invalid version range: ">=zzz"`},
			}},
		},
		{
			name:   "missing dependency downgraded when a nested jar exists",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "somelib"))),
				entry("n", withNested(fabricMeta("n", "1.0"), "META-INF/jars/x.jar")),
			},
			want: []Finding{{
				Severity: SeverityWarning, Code: CodeMissingDependency, Slug: "m",
				Message: `required mod "somelib" (any) is not in the pack (may be bundled inside n)`,
				Detail:  map[string]string{"dependency": "somelib", "required": "any", "note": "may be bundled inside n"},
			}},
		},
		{
			name:   "missing dependency downgraded when a jar is unreadable",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "somelib"))),
				{Slug: "bad", ParseErr: errors.New("boom")},
			},
			want: []Finding{
				{
					Severity: SeverityWarning, Code: CodeParseError, Slug: "bad",
					Message: "mod jar could not be fully read: boom",
					Detail:  map[string]string{"error": "boom"},
				},
				{
					Severity: SeverityWarning, Code: CodeMissingDependency, Slug: "m",
					Message: `required mod "somelib" (any) is not in the pack (unreadable jar present: bad)`,
					Detail:  map[string]string{"dependency": "somelib", "required": "any", "note": "unreadable jar present: bad"},
				},
			},
		},
		{
			name:   "missing dependency stays error next to a library without metadata",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "somelib"))),
				{Slug: "lib", ParseErr: jarmeta.ErrNoMetadata},
			},
			want: []Finding{
				{
					Severity: SeverityError, Code: CodeMissingDependency, Slug: "m",
					Message: `required mod "somelib" (any) is not in the pack`,
					Detail:  map[string]string{"dependency": "somelib", "required": "any"},
				},
				{
					Severity: SeverityInfo, Code: CodeNoMetadata, Slug: "lib",
					Message: "mod jar has no loader metadata (common for libraries)",
				},
			},
		},
		{
			name:   "unreadable and nested notes combine",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "somelib"))),
				entry("n", withNested(fabricMeta("n", "1.0"), "a.jar")),
				{Slug: "bad", ParseErr: jarmeta.ErrMalformed},
			},
			want: []Finding{
				{
					Severity: SeverityWarning, Code: CodeParseError, Slug: "bad",
					Message: "mod jar could not be fully read: " + jarmeta.ErrMalformed.Error(),
					Detail:  map[string]string{"error": jarmeta.ErrMalformed.Error()},
				},
				{
					Severity: SeverityWarning, Code: CodeMissingDependency, Slug: "m",
					Message: `required mod "somelib" (any) is not in the pack (may be bundled inside n; unreadable jar present: bad)`,
					Detail: map[string]string{
						"dependency": "somelib", "required": "any",
						"note": "may be bundled inside n; unreadable jar present: bad",
					},
				},
			},
		},
		{
			name:   "quilt provides with own version fails range",
			target: quiltTarget,
			entries: []Entry{
				entry("m", quiltMeta("m", "1.0", dep("m", "fabric-api", ">=0.95"))),
				entry("qfapi", withProvides(quiltMeta("qfapi", "7.0.0"), map[string]string{"fabric-api": "0.90.0"}, "fabric-api")),
			},
			want: []Finding{{
				Severity: SeverityError, Code: CodeDependencyVersion, Slug: "m",
				Message: `required mod "fabric-api" needs version >=0.95 but the pack has 0.90.0`,
				Detail:  map[string]string{"dependency": "fabric-api", "required": ">=0.95", "found": "0.90.0"},
			}},
		},
		{
			name:   "duplicate via provides",
			target: fabricTarget,
			entries: []Entry{
				entry("one", fabricMeta("a", "1.0")),
				entry("two", withProvides(fabricMeta("b", "1.0"), nil, "a")),
			},
			want: []Finding{
				{
					Severity: SeverityError, Code: CodeDuplicateModID, Slug: "one",
					Message: `mod id "a" is provided by multiple mods: one, two`,
					Detail:  map[string]string{"mod_id": "a", "slugs": "one,two"},
				},
				{
					Severity: SeverityError, Code: CodeDuplicateModID, Slug: "two",
					Message: `mod id "a" is provided by multiple mods: one, two`,
					Detail:  map[string]string{"mod_id": "a", "slugs": "one,two"},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Check(tc.target, tc.entries))
		})
	}
}

// TestCheckNoFindings covers cases that must stay silent.
func TestCheckNoFindings(t *testing.T) {
	fabricAPI := fabricMeta("fabric-api", "0.92.2")
	tests := []struct {
		name    string
		target  Target
		entries []Entry
	}{
		{
			name:   "kotlinforforge loaderVersion is not the forge version",
			target: forgeTarget,
			entries: []Entry{entry("m", func() jarmeta.LoaderMeta {
				m := forgeMeta("m")
				m.ModLoader, m.LoaderVersionRange = "kotlinforforge", "[4,)"
				return m
			}())},
		},
		{
			name:   "fabric api module satisfied by fabric-api",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0",
					dep("m", "fabric-resource-loader-v0", ">=0.11"), dep("m", "fabric-api-base"),
					dep("m", "fabric-lifecycle-events-v1", "*"))),
				entry("fabric-api", fabricAPI),
			},
		},
		{
			name:   "quilted fabric api module satisfied by quilted_fabric_api",
			target: quiltTarget,
			entries: []Entry{
				entry("m", quiltMeta("m", "1.0", dep("m", "quilted_fabric_resource_loader_v0"))),
				entry("qfapi", quiltMeta("quilted_fabric_api", "7.0.0")),
			},
		},
		{
			name:   "quilt provide without version has unknown version",
			target: quiltTarget,
			entries: []Entry{
				entry("m", quiltMeta("m", "1.0", dep("m", "fabric-api", ">=0.95"))),
				entry("qfapi", withProvides(quiltMeta("qfapi", "7.0.0"), nil, "fabric-api")),
			},
		},
		{
			name:   "quilt provide with own version satisfies range",
			target: quiltTarget,
			entries: []Entry{
				entry("m", quiltMeta("m", "1.0", dep("m", "fabric-api", ">=0.80"))),
				entry("qfapi", withProvides(quiltMeta("qfapi", "7.0.0"), map[string]string{"fabric-api": "0.90.0"}, "fabric-api")),
			},
		},
		{
			name:   "fabric provides share the mod version",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "legacy", ">=2.0"))),
				entry("p", withProvides(fabricMeta("p", "2.5"), nil, "legacy")),
			},
		},
		{
			name:    "snapshot pack mc with maven range",
			target:  Target{MinecraftVersion: "24w14a", Loader: jarmeta.LoaderForge},
			entries: []Entry{entry("m", forgeMeta("m", dep("m", "minecraft", "[1.20,1.21)")))},
		},
		{
			name:    "pre-release pack mc with maven range",
			target:  Target{MinecraftVersion: "1.21-pre1", Loader: jarmeta.LoaderNeoForge},
			entries: []Entry{entry("m", forgeMetaAs(jarmeta.LoaderNeoForge, "m", dep("m", "minecraft", "[1.20,1.21)")))},
		},
		{
			name:    "release candidate pack mc with maven range",
			target:  Target{MinecraftVersion: "1.21-rc1", Loader: jarmeta.LoaderForge},
			entries: []Entry{entry("m", forgeMeta("m", dep("m", "minecraft", "[1.20,1.21)")))},
		},
		{
			name:    "fabric comparator with trailing x",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", ">=1.20.x")))},
		},
		{
			name:   "same slug twice",
			target: fabricTarget,
			entries: []Entry{
				entry("x", withProvides(fabricMeta("same", "1.0"), nil, "alias")),
				entry("x", withProvides(fabricMeta("same", "1.0"), nil, "alias")),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, Check(tc.target, tc.entries))
		})
	}
}

func forgeMetaAs(l jarmeta.Loader, id string, deps ...jarmeta.Dependency) jarmeta.LoaderMeta {
	m := forgeMeta(id, deps...)
	m.Loader = l
	return m
}

// TestCheckFabricPrefixIsNotAnyMod ensures only Fabric API modules are
// treated as bundled: other mods starting with "fabric-" stay missing.
func TestCheckFabricPrefixIsNotAnyMod(t *testing.T) {
	got := Check(fabricTarget, []Entry{
		entry("m", fabricMeta("m", "1.0", dep("m", "fabric-language-kotlin"))),
		entry("fabric-api", fabricMeta("fabric-api", "0.92.2")),
	})
	if assert.Len(t, got, 1) {
		assert.Equal(t, SeverityError, got[0].Severity)
		assert.Equal(t, CodeMissingDependency, got[0].Code)
	}
}

// TestCheckFabricModuleWithoutAPI keeps the finding when no Fabric API exists.
func TestCheckFabricModuleWithoutAPI(t *testing.T) {
	got := Check(fabricTarget, []Entry{
		entry("m", fabricMeta("m", "1.0", dep("m", "fabric-resource-loader-v0"))),
	})
	if assert.Len(t, got, 1) {
		assert.Equal(t, CodeMissingDependency, got[0].Code)
		assert.Equal(t, SeverityError, got[0].Severity)
	}
}

func permutations(es []Entry) [][]Entry {
	if len(es) <= 1 {
		return [][]Entry{append([]Entry(nil), es...)}
	}
	var out [][]Entry
	for i := range es {
		rest := append(append([]Entry(nil), es[:i]...), es[i+1:]...)
		for _, p := range permutations(rest) {
			out = append(out, append([]Entry{es[i]}, p...))
		}
	}
	return out
}

// TestCheckInputOrderIndependent checks that findings, including the
// provider named and the lists in details, do not depend on input order.
func TestCheckInputOrderIndependent(t *testing.T) {
	entries := []Entry{
		entry("a", fabricMeta("a", "1.0",
			kind(dep("a", "legacy"), jarmeta.KindBreaks), dep("a", "lib", ">=9.0"))),
		entry("c", withProvides(fabricMeta("c", "3.0"), nil, "legacy")),
		entry("b", withProvides(fabricMeta("b", "2.0"), nil, "legacy")),
		entry("lib2", withProvides(fabricMeta("lib2", "1.5"), nil, "lib")),
		entry("lib1", withProvides(fabricMeta("lib1", "1.0"), nil, "lib")),
	}
	want := Check(fabricTarget, entries)
	var breaks, version *Finding
	for i := range want {
		switch want[i].Code {
		case CodeBreaks:
			breaks = &want[i]
		case CodeDependencyVersion:
			version = &want[i]
		}
	}
	if breaks == nil || version == nil {
		t.Fatalf("expected breaks and dependency_version findings, got %v", want)
	}
	assert.Equal(t, "b", breaks.Detail["other_slug"])
	assert.Equal(t, "1.0, 1.5", version.Detail["found"])
	for _, p := range permutations(entries) {
		assert.Equal(t, want, Check(fabricTarget, p))
	}
}
