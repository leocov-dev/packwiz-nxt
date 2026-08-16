# packwiz-nxt Code Review

Full-codebase review focused on **code cleanliness, design, SOLID principles, and
Go idioms** — not a correctness/bug-hunt pass. Conducted by four parallel
sub-agent reviews, one per architectural layer (`core/`, `fileio/`, `sources/`,
CLI/`cmd`+`internal/commands`), then synthesized here.

Reviewed: 2026-08-16, commit `8bf430e`. Cross-reference:
[`rewrite-progress.md`](rewrite-progress.md) for the feature-parity angle,
[`../../vendor/PACKWIZ-MAP.md`](../../vendor/PACKWIZ-MAP.md) for the original
packwiz's architecture.

## Fix Pass Status (as of commit `068de40`)

A six-batch sub-agent fix pass addressed the High + Medium severity findings
below (Low severity — doc comments, naming nits, magic-number extraction —
was explicitly deferred, per plan). Status by section:

- **Cross-Cutting Themes #1 (global mutable state)** — ✅ Fixed. `core.Registry`
  (mutex-guarded) now unifies `Updaters`/`MetaDownloaders`; `core.defaultLoaderCache`
  is mutex-guarded. `core.DefaultRegistry` remains a process-wide default for
  CLI-behavior compatibility — full per-instance isolation is opt-in, not yet
  the default posture everywhere (see `rewrite-progress.md` gap #7).
- **#2 (printing instead of returning data)** — ✅ Fixed (second pass, below).
- **#3 (hidden viper/global-config coupling)** — ✅ Fixed. `core.ValidatePack`,
  `fileio.GetPackwizCache`, `fileio.RefreshIndexFiles` all take explicit
  parameters now instead of reading global `viper` state.
- **#4 (no context.Context/timeouts in network code)** — ✅ Fixed, pragmatically
  (second pass, below).
- **#5 (cross-provider duplication in `sources/`)** — ✅ Fixed. Dependency-BFS
  skeleton, loader-preference comparator shape, and dependency-override table
  unified across CurseForge/Modrinth (`sources/depresolve.go`,
  `sources/compare.go`, `sources/depoverride.go`). GitHub's intra-package
  asset-matching duplication also fixed.
- **#6 (inconsistent file-organization convention across providers)** — ✅ Fixed
  (second pass, below).
- **#7 (GitHub LSP violation)** — ✅ Fixed. `DoUpdate` now uses the exact
  asset `CheckUpdate` resolved via `ghCachedStateStore`, instead of
  re-deriving it with a weaker heuristic.
- **#8 (duplicated marshal/write boilerplate)** — ✅ Fixed. `core.marshalWithHash`
  and `fileio.writeMarshalled` consolidate both.
- **#9 (domain/wire-format duplication in `core`)** — ✅ Fixed (second pass, below).

Per-file/per-package findings below are **not** individually annotated with
fixed/deferred status — the summary above covers what changed. Notable
concrete bugs fixed beyond the cross-cutting list: `fileio/download.go`'s
file-handle/temp-file leaks and unsynchronized `cacheIndex` access;
`fileio/packexporter.go` (dead/broken, removed); `core.Pack.UpdateAll`/`.Update`/
`SortAscending` (dead code, removed); `core/indexfiles.go`'s panics (now
errors); `cmd/list.go`'s nil-slice bug (mods weren't printed without
`--side`); `cmd/update.go`'s always-empty success message; a second,
independently-discovered instance of the `GetAcceptableGameVersions` panic
on `PackToml` (Batch A only fixed `Pack`'s copy); business logic extracted
out of `cmdcurseforge/{detect,import,export}.go`, `cmdmodrinth/export.go`,
`cmdurl/install.go`, `cmdgithub/install.go`, and `cmdmigrate/minecraft.go`
(the last no longer calls another command's cobra `Run` field directly).

## Second Fix Pass Status (as of commit `208b514`)

A six-batch follow-up closed out everything the first pass deferred, except
the `serve` command gap (explicitly excluded — not a cleanup-batch item), test
coverage, and a library-usage doc (both intentionally out of scope for this
pass; see `rewrite-progress.md`'s Suggested Next Milestones).

- **#2 (printing instead of returning data)** — ✅ Fixed. New `core.Logger`
  interface (`Warnf`/`Infof`), defaulting to `core.PrintLogger` (stdout,
  preserving prior output). Wired into `core.Registry`, `cfApiClient`,
  `ghApiClient`, and a package-level `mrLogger` for the third-party Modrinth
  client. All progress/warning prints in `sources/{cf-ops,gh-api,gh-ops,mr-api,
  mr-ops}.go` and `core/update.go` now go through it. `GetForgeRecommended`
  also changed from a bare `string` return (swallowing 3 distinct failure
  modes into `""`) to `(string, error)`.
- **#4 (no context.Context/timeouts)** — ✅ Fixed, pragmatically. All three
  provider default clients plus `core.GetWithUA`'s client now carry
  `core.DefaultHTTPTimeout` (30s). `core.GetWithUAContext(ctx, ...)` was added
  and wired into `fileio/download.go`'s `downloadNewFile`, the one place a
  `context.Context` was already available but not plumbed through. Did **not**
  cascade `context.Context` through `cmd/` — no command handler holds one
  today, and doing so was judged disproportionate to a cleanup pass.
- **#6 (file-organization convention)** — ✅ Fixed for the one real offender.
  `sources/gh-interfaces.go` (zero actual interfaces, a grab-bag of model
  structs + misplaced ops/updater logic) renamed to `gh-models.go`; `fetchRepo`
  moved to `gh-ops.go`, `ghUpdateData.ToMap()` moved to `gh-updater.go`. Also
  added `GetGithubClient()` for symmetry with CF/MR. The three providers'
  broader api/ops/updater conventions still aren't identical (CF has
  detect/import, MR has export/pack) but that reflects genuinely different
  provider capabilities, not an unaddressed inconsistency.
- **#9 (domain/wire-format duplication)** — ✅ Fixed. New `core/packcommon.go`
  holds `Pack`/`PackToml`'s previously-duplicated `GetMCVersion`/
  `GetSupportedMCVersions`/`GetAcceptableGameVersions`/
  `SetAcceptableGameVersions`/`GetCompatibleLoaders` logic as shared free
  functions; both types now delegate. `PackToml`'s versions (defensive copy,
  handles raw-TOML `[]interface{}`) were the more-correct ones and are now the
  sole implementation. `Mod.GetUpdater()`/`ModToml.GetUpdater()` were also
  byte-identical copies, both hardwired to `DefaultRegistry` independently of
  the `*Registry` `core/update.go` threads through — a new `updaterFor()`
  helper backs both now (still targets `DefaultRegistry` only; documented as
  a known limitation rather than silently fixed halfway).

Also fixed along the way: `selectPreferredHash` (`fileio/download.go`) wasn't
`break`ing on match, silently keeping the *last* matching hash format instead
of the first; unescaped `mod.Name` in `cmdcurseforge/export.go`'s
`createModList` (now `html/template`, was raw string concatenation — an
XSS-shaped correctness issue, not just style); most remaining Low-severity
items (`interface{}`→`any`, `PreferredHashList` copy accessor, misnamed
`resolve.go`→`extensions.go`, descriptive regex names in `nameutil.go`,
`DefaultHashFormat` constant replacing scattered `"sha256"` literals,
`os.ModePerm`→`0755`, stray informal comments, `--config` flag's missing
`viper.BindPFlag`, duplicated "pick primary file" loops in `mr-updater.go`).

**Intentionally still deferred**: renaming `mrUpdateData`'s `mod-id`/`version`
TOML fields to `project-id`/`version-id` (matching `CfUpdateData`) — a
breaking on-disk pack-format change needing a migration path, not a
cleanup-batch item. Test coverage (still ~3.6%) and a documented public
library API remain open per `rewrite-progress.md`'s Suggested Next
Milestones. The `serve` command gap is out of scope by explicit decision, not
an oversight.

## Executive Summary

The rewrite has a real, mostly-working domain/wire-format split and a genuine
`Updater`/`MetaDownloader` extension point — the OCP story for adding a new mod
provider is sound. But the stated goal, "usable as a library, not just a CLI,"
is undermined by the same handful of problems recurring in **every layer**:

1. **Global mutable singletons with no synchronization** — `core.updaters`,
   `core.MetaDownloaders`, `core.defaultLoaderCache` are process-wide maps
   with no mutex and no way to scope per-instance. A library consumer can't
   run two isolated "sessions" in one process, and concurrent access is a
   data race.
2. **Direct `fmt.Println`/`fmt.Printf` deep inside library code** — not just
   in `cmd/` (expected) but in `core/packtoml.go`, `core/update.go`,
   `core/versionutil.go`, `sources/*-ops.go`, and even inside the raw
   `sources/gh-api.go` HTTP client. None of it is suppressible or
   redirectable by an embedding caller.
3. **Hidden dependency on global `viper` state** reaching from the CLI all the
   way into `core.ValidatePack` (`packtoml.go`) and `fileio` (`storeutil.go`,
   `indexloader.go`) — the single biggest concrete obstacle to embedding
   `core`/`fileio` outside the existing CLI.
4. **The exact same cross-cutting duplication that exists in the original
   packwiz was carried over, not fixed** — loader-preference/version-matching
   logic and dependency-resolution BFS loops are independently reimplemented
   per provider in `sources/`, same as upstream.
5. **Business logic still trapped in the CLI layer** in several commands
   (`cmdcurseforge/detect.go`, `cmdcurseforge/import.go`,
   `cmdmodrinth/export.go`, `cmdurl/install.go`) — the rewrite hasn't
   consistently pushed domain logic down into `core`/`fileio`/`sources` yet.
6. **Resource-leak and concurrency risk concentrated in `fileio/download.go`**
   — unclosed file handles on error paths, an unbounded goroutine that can
   leak forever if a caller stops draining its channel, and unsynchronized
   shared state between the download goroutine and `SaveIndex()`.
7. **A real LSP violation**: `github`'s `Updater.DoUpdate` silently re-derives
   which asset to install using a *different, weaker* heuristic than
   `CheckUpdate` used — a self-acknowledged bug in an inline comment
   (`gh-updater.go`) — while CurseForge's and Modrinth's implementations
   faithfully reuse the cached resolution. Same interface, different observable
   behavior depending which provider you're using.

None of this is catastrophic — the codebase builds, passes its (thin) test
suite, and is a genuine improvement in shape over the original's monolithic
packages. But it isn't yet a stable library API, and the gap is fixable with
targeted work rather than a redesign.

## Cross-Cutting Themes (read this section first)

### 1. Global mutable state (High)
| Where | What |
|---|---|
| `core/interfaces.go` | `updaters` (private map) + `MetaDownloaders` (exported map) — no mutex, inconsistent encapsulation (one has `Add`/`Get` accessors, the other is a raw exported map mutated directly by `sources/cf-updater.go`) |
| `core/versionutil.go` | `defaultLoaderCache` — private singleton mutated by `RefreshCache()`, read by `GetVersions()`, no synchronization |

All three are single, process-wide, unsynchronized. This blocks concurrent use
and per-test/per-instance isolation — arguably the single biggest structural
issue for the library goal.

### 2. Printing instead of returning data (High)
Found in `core/packtoml.go` (`ValidatePack`), `core/update.go`
(`BuildUpdateMap`, `GetUpdatableMods`, `UpdateSingleMod`),
`core/versionutil.go` (`GetForgeRecommended` swallows errors via `fmt.Println`
and returns `""`), `fileio/indexwriter.go` (`InitIndexFile`),
`fileio/indexloader.go` (`RefreshIndexFiles` drives an `mpb` progress bar
directly), `sources/cf-ops.go`, `sources/gh-api.go` (inside the raw HTTP
client — the worst instance), `sources/gh-ops.go`, `sources/mr-api.go`,
`sources/mr-ops.go` (which also *swallows* dependency-resolution errors by
printing and continuing instead of returning them, unlike CurseForge's
equivalent function which propagates errors properly).

None of these give a library caller a way to suppress, redirect, or
structurally inspect the output. Fix: return structured warnings/results, or
thread through an injectable logger/callback.

### 3. Hidden `viper`/global-config coupling (High)
`core/packtoml.go`'s `ValidatePack` calls `viper.MergeConfigMap` directly —
`core` mutating global CLI config state as a side effect of pack validation.
`fileio/storeutil.go`'s `GetPackwizCache` and `fileio/indexloader.go`'s
`RefreshIndexFiles` both read `viper.GetString(...)` directly. Any embedder
must pre-populate global viper state correctly before calling into what's
supposed to be a library API. Fix: accept config values as explicit
parameters; keep `viper` resolution in `cmd/`.

### 4. No `context.Context` / timeouts anywhere in network code (Medium-High)
`core/request.go`'s `GetWithUA`, all three `sources/*-api.go` HTTP clients
(`cfDefaultClient`, `ghDefaultClient`, `mrDefaultClient`), and
`fileio/download.go`'s download session all use `http.DefaultClient`/bare
requests with no timeout and no cancellation path. A hung remote server blocks
forever with no way for a caller to cancel — directly relevant to
`fileio/download.go`'s goroutine-leak finding below.

### 5. Cross-provider duplication carried over from upstream, not fixed (Medium-High)
Both `sources/cf-ops.go` and `sources/mr-ops.go` independently implement the
same BFS-with-cycle-limit dependency-resolution skeleton
(`maxCycles`/`mrMaxCycles`, both `= 20`, defined twice). Both
`sources/cf-updater.go` and `sources/mr-api.go` independently implement the
same "compare by MC version → loader preference → tiebreak" candidate-picking
algorithm. Both independently hardcode the same Fabric API/Fabric Language
Kotlin → Quilt-equivalent dependency-override table with different ID types.
This is the exact pattern flagged in the original packwiz's map
(`vendor/PACKWIZ-MAP.md` pattern #5/#6) — the rewrite was a chance to unify it
and didn't.

Within GitHub's own package it's worse: `sources/gh-ops.go`'s `installRelease`
and `sources/gh-updater.go`'s `CheckUpdate` duplicate the *same* asset-regex-matching
logic in the *same* package, with a comment in `gh-updater.go` literally
acknowledging it (`// yes, this is duplicated - i guess we should just cache
asset + tag instead of entire release...?`) — and the duplication is exactly
what causes the LSP violation below.

### 6. Inconsistent file-organization convention across the three provider packages (Medium)
There's no single rule for "which file holds URL-parsing/comparator/interface-impl
logic," and each of the three providers answers differently:
- **CurseForge**: `cf-api.go` = pure HTTP/JSON, `cf-ops.go` = business logic,
  `cf-updater.go` = interface impl **+** URL parsing **+** comparator logic **+**
  dependency-override table (does triple duty).
- **GitHub**: `gh-api.go` = HTTP/JSON, `gh-interfaces.go` = misnamed grab-bag
  (defines **zero interfaces** — it's actually struct definitions + ops logic +
  updater `ToMap()` boilerplate that didn't fit elsewhere), `gh-ops.go` =
  business logic, `gh-updater.go` = interface impl.
- **Modrinth**: `mr-api.go` = almost entirely business logic (URL parsing,
  loader comparison, side/hash resolution, dep-override) wrapping a thin
  third-party client, `mr-ops.go` = mod construction + dependency BFS,
  `mr-updater.go` = interface impl (the cleanest of the three).

A new contributor reading one provider cannot predict where equivalent logic
lives in another. Recommend picking one rule (e.g. api=HTTP/JSON only,
ops=all business/domain logic, updater=only interface-method bodies) and
normalizing all three — this also dissolves the `gh-interfaces.go` naming
confusion.

### 7. LSP violation: GitHub's `Updater` doesn't honor the CheckUpdate→DoUpdate contract (High)
`core.Updater.DoUpdate` is documented as applying the artifact resolved during
`CheckUpdate` (via `CachedState`). CurseForge (`cf-updater.go`) and Modrinth
(`mr-updater.go`) do this faithfully. GitHub's `DoUpdate`
(`sources/gh-updater.go`) ignores its cached `Release` and re-derives the
target asset with a different, looser heuristic (first `.jar`-suffixed asset,
vs. `CheckUpdate`'s regex match) — meaning swapping providers changes
observable update behavior in a way the interface contract doesn't warn about.
Fix: cache the resolved `Asset` itself (not the whole `Release`) in
`CachedState`, and have `DoUpdate` use it directly, mirroring CF/MR.

### 8. Duplicated marshal/write boilerplate (Medium)
- `core`: `ModToml.Marshal()` and `IndexTomlRepresentation.Marshal()` are
  near-identical (build `MarshalResult`, `toml.Marshal`, hash, update-hash) —
  extract a shared `marshalWithHash(v any, hashFormat string)` helper.
- `fileio`: `IndexWriter.Write`, `ModWriter.Write`, `PackWriter.Write` all do
  the identical `CreateFile → defer Close → Marshal() → Write bytes` sequence
  — one shared helper would remove ~30 duplicated lines across three files.
- `fileio`+`cmd`: the "ModWriter → LoadPackIndexFile → UpdateFileHashGiven →
  IndexWriter → RefreshIndexHash → PackWriter" 5-step write sequence is
  duplicated verbatim between `cmdgithub/install.go` and `cmdurl/install.go` —
  should be one `fileio` function owning this invariant, not two cmd-layer
  copies that must stay in lockstep.

### 9. Domain/wire-format duplication instead of a clean split (High, `core` only)
The stated design — `Pack`/`PackToml`, `Mod`/`ModToml` as domain-vs-wire pairs
— is undercut by near-total logic duplication between the pairs:
`GetMCVersion`, `GetSupportedMCVersions`, `GetAcceptableGameVersions`/`Set...`,
`GetCompatibleLoaders` are implemented almost identically on both `Pack` and
`PackToml`; `Mod.GetUpdater()`/`ModToml.GetUpdater()` are identical. If the
logic has to exist on both, the split isn't actually decoupling anything —
pick one owner (the domain type) and have the TOML type stay pure shape.

## Findings by Package

### `core/` — domain model, hashing, interfaces, versioning

**High**
- **Dead/broken public API**: `Pack.UpdateAll()` (`pack.go`) builds a map and
  discards it, always returns `nil` — a silent no-op with zero callers,
  superseded by `update.go`'s real `UpdateAllMods`. `Pack.Update(modSlug)`
  similarly duplicates `UpdateSingleMod` with zero callers. Remove both.
- **Unchecked type assertion can panic**: `Pack.GetAcceptableGameVersions()`
  does `acceptableVersions.([]string)` with no comma-ok check — a bad
  TOML/interface{} value panics the whole process instead of returning an
  error.
- **`panic()` for reachable failure modes**: `core/indexfiles.go`'s
  `indexFileMultipleAlias.MarkedFound()`/`.IsMetaFile()` and the "unknown type
  in IndexFiles" branches in `toMemoryRep`/`toTomlRep` all panic rather than
  return an error — these are invariants reachable through bugs elsewhere in
  the same package, not truly unreachable states.
- **Broken control flow**: `ModToml.SetMetaPath` (`modtoml.go`) sets the slug
  correctly in the `dotIndex == -1` branch, then unconditionally falls through
  to `m.SetSlug(filename[:dotIndex])` — with `dotIndex == -1` this is
  `filename[:-1]`, an invalid slice bound. Missing `else`/`return`.
- **`NewMod` has 11 positional parameters** including adjacent same-typed
  params (`slug, name, fileName string`; `pin, preserve bool`) with no
  compiler protection against transposition — a functional-options or
  struct-literal constructor would be safer for a public library API.
- **`IndexFile`/`indexFileMultipleAlias` duplicate their entire
  `IndexPathHolder` implementation** (5 methods, ~35 duplicated lines) to
  represent "one alias" vs. "N aliases" — a single type with an `[]string` of
  aliases would remove an entire type and interface.
- **`core` does direct, unabstracted network I/O**: `versionutil.go`'s
  `fetch*Versions` functions and `mcversion.go`'s `GetMinecraftVersions` call
  `GetWithUA`/`http.DefaultClient` directly against hardcoded URLs — makes
  `core` untestable without live network calls and uncancelable.
- **`GetForgeRecommended` swallows its own errors** — signature is
  `(mcVersion string) string` with no error return; on failure it prints and
  returns `""`, indistinguishable from "genuinely no recommendation exists."
- **Three separate, inconsistent version-comparison implementations coexist**:
  a hand-rolled dot-split comparator (`versionordering.go`), `Masterminds/semver`
  (used for pack-format validation), and `FlexVer` (used for MC/loader
  sorting) — different semantics, invites inconsistent sort behavior depending
  which call site you're reading.
- **Dead code**: `SortAscending` (`versionordering.go`) has zero callers.
  `ModToml.alias`/`.preserve` fields (`modtoml.go`) are declared but never
  read or written anywhere.

**Medium**
- `viper` and `fmt.Println` coupling in `packtoml.go`'s `ValidatePack` (see
  cross-cutting #2/#3).
- `IndexFS.packRoot` and pervasive `filepath.*` calls bake OS path semantics
  into what's meant to be a pure index/domain type — this resolution logic
  more naturally belongs in `fileio`, which already owns real I/O.
- `IndexTomlRepresentation.GetHashFormat()` hardcodes `"sha256"` and ignores
  the `hashFormat` field that `UpdateHash` dutifully writes a few lines away —
  dead/misleading state. `ModToml.GetHashFormat()` has the same hardcoding.
- `fetchMavenList`/`fetchMavenMap` (`versionutil.go`) duplicate ~12 lines of
  fetch+XML-decode boilerplate.
- `errors.New(fmt.Sprintf(...))` anti-pattern in `pack.go`'s `getIndexRepr` —
  should be `fmt.Errorf("...%w", err)`.
- Hash algorithms require editing `GetHashImpl`'s switch statement to extend
  (`hash.go`) — inconsistent with the OCP-friendly `Updater` registry pattern
  used elsewhere in the same package.
- `versionutil.go` is a 392-line god-file mixing loader-name registry, a
  generic slice-index utility, five bespoke Maven-XML fetchers, and a
  package-level cache — four different concerns with no internal grouping.

**Low** (representative, not exhaustive — full list in the raw sub-agent
transcripts if needed)
- `interface{}` used instead of `any` throughout `interfaces.go` despite Go
  1.23 — pure style nit.
- `PreferredHashList` is an exported mutable `[]string` — any caller can
  mutate the shared slice in place.
- Sparse doc comments on exported types/functions package-wide
  (`Mod`, `NewMod`, `ManualDownload`, `MarshalResult`, `HashableObject`,
  `ModLoaders`, most `fetch*` functions, etc.).
- `resolve.go` is 7 lines of extension constants with a name that doesn't
  match its content (no "resolve" logic present).
- Regexes in `nameutil.go` named `slugifyRegex1`…`slugifyRegex5` instead of
  descriptively.
- Typo: `"mod: %s is alreay up to date\n"` (`update.go`).

### `fileio/` — filesystem/download/cache layer

**High**
- **`CacheIndex.rehashFile` leaks a file handle** — `os.Open` result is used
  via `io.Copy` but never `.Close()`d on any path. Every rehash operation
  leaks an fd.
- **`downloadNewFile` leaks the temp file on multiple error paths** — created
  at the top of the function, never closed/removed on the `resp.StatusCode !=
  200`, `metaDownloaderData.DownloadFile()` error, or `teeHashes` failure
  branches. Needs a `defer`-based cleanup covering every early return.
- **No cancellation support in `StartDownloads`** — an unbuffered channel fed
  by a background goroutine with no `context.Context`. If a caller stops
  draining after the first error (a very natural pattern), the goroutine
  blocks forever on the channel send — a permanent goroutine leak with no way
  to cancel.
- **Unsynchronized shared state between the download goroutine and
  `SaveIndex()`** — the goroutine mutates `d.cacheIndex` maps/slices while
  `SaveIndex()` can be called concurrently from the main goroutine; nothing
  documents or enforces "drain the channel fully before calling SaveIndex."
  A real data-race risk baked into the public API contract.
- **`ExportPack` (`packexporter.go`) is dead, broken stub code** — never
  closes the output file or the zip writer (so any produced zip's central
  directory is never flushed = corrupt file), discards the one real error it
  captures and unconditionally `return nil`s, and never actually writes any
  pack/index/mod content into the archive. Confirmed zero callers anywhere in
  the repo. Either finish it (mirroring `packwriter.go`'s pattern) or delete
  it — as-is it's exported dead code masquerading as a working function.
- **`InitIndexFile` (`indexwriter.go`) prints directly to stdout** with no way
  to opt out, and doesn't even return whether a file was newly created vs.
  already existed.
- **`RefreshIndexFiles` (`indexloader.go`) drives an `mpb` terminal progress
  bar directly** — a library function producing terminal control codes with
  no callback/suppress mechanism; breaks embedding in anything that isn't the
  existing terminal CLI.
- Global `viper` coupling in `storeutil.go`/`indexloader.go` (cross-cutting
  #3) — the single biggest concrete obstacle to using `fileio` standalone.

**Medium**
- `CreateDownloadSession` (`download.go`, ~135 lines) is a god-function doing
  cache bootstrap, index migration, import-file processing, task planning, and
  metadata-downloader resolution all in one place.
- `RefreshIndexFiles` (~93 lines) similarly mixes path resolution, ignore-file
  reading, tree walking, progress UI, and hash updates.
- `Writable` interface (`writer.go`) requires an `UpdateHash` method that
  none of its three consumers (`IndexWriter`/`ModWriter`/`PackWriter`) ever
  call — an ISP violation; it's really an internal detail of `core`'s own
  marshaling, not of "write to disk."
- Write-boilerplate duplication across `indexwriter.go`/`modwriter.go`/
  `packwriter.go` (cross-cutting #8).
- `CacheIndex`/`CacheIndexHandle` are exported but have zero external
  callers anywhere in the repo — pure implementation detail leaking into the
  public surface; consider unexporting.
- `gitignore.go`'s `readGitignore` conflates "file missing" with "any read
  error" — a permission error or directory-instead-of-file is silently
  treated the same as "no `.packwizignore` exists," discarding the real
  error (a stale inline TODO already acknowledges this).
- Repeated silent-discard of `filepath.Abs` errors in `indexloader.go` (4
  call sites).
- `fileutil.go`'s `CreateFile` discards the real `MkdirAll` error when it
  fails, returning the stale original `os.Create` error instead.

**Low**
- `selectPreferredHash` doesn't `break` on match — keeps the *last* matching
  entry in iteration order rather than the first, which is surprising given
  the name.
- Non-idiomatic multi-line error strings with trailing `\n`/punctuation
  (`download.go` ×2).
- `"sha256"` as the default hash format is a bare string literal duplicated
  in at least 3 places instead of one shared constant.
- Deferred `f.Close()` errors ignored across all three writer files — if the
  flush fails, the caller never learns the file may be truncated.
- `os.ModePerm` (0777) used for created directories — 0755 is more
  conventional.
- Sparse doc comments on the package's main library entry points
  (`DownloadSession`, `CacheIndex`, `CreateDownloadSession`, etc.).

### `sources/` — CurseForge / GitHub / Modrinth provider integrations

**High**
- **`gh-interfaces.go` defines zero interfaces** — it's a mislabeled grab-bag
  of `Repo`/`Release`/`Asset` structs (an "api.go" concern), `fetchRepo` (an
  "ops.go" concern), and `ghUpdateData.ToMap()` (an "updater.go" concern).
  This is the clearest single SRP/naming finding in the package — see
  cross-cutting #6.
- **GitHub's asset-selection logic is duplicated within the same package**
  between `gh-ops.go`'s `installRelease` and `gh-updater.go`'s `CheckUpdate`
  — and the divergence between that and `DoUpdate`'s separate, weaker
  re-derivation is the LSP violation in cross-cutting #7.
- **`GetCurseforgeVersion` (`cf-updater.go`, ~75 lines) is a 30-branch
  cascading if/else hardcoding a Minecraft snapshot→release lookup table** —
  pure magic numbers (`year >= 22 && week >= 11`) with no structural
  documentation; should be a data table iterated in a loop.
- **`CurseforgeFindMissingDependencies` (cf-ops.go, ~90 lines) and
  `ModrinthFindMissingDependencies` (mr-ops.go, ~130 lines) are both
  god-functions** combining queue-building, network fetching, cycle-bounded
  BFS, and dependency-ID mapping in one place — see cross-cutting #5 for the
  duplication angle.
- **`fmt.Printf`/`fmt.Println` inside the raw GitHub HTTP client itself**
  (`gh-api.go`'s `makeGet`) — every single GitHub API call can print to
  stdout with zero suppression mechanism; the worst instance of the
  printing-in-library-code theme.
- **Error-handling contract differs between CF's and MR's "find missing
  dependencies"**: CurseForge propagates network failures as real errors;
  Modrinth's equivalent swallows them via `fmt.Printf` + `continue`. Same
  architectural role, different failure semantics — a real API-contract
  inconsistency for anyone integrating both providers generically.
- **`ParseAsParseAsFilename`** (`mr-api.go`) — an exported function name with
  a doubled "ParseAs" typo from copy-paste, a visible library-API defect.

**Medium**
- CurseForge API client (`cf-api.go`) never closes a single HTTP response
  body across 8 methods — a real resource leak, inconsistent with GitHub's
  `gh-interfaces.go`/`gh-ops.go` which do close bodies correctly, and even
  with CF's own `cf-updater.go` which closes correctly on one path — showing
  the pattern is known but not applied consistently.
- `CfFindLatestFile` (`cf-updater.go`, ~70 lines) duplicates its entire
  3-step comparator block twice (once per candidate source) instead of
  factoring into one shared comparator.
- `CheckUpdate` (`cf-updater.go`) decodes each mod's `CfUpdateData` twice —
  once to build a batch-fetch list, again after the fetch, discarding the
  first decode entirely.
- No `GetGithubClient()` accessor exists, unlike CF's `GetCurseforgeClient()`
  and MR's `GetModrinthClient()` — an unintentional asymmetry in the public
  surface (ISP angle: can a consumer swap/mock the GitHub client or not?).
- Four near-identical single-field URL-parsing functions in `mr-api.go`
  (`ParseAsModrinthSlug`/`Version`/`VersionID`/`Filename`) each re-run the
  full regex list, duplicating what `ParseModrinthSlugOrUrl` already computes
  in one pass — likely dead/redundant.
- `mrCompareLoaderLists` (`mr-api.go`) has an asymmetric comparator: the loop
  over list `a` guards `idx != -1` before comparing, the loop over list `b`
  does not — worth a maintainer's second look given how load-bearing loader
  preference is for mod resolution.
- Dependency-override "hardcoded quirk" tables (`MapDepOverride` in
  `cf-updater.go`, `mrMapDepOverride` in `mr-api.go`) implement the identical
  Fabric API/FLK→Quilt rule with the identical version-gate check
  copy-pasted verbatim, differing only in ID literals (cross-cutting #5).
- `fmt.Println("Finding dependencies...")` and friends inside `cf-ops.go`,
  `gh-ops.go`, `mr-ops.go`, and (worse, since it's nominally an "api" file)
  `mr-api.go`.

**Low**
- `noinspection GoUnusedConst` IDE-pragma comments checked into `cf-api.go`
  (6 occurrences) — editor tooling debris, not real lint suppressions.
- Inconsistent exported/unexported enum constants within single const blocks
  in `cf-api.go` (`DependencyTypeRequired` exported, siblings not).
- Magic literal `200` for HTTP status instead of `http.StatusOK`, repeated
  across `cf-api.go` and `gh-api.go`.
- No `http.Client.Timeout` or `context.Context` anywhere in any of the three
  provider API clients (cross-cutting #4).
- Duplicated "pick primary file" loop logic exists 3 times across
  `mr-ops.go`/`mr-updater.go` instead of calling the one already-extracted
  `GetModrinthVersionPrimaryFile` helper.
- Commented-out dead code referencing `viper` inside `mr-api.go`'s
  `mrGetProjectTypeFolder` — notable because `sources/` isn't supposed to be
  CLI-config-aware at all, even in a comment.
- `mrUpdateData`'s TOML field names (`mod-id`/`version`) are flagged via
  inline TODO as pending rename to `project-id`/`version-id` — meanwhile
  `CfUpdateData` already uses the "new" naming (`project-id`/`file-id`),
  meaning the two providers' on-disk schemas are asymmetric today.
- Naming convention differs per provider: CF mixes `Cf`/`Curseforge` prefixes
  inconsistently for exported names; GitHub mostly keeps helpers unexported
  (cleanest); Modrinth has the most internally consistent rule (`mr`=private,
  `Modrinth`=public).

### CLI / `cmd` + `internal/commands` + `internal/shared`

**High**
- **`packinterop.ReadMetadata` (used by `cmdcurseforge/import.go`) calls
  `shared.Exitf` three times** — a package meant to be reusable pack-import
  parsing logic can terminate the whole process, which is incompatible with
  it being called from anything other than this specific CLI command.
- **`cmdcurseforge/detect.go`'s `Run` (~90 lines) does real business logic
  inline**: walks the `mods/` directory, computes murmur hashes via free
  functions defined in the same cmd file, calls the CF fingerprint API, and
  constructs mods — a self-contained algorithm with zero CLI dependency that
  belongs in `sources`. (The file's own TODOs already acknowledge this:
  `// TODO: make all of this less bad and hardcoded`.)
- **`cmdcurseforge/export.go`'s and `cmdmodrinth/export.go`'s `Run` functions
  are 160–190 lines each**, mixing side-filtering (duplicated with
  `cmd/list.go`), manifest-metadata parsing, and five repeated manual
  zip-writer close-on-error blocks per file — a shared "with zip writer"
  helper would remove most of this.
- **`cmdcurseforge/import.go`'s `Run` is ~290 lines** — file/zip/directory
  detection, two-pass CurseForge API querying, per-mod creation, and raw
  filesystem override-copying all inline. The single worst offender for
  "business logic trapped in the cmd layer" found anywhere in the review.
- **`cmdmigrate/minecraft.go` invokes other commands' cobra `Run` fields
  directly** (`loaderCommand.Run(...)`, `packCmd.UpdateCmd.Run(...)`),
  including setting `viper.Set("update.all", true)` purely to make the
  borrowed `Run` behave — treats `Command.Run` as a callable library API,
  which it isn't designed to be, and forces `cmd.UpdateCmd` to stay exported
  just to support this.
- **`cmdurl/install.go` bypasses the shared download/cache machinery
  entirely**, hand-rolling its own HTTP-fetch-and-hash (`getHash`) instead of
  reusing `fileio`'s `DownloadSession` — the one command that's architecturally
  inconsistent with how every other provider downloads and hashes files.
- **`cmd/list.go`'s side-filtering has a real bug-shaped smell**: `mods` is
  declared as a nil slice and only populated inside an `if
  viper.IsSet("list.side")` block using in-place compaction that assumes
  `mods` is already populated — when the flag is unset, `mods` stays
  permanently nil. The correct in-place-compaction pattern (applied to an
  actually-initialized slice) exists correctly a few files away in
  `cmdcurseforge/export.go` — exactly the kind of duplication-instead-of-shared-helper
  this review flags.
- **`cmdsettings/acceptable_versions.go`'s `Run` (~80 lines) has three
  branches that duplicate ~15 lines of write+print logic each** — the
  mutation logic itself (pure list manipulation) belongs on `core.Pack` as a
  proper setter, mirroring how `SetAcceptableGameVersions` already exists.

**Medium**
- `cmd/init.go`'s `Run` + five helper functions total ~140 lines of
  interleaved prompting and domain validation (loader-version resolution
  against `core.ModLoaders`) — the validation logic should be a `core`
  function taking already-gathered strings, leaving only prompting in `cmd`.
- `cmd/remove.go` calls `os.Remove` directly on a mod's metadata file — raw
  filesystem mutation in the cmd layer instead of through `fileio`.
- The "ModWriter → IndexWriter → PackWriter" 5-step write sequence is
  duplicated verbatim between `cmdgithub/install.go` and `cmdurl/install.go`
  (cross-cutting #8).
- `SearchCurseforgeInternal` (`cmdcurseforge/install.go`) is exported but
  calls `shared.Exitln`/`Exitf` in five places — exporting it while embedding
  process-terminating calls is confusing; either unexport it or make it
  return errors.
- `internal/shared/downloadutil.go` mixes low-level zip-writing (`AddToZip`,
  business logic reused by two export commands) with high-level UX text
  (`PrintDisclaimer`) — arguably this zip-assembly logic belongs in `fileio`
  (which already owns `DownloadSession`) rather than in a "thin CLI helpers"
  package.
- `internal/shared/utils.go` bundles a domain-specific Forge/NeoForge
  version-string parser (`GetRawForgeVersion`) next to generic
  process-exit helpers — unrelated concerns in one file.
- A raw `panic(err)` in `cmdmodrinth/export.go` (line 92) is the only panic
  call found across the entire CLI layer — everywhere else uses
  `shared.Exitln`/`Exitf`; inconsistent and, unlike a returned error,
  uncatchable by an embedding caller.
- `cmd/update.go`: `singleUpdatedName` is declared but never assigned in the
  single-mod branch, so the final success message always prints an empty
  name.

**Low**
- `cmd/root.go`: the `--config` flag is declared but never bound with
  `viper.BindPFlag`, unlike every other flag in the same function.
- Several stray informal comments (`// ok this is epic`, `// Why is variable
  shadowing a thing!!!!`) that should be replaced with real explanations or
  removed.
- Commented-out code (`//shared.AddNonMetafileOverrides(...)`) left in both
  `cmdcurseforge/export.go` and `cmdmodrinth/export.go` instead of being
  removed or tracked.
- `createModList` (`cmdcurseforge/export.go`) builds HTML via raw string
  concatenation instead of `html/template`.
- Inconsistent exit-call style: `os.Exit(1)` used directly in
  `shared/downloadutil.go` and `cmdmigrate/loader.go` instead of the
  otherwise-consistent `shared.Exitln`/`Exitf`.

**Positive note**: the `shared.Exitln`/`Exitf` convention is applied
consistently across nearly the entire CLI layer — a genuine strength for a
CLI binary (just not for the packages that need to be embeddable, see above).
`cmdmodrinth/install.go`'s dispatch-to-named-helpers structure
(`installVersionById`/`installViaSearch`/`installProject`/`installVersion`)
is a good example of decomposition worth applying to `cmdcurseforge/import.go`
and `detect.go`. The `packinterop` interfaces (`ImportPackSource`/
`ImportPackMetadata`/`ImportPackFile`) are well-scoped and consistently
implemented by both the disk and zip backends — a clean ISP/LSP example.

## Suggested Priority Order

1. **Fix the resource leaks and goroutine/cancellation risk in
   `fileio/download.go`** — concrete correctness-adjacent bugs, not just
   style (leaked fds, leaked temp files, unbounded goroutine, unsynchronized
   shared state).
2. **Fix the GitHub `DoUpdate`/`CheckUpdate` LSP violation** — cache the
   resolved asset, not the whole release.
3. **Remove/finish `fileio/packexporter.go`** (dead, broken, unused) and the
   dead `core.Pack.UpdateAll`/`Pack.Update`/`SortAscending`/`ModToml`
   alias/preserve fields — cheap wins, no design decisions required.
4. **Break the global `viper` dependency out of `core`/`fileio`** — this is
   the biggest lever for the stated "usable as a library" goal; everything
   else is secondary to it.
5. **Replace the three unsynchronized global singletons** (`updaters`,
   `MetaDownloaders`, `defaultLoaderCache`) with either mutex-guarded access
   or, better, instance-scoped registries a caller constructs explicitly.
6. **Push the remaining business logic in `cmdcurseforge/{detect,import}.go`,
   `cmdmodrinth/export.go`, and `cmdurl/install.go` down into
   `sources`/`fileio`** — these are the last big holdouts of the "CLI is a
   thin adapter" architecture goal.
7. **Unify the cross-provider duplication in `sources/`** (dependency-BFS
   skeleton, loader-preference comparator shape, dep-override table) behind
   shared helpers — reduces the maintenance burden of three independently
   drifting copies.
8. Everything else in this document (printing-instead-of-returning,
   marshal/write boilerplate, file-organization consistency, doc comments) is
   lower-urgency cleanup that can be picked up incrementally.
