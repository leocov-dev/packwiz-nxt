package compat

import (
	"cmp"
	"slices"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

// Severity ranks how serious a finding is.
type Severity string

// Severities, from most to least serious.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

func (s Severity) rank() int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	}
	return 2
}

// Code identifies the kind of a finding.
type Code string

// Finding codes.
const (
	// CodeWrongLoader (error): the jar has no metadata for the pack loader.
	// Quilt packs also accept Fabric metadata.
	CodeWrongLoader Code = "wrong_loader"
	// CodeMCVersionMismatch (error): the jar's minecraft requirement excludes
	// the pack Minecraft version.
	CodeMCVersionMismatch Code = "mc_version_mismatch"
	// CodeLoaderVersionMismatch (error): the jar's requirement on the loader
	// excludes the pack loader version. Skipped when Target.LoaderVersion is empty.
	CodeLoaderVersionMismatch Code = "loader_version_mismatch"
	// CodeMissingDependency (error): a mandatory dependency is not provided
	// by any entry of the pack.
	CodeMissingDependency Code = "missing_dependency"
	// CodeDependencyVersion (error): a mandatory dependency is present but
	// its version does not satisfy the declared range.
	CodeDependencyVersion Code = "dependency_version"
	// CodeBreaks (error): another entry matches a "breaks" declaration.
	CodeBreaks Code = "breaks"
	// CodeConflicts (warning): another entry matches a "conflicts" declaration.
	CodeConflicts Code = "conflicts"
	// CodeDuplicateModID (error): two entries provide the same mod id.
	CodeDuplicateModID Code = "duplicate_mod_id"
	// CodeNoMetadata (info): the jar has no loader metadata (common for
	// libraries and coremods).
	CodeNoMetadata Code = "no_metadata"
	// CodeParseError (warning): the jar or its metadata could not be (fully) read.
	CodeParseError Code = "parse_error"
	// CodeClientOnlyOnServer (info): the jar declares an environment that
	// does not match the pack side of the entry.
	CodeClientOnlyOnServer Code = "client_only_on_server"
	// CodeUnevaluableRange (info): a declared version range could not be
	// parsed, so the check depending on it was skipped.
	CodeUnevaluableRange Code = "unevaluable_range"
)

// Target is what the pack runs on.
type Target struct {
	MinecraftVersion string
	Loader           jarmeta.Loader
	// LoaderVersion may be empty, which skips loader version checks. Use the
	// bare loader version ("0.15.11", "47.2.0", "21.1.172").
	LoaderVersion string
}

// Entry is one mod of the pack together with its parsed jar metadata.
type Entry struct {
	// Slug is the packwiz mod identity used in findings.
	Slug string
	// Side is the pack side of the mod; empty means both.
	Side core.ModSide
	// Metas is one LoaderMeta per loader found in the jar (may be empty).
	Metas []jarmeta.LoaderMeta
	// ParseErr is non-nil if the jar could not be (fully) parsed. It is also
	// set, to jarmeta.ErrNoMetadata, when Metas is empty for lack of metadata.
	ParseErr error
}

// Finding is one informational result about a pack entry.
type Finding struct {
	Severity Severity          `json:"severity"`
	Code     Code              `json:"code"`
	Slug     string            `json:"slug"`
	Message  string            `json:"message"`
	Detail   map[string]string `json:"detail,omitempty"`
}

// sortFindings orders by severity (errors first), slug, code, message.
func sortFindings(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.Severity.rank(), b.Severity.rank()),
			cmp.Compare(a.Slug, b.Slug),
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Message, b.Message),
		)
	})
}
