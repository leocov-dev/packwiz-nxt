# AGENTS.md — packwiz-nxt

Guidance for anyone (human or agent) writing Go code in this module. `packwiz-nxt`
is a **library first, CLI second**: `core/`, `fileio/`, and `sources/` must be
safely embeddable by a caller that is not `cmd/`. Every rule below exists to
protect that property. See `.plan/rewrite-progress.md` for the feature-parity
backlog.

## 0. Out of scope

- **No `serve` command.** The original packwiz has a `serve` command; this is a
  deliberate exclusion in packwiz-nxt, not an oversight or a TODO. Don't add
  it, stub it, or reference it as planned/future work.

## 1. Library-first design

- **No global mutable state.** Don't add new package-level `var` maps/slices
  that are mutated at runtime. If you need a registry or cache, model it as a
  struct with a constructor (`NewXxx()`) and mutex-guarded methods, the way
  `core.Registry` does in `core/interfaces.go`. A package-level
  `DefaultXxx` instance may exist for CLI convenience, but the isolated,
  per-instance type must be the primary API.
- **No hidden config dependencies.** Never read `viper.Get*` or any other
  global config source from `core/`, `fileio/`, or `sources/`. Accept
  configuration as explicit function parameters or struct fields; resolve
  `viper`/flags/env only in `cmd/` and `internal/commands`, then pass plain
  values down.
- **Never print from library code.** `fmt.Println`/`fmt.Printf`/progress bars
  inside `core/`, `fileio/`, or `sources/` prevent an embedder from
  suppressing or redirecting output, and make errors invisible to callers.
  Instead:
  - Return the result/error and let the caller decide how to present it.
  - If a long-running operation needs progress reporting, thread through an
    injectable callback or small interface (e.g. `func(update string)`), never
    call a concrete printer/progress-bar library directly.
  `cmd/` and `internal/commands` are the only places allowed to write to
  stdout/stderr or drive a UI.
- **Business logic lives in `core`/`fileio`/`sources`, not in `cmd/`.** A
  `cmd/*.go` or `internal/commands/*.go` file should parse flags, call into a
  library function, and format the result. If you find yourself writing
  multi-step logic, branching, or parsing inside a command file, extract it
  into the appropriate library package and give it a table-driven test.
- **Network code must accept a `context.Context` and use a client with a
  timeout.** No bare `http.DefaultClient` or context-less requests in new
  code; follow `fileio.DownloadSession.StartDownloads`'s pattern of accepting
  a `context.Context` for cancellation.

## 2. SOLID in this codebase

- **SRP**: one file, one responsibility. Don't let an "updater" file also own
  URL parsing, comparator logic, and a dependency-override table (see the
  `sources/cf-updater.go` critique in the code review) — split by
  responsibility, not just by provider.
- **OCP**: new mod providers/sources should be addable by implementing
  `core.Updater`/`core.MetaDownloader` and registering with a `Registry`, not
  by editing existing provider code or switch statements over provider name.
- **LSP**: every `Updater` implementation must honor the interface contract
  identically. In particular, `DoUpdate` must act on the exact resolution
  produced by the preceding `CheckUpdate` (via `UpdateCheck.CachedState`),
  never re-derive it with different logic — this was a real bug (GitHub's
  updater) and is exactly the kind of behavioral divergence LSP forbids.
- **ISP**: keep interfaces (`Updater`, `MetaDownloader`, `HashableObject`,
  etc.) small and focused on what callers actually need; don't grow a single
  interface to serve every provider's internal needs.
- **DIP**: `core`/`fileio` should depend on interfaces (`Updater`,
  `MetaDownloader`) that `sources/*` implements, not the other way around.
  Avoid `core`/`fileio` importing concrete types from `sources/`.

## 3. Avoid duplication across providers

`sources/` has three provider packages (CurseForge, GitHub, Modrinth) that
tend to reimplement the same shape of logic independently (dependency-BFS
with a cycle limit, "compare by MC version → loader preference → tiebreak"
candidate picking, Fabric/Quilt dependency-override tables). When adding or
touching provider logic:

- Check whether the same shape of logic already exists in another provider
  (`sources/depresolve.go`, `sources/compare.go`, `sources/depoverride.go`
  hold the already-unified pieces) before writing a new independent copy.
- If you must implement something per-provider, prefer extending the shared
  helper over copy-pasting and tweaking.

### File-organization convention per provider

Until the three providers are normalized, follow this rule for **new**
code so we stop diverging further:
- `*-api.go`: pure HTTP/JSON — request/response shapes and the raw client
  call only. No business logic.
- `*-ops.go`: business/domain logic (mod construction, dependency
  resolution, side/hash resolution).
- `*-updater.go`: only the `core.Updater`/`core.MetaDownloader` interface
  method bodies — delegate to `*-ops.go` for actual logic.

## 4. Go idioms

- Run `make fmt` (gofmt) before committing; `make lint` must be clean.
- Errors: use `fmt.Errorf("doing X: %w", err)` to wrap and preserve the chain
  when propagating; use `errors.New("static message")` only for a fixed,
  non-wrapped message. Error strings are lowercase, no trailing punctuation,
  no capitalized first word (standard Go convention) — some existing code
  violates this; don't copy it in new code.
- Return errors, don't panic, in any code reachable from a library entry
  point (`core`, `fileio`, `sources`). Reserve `panic` for truly unrecoverable
  programmer errors, and never as a substitute for error handling on data
  that came from a file, network response, or user input.
- Accept interfaces, return concrete types where practical; keep exported
  interfaces minimal (see ISP above).
- Use table-driven tests (see `core/*_test.go`, `.testdata/`, `.snapshots/`
  for existing examples) for new logic in `core`/`fileio`/`sources`.
  Coverage in this module is currently thin (~3.6%) — new code should not
  make that worse; add tests alongside new logic rather than after the fact.
- Guard any shared/mutable state with a `sync.Mutex`/`sync.RWMutex`, following
  the `core.Registry` pattern, and document in the type's doc comment that a
  zero value is/isn't usable.
- Prefer explicit constructors (`NewXxx`) over exported structs that require
  the caller to know which fields must be initialized.
- Keep doc comments on all exported identifiers, starting with the
  identifier's name, per standard Go convention (`// Registry holds ...`).

## 5. Before submitting a change

1. `make fmt` then `make lint` (gofmt clean).
2. `make test` — all existing tests must still pass; add new tests for new
   behavior per the strategy above.
3. Re-read your diff against sections 1–3 above: did you add global state,
   print from library code, read `viper` outside `cmd/`, or duplicate
   provider logic that already exists in `sources/depresolve.go`,
   `sources/compare.go`, or `sources/depoverride.go`?
