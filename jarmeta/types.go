// Package jarmeta reads the loader metadata embedded in a mod jar
// (fabric.mod.json, quilt.mod.json, META-INF/neoforge.mods.toml and
// META-INF/mods.toml) without extracting anything else from the archive.
//
// The package is pure: no network, no filesystem, no printing. Version ranges
// are preserved exactly as declared; evaluating them is the caller's job.
package jarmeta

import "strings"

// Loader identifies a mod loader. Values match the loader names used by
// packwiz packs ("fabric", "quilt", "forge", "neoforge").
type Loader string

// Known loaders.
const (
	LoaderFabric   Loader = "fabric"
	LoaderQuilt    Loader = "quilt"
	LoaderForge    Loader = "forge"
	LoaderNeoForge Loader = "neoforge"
)

// Environment is the side(s) a mod declares it runs on.
type Environment string

// Declared environments.
const (
	EnvBoth   Environment = "both"
	EnvClient Environment = "client"
	EnvServer Environment = "server"
)

// Side is the per-dependency side declared by Forge/NeoForge.
type Side string

// Dependency sides. SideBoth is also used when no side is declared.
const (
	SideBoth   Side = "BOTH"
	SideClient Side = "CLIENT"
	SideServer Side = "SERVER"
)

// DependencyKind classifies a dependency relation.
type DependencyKind string

// Dependency kinds.
const (
	// KindDepends is a requirement (see Dependency.Mandatory).
	KindDepends DependencyKind = "depends"
	// KindRecommends is a soft recommendation (Fabric only).
	KindRecommends DependencyKind = "recommends"
	// KindSuggests is a soft suggestion (Fabric only).
	KindSuggests DependencyKind = "suggests"
	// KindBreaks marks versions that must not be loaded together with this mod.
	KindBreaks DependencyKind = "breaks"
	// KindConflicts marks versions that are discouraged together with this mod.
	KindConflicts DependencyKind = "conflicts"
)

// Well-known dependency ids that phase-2 consumers interpret specially.
const (
	IDMinecraft     = "minecraft"
	IDFabricLoader  = "fabricloader"
	IDQuiltLoader   = "quilt_loader"
	IDForge         = "forge"
	IDNeoForge      = "neoforge"
	IDJava          = "java"
	placeholderMark = "${"
)

// Mod is one mod declared in a metadata file. Fabric and Quilt declare one;
// Forge and NeoForge may declare several [[mods]] tables.
type Mod struct {
	ID      string
	Version string
	// VersionIsPlaceholder is true when Version is an unexpanded build
	// placeholder such as "${file.jarVersion}". Version is kept as declared.
	VersionIsPlaceholder bool
}

// Dependency is one declared relation from a mod to another mod id.
// Ranges are raw strings as written in the metadata; they are not validated.
type Dependency struct {
	// Owner is the id of the mod that declares this dependency.
	Owner string
	// ID is the target mod id. Special ids (minecraft, fabricloader, java...)
	// are ordinary dependencies.
	ID string
	// Kind is the relation type.
	Kind DependencyKind
	// VersionRanges are alternatives: the dependency is satisfied if ANY
	// matches. Empty means no range was declared.
	VersionRanges []string
	// VersionRangesAll are ranges that must ALL match (Quilt "all" form).
	// Usually empty.
	VersionRangesAll []string
	// Mandatory is only meaningful for KindDepends: false means the
	// dependency is optional.
	Mandatory bool
	// Side is the Forge/NeoForge per-dependency side; SideBoth otherwise.
	Side Side
}

// LoaderMeta is the metadata one loader's file declares for a jar.
type LoaderMeta struct {
	Loader Loader
	// ModID is the primary mod id (first declared mod).
	ModID string
	// Version is the primary mod's version as declared.
	Version string
	// VersionIsPlaceholder reports whether Version is an unexpanded placeholder.
	VersionIsPlaceholder bool
	// Name is the human display name, if declared.
	Name string
	// Mods lists every declared mod, primary first. Always non-empty.
	Mods []Mod
	// Provides are extra ids the primary mod also answers to.
	Provides []string
	// Dependencies of every declared mod, in a deterministic order.
	Dependencies []Dependency
	// Environment is the declared side of the whole mod.
	Environment Environment
	// ModLoader is the Forge/NeoForge language loader ("javafml"...); empty otherwise.
	ModLoader string
	// LoaderVersionRange is the Forge/NeoForge "loaderVersion" range; empty otherwise.
	LoaderVersionRange string
}

// FindDependency returns the first dependency with the given id and kind.
func (m LoaderMeta) FindDependency(id string, kind DependencyKind) (Dependency, bool) {
	for _, d := range m.Dependencies {
		if d.ID == id && d.Kind == kind {
			return d, true
		}
	}
	return Dependency{}, false
}

// MinecraftDependency returns the "minecraft" requirement, if declared.
func (m LoaderMeta) MinecraftDependency() (Dependency, bool) {
	return m.FindDependency(IDMinecraft, KindDepends)
}

// MinecraftRanges returns the alternative Minecraft version ranges the mod
// declares, and whether a minecraft requirement exists at all.
func (m LoaderMeta) MinecraftRanges() ([]string, bool) {
	d, ok := m.MinecraftDependency()
	return d.VersionRanges, ok
}

// LoaderDependency returns the requirement on the loader itself
// (fabricloader, quilt_loader, forge or neoforge), if declared.
func (m LoaderMeta) LoaderDependency() (Dependency, bool) {
	var id string
	switch m.Loader {
	case LoaderFabric:
		id = IDFabricLoader
	case LoaderQuilt:
		id = IDQuiltLoader
	case LoaderForge:
		id = IDForge
	case LoaderNeoForge:
		id = IDNeoForge
	}
	return m.FindDependency(id, KindDepends)
}

func isPlaceholder(version string) bool {
	return strings.Contains(version, placeholderMark)
}

func newMod(id, version string) Mod {
	return Mod{ID: id, Version: version, VersionIsPlaceholder: isPlaceholder(version)}
}
