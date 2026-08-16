# packwiz-nxt Rewrite Progress

Tracks `packwiz-nxt` against the original `packwiz` (mapped in
`../../vendor/PACKWIZ-MAP.md`). Goal of the rewrite, per the README: make
packwiz usable as a **library**, without forcing filesystem writes, while
keeping the original CLI mostly intact.

Last reviewed: 2026-08-16 (commit `8bf430e`).

## Snapshot

- `go build ./...` — passes.
- `go vet ./...` — clean.
- `go test ./...` — passes, but only `core` and `core/murmur2` have any test
  files. Checked-in `.coverage/coverage.out` shows **4.1%** total statement
  coverage (stale — regenerate with `make test` before trusting the number).
- CLI command tree, provider integrations (CurseForge/Modrinth/GitHub/URL),
  and export/import are present and structurally match the original almost
  package-for-package, with one command dropped (`serve`, see below).

## Architecture Changes vs. Original

| Original | packwiz-nxt | Notes |
|---|---|---|
| `core/` (model + hashing + download/cache + interfaces) | `core/` (model + hashing + interfaces only) | Filesystem-touching code moved out. |
| — | `fileio/` (new) | All disk I/O: pack/index/mod load+write, download/cache, gitignore, store paths. Mirrors old `core/download.go`, `core/index.go`, `core/pack.go`, `core/mod.go` file-side responsibilities. |
| `core/*.go` structs doubled as TOML (de)serialization | `core/*toml.go` (`packtoml.go`, `indextoml.go`, `modtoml.go`, `indexfiles.go`) vs. plain domain structs (`pack.go`, `mod.go`) | Explicit split between wire format (`PackToml`/`IndexFS`/`ModToml`) and in-memory domain model (`Pack`/`Mod`), with `FromPackAndModsMeta`/`AsPackToml`/`AsModToml` conversion functions. This is the core enabler of "library without FS writes" — callers can build/marshal a `Pack` and get bytes back (`MarshalResult`) without `fileio` ever touching disk. |
| `curseforge`, `modrinth`, `github` top-level packages (cmd + API client + updater all together) | Split into `sources/` (API clients + `*-ops.go` business logic + `*-updater.go`, no cobra/fs) and `internal/commands/cmd*` (cobra command wiring only, calls into `sources`) | Clean CLI/library separation — `sources` has zero cobra or file-write dependencies (confirm during any future audit; not exhaustively verified here). |
| `core.Updaters map[string]Updater` (direct map access) | `core.AddUpdater`/`core.GetUpdater` (registry functions over unexported `updaters` map), `Updater` interface gained `GetName() string` | Slightly more encapsulated; behavior equivalent. |
| `core.MetaDownloaders map[string]MetaDownloader` (direct map access) | Same — still a public raw map (`fileio/download.go:663`, `sources/cf-updater.go:20`) | **Inconsistent with the `Updater` registry refactor above** — not yet given the same `Add`/`Get` treatment. Minor cleanup item. |
| `core.ModLoaders` / `versionutil.go` | `core/versionutil.go` + `core/versionordering.go` (new) | Present, plus an added file for version-ordering logic split out. |
| CurseForge API key baked in (obfuscated) | No bundled key — must be supplied via `-ldflags -X main.CfApiKey=...` or `config.SetCurseforgeApiKey(...)` | Deliberate change (README), not a gap. Same pattern added for GitHub token (`config.SetGitHubApiKey`), which the original didn't need to abstract this way. |
| `cmd/serve.go` — local HTTP server + auto-refresh | **Absent** | See Gaps below. |

## Feature Parity Checklist

Legend: ✅ present/ported · ⚠️ present but worth a closer look · ❌ missing

### Root commands (`cmd/`)
- ✅ `init`
- ✅ `list`
- ✅ `pin` / `unpin`
- ✅ `refresh`
- ✅ `rehash`
- ✅ `remove`
- ✅ `update`
- ❌ `serve` / `server` — no equivalent anywhere in `packwiz-nxt`. Original
  `cmd/serve.go` ran a local HTTP server over the pack directory with
  auto-refresh-on-request and file-list restriction. Not started.

### CurseForge (`internal/commands/cmdcurseforge/` + `sources/cf-*.go`)
- ✅ `add`/`install`/`get` (`install.go`, `sources/cf-ops.go`)
- ✅ `export` (`export.go`)
- ✅ `import` (`import.go`) — HTTP-source import still stubbed
  (`shared.Exitln("HTTP not supported (yet)")`), same as upstream original
  (not a regression, just an inherited gap).
- ✅ `detect` (`detect.go`)
- ✅ `open`/`doc` (`open.go`)
- ✅ `packinterop` subpackage fully ported (disk/zip sources, manifest,
  minecraftinstance, translation)
- ✅ Windows Curse/Twitch dir detection (`cursedir_windows.go`/`cursedir_other.go`)
- ✅ `cf-updater.go` implements `core.Updater` + registers `MetaDownloaders["curseforge"]`

### Modrinth (`internal/commands/cmdmodrinth/` + `sources/mr-*.go`)
- ✅ `add`/`install`/`get` (`install.go`)
- ✅ `export` (`export.go`)
- ✅ `mr-updater.go` implements `core.Updater`
- ✅ `mr-pack.go` — `.mrpack`/`modrinth.index.json` schema structs

### GitHub (`internal/commands/cmdgithub/` + `sources/gh-*.go`)
- ✅ `add`/`install`/`get` (`install.go`)
- ✅ `gh-updater.go` implements `core.Updater`
- ✅ `gh-api.go`/`gh-ops.go`/`gh-interfaces.go`

### URL (`internal/commands/cmdurl/`)
- ✅ `add` (`install.go`) — no updater, matches original (URL mods aren't updatable)

### Migrate (`internal/commands/cmdmigrate/`)
- ✅ `migrate minecraft`
- ✅ `migrate loader`

### Settings (`internal/commands/cmdsettings/`)
- ✅ `settings acceptable-versions`/`av`

### Utils (`internal/commands/cmdutils/`)
- ✅ `utils markdown`/`md`

## Gaps / Open Items

1. **`serve` command is missing entirely.** Biggest functional gap vs. the
   original CLI. Needs a decision: port as-is, or design it as a
   library-friendly HTTP handler now that `fileio`/`core` are split (this
   could actually be a good validation case for the new architecture).
2. **`core.MetaDownloaders` wasn't migrated to the `Add`/`Get` registry
   pattern** used for `core.Updaters` — inconsistent API surface, easy
   follow-up cleanup (`core/interfaces.go`, `fileio/download.go:663`,
   `sources/cf-updater.go:20`).
3. **Test coverage is very thin.** Only `core` and `core/murmur2` have tests
   (ported from originals, plus `.testdata`/`.snapshots` fixtures). Nothing in
   `fileio`, `sources`, `cmd`, or any `internal/commands/cmd*` package has a
   single test. Given the whole point of the rewrite is to expose a stable
   library API, `fileio` (load/write) and `sources` (provider ops/updaters)
   are the highest-value places to add coverage before calling this stable.
4. **No documented/stable public library API yet.** `fileio.LoadAll` /
   `fileio.WriteAll` / `core.FromPackAndModsMeta` exist and look like the
   intended entry points, but there's no example, doc comment set, or
   `AGENTS.md`/library usage doc describing "how to use packwiz-nxt as a
   dependency" — only the CLI is exercised (`cmd/`, `main.go`). Worth writing
   once the API stabilizes.
5. Inherited (non-regression) TODOs worth tracking if picking up loose ends
   from upstream: CurseForge HTTP-source import (`import.go:39`), Forge
   "recommended version" resolution edge case (`cmdmigrate/loader.go:57`),
   multithreaded hashing on index load (`fileio/indexloader.go:53`), index
   housekeeping/LRU eviction (`fileio/download.go:702`). None of these are
   rewrite-introduced regressions — same state as upstream packwiz.
6. `bin/packa`, `bin/packb` under version control look like manual
   test/scratch fixtures (sample packs + a committed `packwiz` binary) —
   worth deciding whether these belong in `.gitignore` rather than the repo,
   independent of feature work.

## Suggested Next Milestones

1. Decide the fate of `serve` (port vs. redesign) and implement it.
2. Add unit tests for `fileio` (load/write round-trip) and `sources`
   (updater `ParseUpdate`/`CheckUpdate` logic, mockable via HTTP test
   servers) — these are the packages a library consumer will actually call
   into.
3. Unify `MetaDownloaders` under the same registry pattern as `Updaters`.
4. Write a minimal "using packwiz-nxt as a library" example/doc once (1) and
   (2) make the API trustworthy, and land it as `packwiz-nxt/AGENTS.md` or a
   `library/` example per this project's README-vs-AGENTS split convention.
