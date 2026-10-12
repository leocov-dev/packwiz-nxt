package compat

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/jarmeta"
	"github.com/stretchr/testify/assert"
)

// fabricMeta builds a Fabric LoaderMeta with the usual platform requirements.
func fabricMeta(id, version string, deps ...jarmeta.Dependency) jarmeta.LoaderMeta {
	return jarmeta.LoaderMeta{
		Loader: jarmeta.LoaderFabric, ModID: id, Version: version,
		Mods:         []jarmeta.Mod{{ID: id, Version: version}},
		Dependencies: deps,
		Environment:  jarmeta.EnvBoth,
	}
}

func dep(owner, id string, ranges ...string) jarmeta.Dependency {
	return jarmeta.Dependency{
		Owner: owner, ID: id, Kind: jarmeta.KindDepends, Mandatory: true,
		VersionRanges: ranges, Side: jarmeta.SideBoth,
	}
}

func kind(d jarmeta.Dependency, k jarmeta.DependencyKind) jarmeta.Dependency {
	d.Kind = k
	d.Mandatory = false
	return d
}

func entry(slug string, metas ...jarmeta.LoaderMeta) Entry {
	return Entry{Slug: slug, Side: core.UniversalSide, Metas: metas}
}

var fabricTarget = Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderFabric, LoaderVersion: "0.15.11"}

// codes returns "severity:code:slug" for each finding.
func codes(fs []Finding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%s:%s:%s", f.Severity, f.Code, f.Slug))
	}
	return out
}

func TestCheckFindings(t *testing.T) {
	tests := []struct {
		name    string
		target  Target
		entries []Entry
		want    []string
	}{
		{
			name: "wrong loader lists jar loaders",
			entries: []Entry{entry("m", jarmeta.LoaderMeta{
				Loader: jarmeta.LoaderForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
			})},
			target: fabricTarget,
			want:   []string{"error:wrong_loader:m"},
		},
		{
			name:   "quilt accepts fabric jar",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderQuilt, LoaderVersion: "0.26.0"},
			entries: []Entry{entry("m", fabricMeta("m", "1.0.0",
				dep("m", "minecraft", "~1.20"), dep("m", "fabricloader", ">=0.15")))},
			want: []string{},
		},
		{
			name:    "fabric pack rejects quilt-only jar",
			target:  fabricTarget,
			entries: []Entry{entry("m", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}}})},
			want:    []string{"error:wrong_loader:m"},
		},
		{
			name:   "multi-loader jar picks matching meta",
			target: fabricTarget,
			entries: []Entry{entry("m",
				jarmeta.LoaderMeta{Loader: jarmeta.LoaderForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
					Dependencies: []jarmeta.Dependency{dep("m", "minecraft", "[1.19,1.20)")}},
				fabricMeta("m", "1.0", dep("m", "minecraft", "1.20.x")))},
			want: []string{},
		},
		{
			name:    "mc mismatch",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", "1.19.x")))},
			want:    []string{"error:mc_version_mismatch:m"},
		},
		{
			name:    "mc mismatch any-of alternatives satisfied",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", "1.19.x", "1.20.x")))},
			want:    []string{},
		},
		{
			name:    "mc range unparsable gives info",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", ">=zzz")))},
			want:    []string{"info:unevaluable_range:m"},
		},
		{
			name:    "pack mc snapshot skips comparator without finding",
			target:  Target{MinecraftVersion: "24w14a", Loader: jarmeta.LoaderFabric},
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", ">=1.20")))},
			want:    []string{},
		},
		{
			name:    "empty pack mc skips",
			target:  Target{Loader: jarmeta.LoaderFabric},
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "minecraft", "1.19.x")))},
			want:    []string{},
		},
		{
			name:    "loader version mismatch",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "fabricloader", ">=0.16")))},
			want:    []string{"error:loader_version_mismatch:m"},
		},
		{
			name:    "loader version skipped when empty",
			target:  Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderFabric},
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "fabricloader", ">=0.16")))},
			want:    []string{},
		},
		{
			name:   "quilt pack skips fabricloader range on fabric jar",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderQuilt, LoaderVersion: "0.26.0"},
			entries: []Entry{entry("m", fabricMeta("m", "1.0",
				dep("m", "fabricloader", ">=0.15")))},
			want: []string{},
		},
		{
			name:   "forge file-level loader range mismatch",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderForge, LoaderVersion: "47.2.0"},
			entries: []Entry{entry("m", jarmeta.LoaderMeta{
				Loader: jarmeta.LoaderForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}}, ModLoader: "javafml",
				LoaderVersionRange: "[48,)",
			})},
			want: []string{"error:loader_version_mismatch:m"},
		},
		{
			name:   "neoforge dependency range mismatch, file range ignored",
			target: Target{MinecraftVersion: "1.21.1", Loader: jarmeta.LoaderNeoForge, LoaderVersion: "21.1.172"},
			entries: []Entry{entry("m", jarmeta.LoaderMeta{
				Loader: jarmeta.LoaderNeoForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
				LoaderVersionRange: "[2,)",
				Dependencies:       []jarmeta.Dependency{dep("m", "neoforge", "[21.2,)"), dep("m", "minecraft", "[1.21,1.22)")},
			})},
			want: []string{"error:loader_version_mismatch:m"},
		},
		{
			name:    "missing dependency",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", dep("m", "fabric-api", ">=0.90")))},
			want:    []string{"error:missing_dependency:m"},
		},
		{
			name:   "platform ids ignored",
			target: fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0",
				dep("m", "java", ">=17"), dep("m", "forge", "*"), dep("m", "quilt_loader", "*")))},
			want: []string{},
		},
		{
			name:    "optional dependency not reported",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", func() jarmeta.Dependency { d := dep("m", "x"); d.Mandatory = false; return d }()))},
			want:    []string{},
		},
		{
			name:    "recommends and suggests not reported",
			target:  fabricTarget,
			entries: []Entry{entry("m", fabricMeta("m", "1.0", kind(dep("m", "x"), jarmeta.KindRecommends), kind(dep("m", "y"), jarmeta.KindSuggests)))},
			want:    []string{},
		},
		{
			name:   "provides satisfies dependency",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "oldname", ">=1.0"))),
				entry("p", func() jarmeta.LoaderMeta {
					p := fabricMeta("p", "2.0")
					p.Provides = []string{"oldname"}
					return p
				}()),
			},
			want: []string{},
		},
		{
			name:   "dependency version mismatch",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=2.0"))),
				entry("lib", fabricMeta("lib", "1.5.0")),
			},
			want: []string{"error:dependency_version:m"},
		},
		{
			name:   "dependency version satisfied",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=2.0"))),
				entry("lib", fabricMeta("lib", "2.1.0")),
			},
			want: []string{},
		},
		{
			name:   "all-of ranges unsatisfied",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", func() jarmeta.Dependency {
					d := dep("m", "lib")
					d.VersionRangesAll = []string{">=1", "<2"}
					return d
				}())),
				entry("lib", fabricMeta("lib", "2.1.0")),
			},
			want: []string{"error:dependency_version:m"},
		},
		{
			name:   "placeholder provider version no false positive",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=2.0"))),
				entry("lib", func() jarmeta.LoaderMeta {
					l := fabricMeta("lib", "${file.jarVersion}")
					l.VersionIsPlaceholder = true
					l.Mods[0].VersionIsPlaceholder = true
					return l
				}()),
			},
			want: []string{},
		},
		{
			name:   "placeholder detected from text without flag",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=2.0"))),
				entry("lib", fabricMeta("lib", "${version}")),
			},
			want: []string{},
		},
		{
			name:   "non-semver provider version is unknown",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=2.0"))),
				entry("lib", fabricMeta("lib", "a.b.c.d")),
			},
			want: []string{},
		},
		{
			name:   "bad dependency range gives info only",
			target: fabricTarget,
			entries: []Entry{
				entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=nope"))),
				entry("lib", fabricMeta("lib", "1.0.0")),
			},
			want: []string{"info:unevaluable_range:m"},
		},
		{
			name:   "quilt group satisfied by one member",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderQuilt},
			entries: []Entry{
				entry("m", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
					Dependencies: []jarmeta.Dependency{{Owner: "m", Kind: jarmeta.KindDepends, Mandatory: true,
						AnyOf: []jarmeta.Dependency{dep("m", "a"), dep("m", "b", ">=1")}}}}),
				entry("b", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "b", Version: "1.2", Mods: []jarmeta.Mod{{ID: "b", Version: "1.2"}}}),
			},
			want: []string{},
		},
		{
			name:   "quilt group none present",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderQuilt},
			entries: []Entry{
				entry("m", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
					Dependencies: []jarmeta.Dependency{{Owner: "m", Kind: jarmeta.KindDepends, Mandatory: true,
						AnyOf: []jarmeta.Dependency{dep("m", "a"), dep("m", "b", ">=1")}}}}),
			},
			want: []string{"error:missing_dependency:m"},
		},
		{
			name:   "quilt group wrong versions",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderQuilt},
			entries: []Entry{
				entry("m", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
					Dependencies: []jarmeta.Dependency{{Owner: "m", Kind: jarmeta.KindDepends, Mandatory: true,
						AnyOf: []jarmeta.Dependency{dep("m", "a"), dep("m", "b", ">=3")}}}}),
				entry("b", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "b", Version: "1.2", Mods: []jarmeta.Mod{{ID: "b", Version: "1.2"}}}),
			},
			want: []string{"error:dependency_version:m"},
		},
		{
			name:   "forge client-only dependency ignored for server entry",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderForge},
			entries: []Entry{{Slug: "m", Side: core.ServerSide, Metas: []jarmeta.LoaderMeta{{
				Loader: jarmeta.LoaderForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
				Dependencies: []jarmeta.Dependency{func() jarmeta.Dependency {
					d := dep("m", "clientlib")
					d.Side = jarmeta.SideClient
					return d
				}()},
			}}}},
			want: []string{},
		},
		{
			name:   "forge client-only dependency reported for both entry",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderForge},
			entries: []Entry{entry("m", jarmeta.LoaderMeta{
				Loader: jarmeta.LoaderForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m"}},
				Dependencies: []jarmeta.Dependency{func() jarmeta.Dependency {
					d := dep("m", "clientlib")
					d.Side = jarmeta.SideClient
					return d
				}()},
			})},
			want: []string{"error:missing_dependency:m"},
		},
		{
			name:   "forge second mod in jar satisfies dependency",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderForge},
			entries: []Entry{entry("m", jarmeta.LoaderMeta{
				Loader: jarmeta.LoaderForge, ModID: "m", Mods: []jarmeta.Mod{{ID: "m", Version: "1"}, {ID: "m_api", Version: "1"}},
				Dependencies: []jarmeta.Dependency{dep("m", "m_api", "[1,)")},
			})},
			want: []string{},
		},
		{
			name:   "breaks",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "b", "<2.0"), jarmeta.KindBreaks))),
				entry("b", fabricMeta("b", "1.5")),
			},
			want: []string{"error:breaks:a"},
		},
		{
			name:   "breaks not matching version",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "b", "<2.0"), jarmeta.KindBreaks))),
				entry("b", fabricMeta("b", "2.5")),
			},
			want: []string{},
		},
		{
			name:   "breaks without range matches any version",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "b"), jarmeta.KindBreaks))),
				entry("b", fabricMeta("b", "2.5")),
			},
			want: []string{"error:breaks:a"},
		},
		{
			name:   "breaks with placeholder version does not fire",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "b", "<2.0"), jarmeta.KindBreaks))),
				entry("b", fabricMeta("b", "${v}")),
			},
			want: []string{},
		},
		{
			name:   "conflicts is warning",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "b", "*"), jarmeta.KindConflicts))),
				entry("b", fabricMeta("b", "1.0")),
			},
			want: []string{"warning:conflicts:a"},
		},
		{
			name:   "breaks matches provided id",
			target: fabricTarget,
			entries: []Entry{
				entry("a", fabricMeta("a", "1.0", kind(dep("a", "legacy"), jarmeta.KindBreaks))),
				entry("b", func() jarmeta.LoaderMeta { b := fabricMeta("b", "1.0"); b.Provides = []string{"legacy"}; return b }()),
			},
			want: []string{"error:breaks:a"},
		},
		{
			name:   "breaks alternative group",
			target: Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderQuilt},
			entries: []Entry{
				entry("a", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "a", Mods: []jarmeta.Mod{{ID: "a"}},
					Dependencies: []jarmeta.Dependency{{Owner: "a", Kind: jarmeta.KindBreaks,
						AnyOf: []jarmeta.Dependency{kind(dep("a", "x"), jarmeta.KindBreaks), kind(dep("a", "b"), jarmeta.KindBreaks)}}}}),
				entry("b", jarmeta.LoaderMeta{Loader: jarmeta.LoaderQuilt, ModID: "b", Mods: []jarmeta.Mod{{ID: "b"}}}),
			},
			want: []string{"error:breaks:a"},
		},
		{
			name:   "duplicate mod id",
			target: fabricTarget,
			entries: []Entry{
				entry("one", fabricMeta("same", "1.0")),
				entry("two", fabricMeta("same", "1.1")),
			},
			want: []string{"error:duplicate_mod_id:one", "error:duplicate_mod_id:two"},
		},
		{
			name:    "same slug twice is not a duplicate",
			target:  fabricTarget,
			entries: []Entry{entry("x", fabricMeta("same", "1.0")), entry("x", fabricMeta("same", "1.0"))},
			want:    []string{},
		},
		{
			name:    "no metadata",
			target:  fabricTarget,
			entries: []Entry{{Slug: "lib", ParseErr: jarmeta.ErrNoMetadata}},
			want:    []string{"info:no_metadata:lib"},
		},
		{
			name:    "no metadata without error",
			target:  fabricTarget,
			entries: []Entry{{Slug: "lib"}},
			want:    []string{"info:no_metadata:lib"},
		},
		{
			name:    "wrapped no metadata",
			target:  fabricTarget,
			entries: []Entry{{Slug: "lib", ParseErr: fmt.Errorf("scan: %w", jarmeta.ErrNoMetadata)}},
			want:    []string{"info:no_metadata:lib"},
		},
		{
			name:    "parse error",
			target:  fabricTarget,
			entries: []Entry{{Slug: "bad", ParseErr: jarmeta.ErrMalformed}},
			want:    []string{"warning:parse_error:bad"},
		},
		{
			name:   "partial parse keeps checking metas",
			target: fabricTarget,
			entries: []Entry{{Slug: "m", ParseErr: errors.New("forge metadata malformed"),
				Metas: []jarmeta.LoaderMeta{fabricMeta("m", "1.0", dep("m", "minecraft", "1.19.x"))}}},
			want: []string{"error:mc_version_mismatch:m", "warning:parse_error:m"},
		},
		{
			name:   "client only mod on server side",
			target: fabricTarget,
			entries: []Entry{{Slug: "c", Side: core.ServerSide, Metas: []jarmeta.LoaderMeta{func() jarmeta.LoaderMeta {
				m := fabricMeta("c", "1.0")
				m.Environment = jarmeta.EnvClient
				return m
			}()}}},
			want: []string{"info:side_mismatch:c"},
		},
		{
			name:   "server only mod on client side",
			target: fabricTarget,
			entries: []Entry{{Slug: "s", Side: core.ClientSide, Metas: []jarmeta.LoaderMeta{func() jarmeta.LoaderMeta {
				m := fabricMeta("s", "1.0")
				m.Environment = jarmeta.EnvServer
				return m
			}()}}},
			want: []string{"info:side_mismatch:s"},
		},
		{
			name:   "client only mod on client side is fine",
			target: fabricTarget,
			entries: []Entry{{Slug: "c", Side: core.ClientSide, Metas: []jarmeta.LoaderMeta{func() jarmeta.LoaderMeta {
				m := fabricMeta("c", "1.0")
				m.Environment = jarmeta.EnvClient
				return m
			}()}}},
			want: []string{},
		},
		{name: "empty input", target: fabricTarget, entries: nil, want: []string{}},
		{
			name:   "odd input does not panic",
			target: Target{},
			entries: []Entry{
				{},
				{Slug: "z", Metas: []jarmeta.LoaderMeta{{}}},
				{Slug: "y", Metas: []jarmeta.LoaderMeta{{Loader: jarmeta.LoaderFabric, Dependencies: []jarmeta.Dependency{{Kind: jarmeta.KindDepends, Mandatory: true, AnyOf: []jarmeta.Dependency{}}}}}},
			},
			want: []string{"info:no_metadata:"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := codes(Check(tc.target, tc.entries))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("findings\n got: %v\nwant: %v", got, tc.want)
			}
		})
	}
}

func TestCheckOrderDeterministic(t *testing.T) {
	entries := []Entry{
		{Slug: "z-lib", ParseErr: jarmeta.ErrNoMetadata},
		entry("b", fabricMeta("b", "1.0", dep("b", "missing"), dep("b", "minecraft", "1.19.x"))),
		entry("a", fabricMeta("a", "1.0", dep("a", "missing"))),
		{Slug: "w", ParseErr: errors.New("boom")},
	}
	// the unreadable jar "w" may hold the missing mods, so those are warnings
	want := []string{
		"error:mc_version_mismatch:b",
		"warning:missing_dependency:a",
		"warning:missing_dependency:b",
		"warning:parse_error:w",
		"info:no_metadata:z-lib",
	}
	for i := 0; i < 20; i++ {
		got := codes(Check(fabricTarget, entries))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d\n got: %v\nwant: %v", i, got, want)
		}
		// reversed input gives the same output
		rev := []Entry{entries[3], entries[2], entries[1], entries[0]}
		if got := codes(Check(fabricTarget, rev)); !reflect.DeepEqual(got, want) {
			t.Fatalf("reversed run %d\n got: %v\nwant: %v", i, got, want)
		}
	}
}

func TestCheckNeverNil(t *testing.T) {
	if got := Check(Target{}, nil); got == nil || len(got) != 0 {
		t.Fatalf("Check(nil) = %#v, want empty non-nil", got)
	}
}

func TestFindingDetail(t *testing.T) {
	fs := Check(fabricTarget, []Entry{
		entry("m", fabricMeta("m", "1.0", dep("m", "lib", ">=2.0"))),
		entry("lib", fabricMeta("lib", "1.5.0")),
	})
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(fs), fs)
	}
	f := fs[0]
	want := map[string]string{"dependency": "lib", "required": ">=2.0", "found": "1.5.0"}
	if !reflect.DeepEqual(f.Detail, want) {
		t.Fatalf("detail = %v, want %v", f.Detail, want)
	}
	if f.Message == "" {
		t.Fatal("empty message")
	}
}

// TestCheckRealisticFabricPack is a small end-to-end fabric pack.
func TestCheckRealisticFabricPack(t *testing.T) {
	target := Target{MinecraftVersion: "1.20.1", Loader: jarmeta.LoaderFabric, LoaderVersion: "0.15.11"}

	fabricAPI := fabricMeta("fabric-api", "0.92.2+1.20.1",
		dep("fabric-api", "minecraft", "~1.20.1"),
		dep("fabric-api", "fabricloader", ">=0.14.21"),
		dep("fabric-api", "java", ">=17"))
	fabricAPI.Provides = []string{"fabric"}

	sodiumLike := fabricMeta("lithium", "0.11.2",
		dep("lithium", "minecraft", "1.20.x"),
		dep("lithium", "fabricloader", ">=0.14.0"),
		dep("lithium", "fabric-api", ">=0.93.0"))

	modmenu := fabricMeta("modmenu", "7.2.2",
		dep("modmenu", "minecraft", ">=1.20 <1.20.2"),
		dep("modmenu", "fabric-api", ">=0.80.0"),
		kind(dep("modmenu", "optifine"), jarmeta.KindBreaks))

	oldMod := fabricMeta("oldmod", "1.0.0",
		dep("oldmod", "minecraft", "1.19.x"))

	entries := []Entry{
		entry("oldmod", oldMod),
		entry("modmenu", modmenu),
		entry("lithium", sodiumLike),
		entry("fabric-api", fabricAPI),
		{Slug: "somelib", ParseErr: jarmeta.ErrNoMetadata},
	}
	got := Check(target, entries)

	type brief struct {
		Sev  Severity
		Code Code
		Slug string
	}
	var gotB []brief
	for _, f := range got {
		gotB = append(gotB, brief{f.Severity, f.Code, f.Slug})
	}
	want := []brief{
		{SeverityError, CodeDependencyVersion, "lithium"},
		{SeverityError, CodeMCVersionMismatch, "oldmod"},
		{SeverityInfo, CodeNoMetadata, "somelib"},
	}
	if !reflect.DeepEqual(gotB, want) {
		t.Fatalf("findings\n got: %v\nwant: %v", gotB, want)
	}
	if got[0].Detail["found"] != "0.92.2+1.20.1" || got[0].Detail["dependency"] != "fabric-api" {
		t.Fatalf("unexpected detail: %v", got[0].Detail)
	}
}

func TestCheckBundledMods(t *testing.T) {
	t.Run("bundled mod satisfies dependency", func(t *testing.T) {
		got := Check(fabricTarget, []Entry{
			entry("m", fabricMeta("m", "1.0", dep("m", "somelib", ">=1.0"))),
			entry("n", withBundled(fabricMeta("n", "1.0"), "somelib")),
		})
		assert.Empty(t, got)
	})
	t.Run("bundled version out of range is reported", func(t *testing.T) {
		got := Check(fabricTarget, []Entry{
			entry("m", fabricMeta("m", "1.0", dep("m", "somelib", ">=2.0"))),
			entry("n", withBundled(fabricMeta("n", "1.0"), "somelib")),
		})
		assert.Equal(t, []string{"error:dependency_version:m"}, codes(got))
	})
	t.Run("same lib bundled twice or alongside is not a duplicate", func(t *testing.T) {
		got := Check(fabricTarget, []Entry{
			entry("a", withBundled(fabricMeta("a", "1.0"), "lib")),
			entry("b", withBundled(fabricMeta("b", "1.0"), "lib")),
			entry("lib", fabricMeta("lib", "1.0")),
		})
		assert.Empty(t, got)
	})
	t.Run("fully opened nested jars do not downgrade a missing dependency", func(t *testing.T) {
		got := Check(fabricTarget, []Entry{
			entry("m", fabricMeta("m", "1.0", dep("m", "other"))),
			entry("n", withBundled(fabricMeta("n", "1.0"), "somelib")),
		})
		assert.Equal(t, []string{"error:missing_dependency:m"}, codes(got))
	})
}
