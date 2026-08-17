# Using packwiz-nxt as a library

`packwiz-nxt` is designed to be embedded, not just run as a CLI: `core`,
`fileio`, and `sources` never print to stdout, read global config, or panic on
bad input, so they're safe to call from your own program. This guide walks
through the common flows with real, minimal code. All snippets are elided
`package main` fragments (error handling shortened for readability) — see the
linked source files for full signatures.

For CLI-level detail (build flags, API key setup for local development) see
the top-level [README](../README.md).

## Installation

```shell
go get github.com/leocov-dev/packwiz-nxt
```

## Configuring API keys

CurseForge and GitHub require an API key; Modrinth does not. Keys are
process-wide and only need to be set once, before you make any calls that
need them:

```go
import "github.com/leocov-dev/packwiz-nxt/config"

func main() {
	config.SetCurseforgeApiKey(cfApiKeyBase64)
	config.SetGitHubApiKey(ghApiKey)
	// ... rest of your program
}
```

(`config/config.go`)

## Core concepts

- **`core.Pack`** (`core/pack.go`) — the in-memory modpack: name/author/version,
  Minecraft/loader versions, and a `Mods map[string]*core.Mod` keyed by slug.
  This is the type you build up and pass to most other functions.
- **`core.PackToml` / `core.ModToml`** — the on-disk `pack.toml` / `<mod>.toml`
  shapes. You usually don't touch these directly; `fileio` converts to/from
  `core.Pack` for you.
- **`core.Registry`** (`core/interfaces.go`) — holds the set of provider
  `Updater`s and `MetaDownloader`s (CurseForge/GitHub/Modrinth). A zero-value
  `Registry` is **not** usable — always get one via `core.NewRegistry()` or
  use the package-level `core.DefaultRegistry`.
  - Importing the `sources` package (which you need anyway to talk to a
    provider) automatically registers CurseForge/GitHub/Modrinth on
    `core.DefaultRegistry` via each provider's `init()`. So in a typical
    single-process program, just using `sources.*` functions and passing
    `nil`/`core.DefaultRegistry` wherever a `*core.Registry` is expected is
    enough — you don't need to build your own registry.
  - Build an isolated registry instead (e.g. for tests, or if you want
    per-request isolation) with:
    ```go
    reg := core.NewRegistry()
    sources.RegisterAll(reg) // sources/register.go
    ```

## Creating a new pack

```go
import "github.com/leocov-dev/packwiz-nxt/core"
import "github.com/leocov-dev/packwiz-nxt/fileio"

pack := core.NewPack(
	"My Modpack", // name
	"Alice",      // author
	"1.0.0",      // version
	"",           // description
	"1.21.1",     // mcVersion
	core.LoaderInfo{"fabric": "0.16.9"},
)

if err := fileio.WriteAll(*pack, "/path/to/pack/dir"); err != nil {
	// handle error
}
```

`fileio.WriteAll` writes `pack.toml`, `index.toml`, and every mod's `.toml`
file under `targetDir`. This is exactly what `packwiz init` does
(`cmd/init.go`).

## Loading an existing pack

```go
pack, err := fileio.LoadAll("/path/to/pack/dir/pack.toml")
if err != nil {
	// handle error
}
// pack is *core.Pack, ready to read/mutate
```

`fileio.LoadAll` (`fileio/packloader.go`) reads `pack.toml`, resolves and
loads `index.toml`, loads every referenced mod `.toml` file, and validates the
result — one call gets you a fully-populated `*core.Pack`.

## Adding a mod (Modrinth example)

Modrinth needs no API key, so it's the simplest provider to start with. The
same shape applies to CurseForge (`sources.CurseforgeNewMod`, needs
`config.SetCurseforgeApiKey` first) and GitHub (`sources.GitHubNewMod`, needs
`config.SetGitHubApiKey` for higher rate limits).

```go
import (
	"github.com/leocov-dev/packwiz-nxt/fileio"
	"github.com/leocov-dev/packwiz-nxt/sources"
)

project, err := sources.GetModrinthClient().Projects.Get("sodium")
if err != nil {
	// handle error
}

version, err := sources.ModrinthGetLatestVersion(*project.ID, *project.Title, *pack, "")
if err != nil {
	// handle error
}

mod, err := sources.ModrinthNewMod(project, version, "mods", pack.GetCompatibleLoaders(), "")
if err != nil {
	// handle error
}

pack.SetMod(mod) // attaches mod to pack.Mods, keyed by slug

if err := fileio.WriteAll(*pack, "/path/to/pack/dir"); err != nil {
	// handle error
}
```

This mirrors `internal/commands/cmdmodrinth/install.go`'s `installVersion`.
Optionally, resolve missing dependencies first and add them the same way:

```go
missing, err := sources.ModrinthFindMissingDependencies(version, *pack, "")
for _, dep := range missing {
	pack.SetMod(dep)
}
```

`sources.ModrinthNewMod` only builds the mod's metadata — it doesn't download
the file. See the next section to fetch the actual jar.

## Downloading mod files

`fileio.CreateDownloadSession` plans downloads (using the local cache to skip
files you already have) for a set of mods; `StartDownloads` executes them and
streams results back over a channel.

```go
import "context"

session, err := fileio.CreateDownloadSession(nil, pack.GetModsList(), []string{"sha512"})
if err != nil {
	// handle error
}

for dl := range session.StartDownloads(context.Background()) {
	if dl.Error != nil {
		// handle per-file error, dl.Mod identifies which mod failed
		continue
	}
	defer dl.File.Close()
	// dl.File is the downloaded file in the local cache; dl.Hashes["sha512"]
	// holds the hash you asked for
}

if err := session.SaveIndex(); err != nil {
	// handle error
}
```

Passing `nil` as the registry falls back to `core.DefaultRegistry`
(`fileio/download.go`). `hashesToObtain` (e.g. `"sha1"`, `"sha256"`,
`"sha512"`, `"length-bytes"`) controls which hashes get computed/recorded per
file.

## Checking and applying updates

```go
updates, err := core.GetUpdatableMods(nil, *pack) // nil -> core.DefaultRegistry
if err != nil {
	// handle error
}
for source, data := range updates {
	for _, mod := range data.Mods {
		fmt.Println(source, "has an update for", mod.Name)
	}
}

if err := core.UpdateAllMods(nil, *pack); err != nil {
	// handle error
}
if err := fileio.WriteAll(*pack, "/path/to/pack/dir"); err != nil {
	// handle error
}
```

Note `GetUpdatableMods`/`UpdateAllMods`/`UpdateSingleMod` (`core/update.go`)
take `core.Pack` **by value**, not `*core.Pack` — dereference your pack
pointer when calling them. `UpdateAllMods` mutates the mods in place (via
their pointers in `pack.Mods`); write the pack back out afterward to persist
the changes, mirroring `cmd/update.go`. Pinned mods (`mod.Pin == true`) are
skipped automatically.

## Building a pack without touching disk

If you're not managing a `pack.toml` on disk at all — e.g. storing pack/mod
state in your own database — you can skip `fileio` entirely and build
`core.Pack` by hand, then call straight into `sources.*`:

```go
pack := &core.Pack{
	Name:     "My Modpack",
	Versions: map[string]string{"minecraft": "1.21.1", "fabric": "0.16.9"},
	Mods:     map[string]*core.Mod{},
}

project, _ := sources.GetModrinthClient().Projects.Get("sodium")
version, _ := sources.ModrinthGetLatestVersion(*project.ID, *project.Title, *pack, "")
mod, _ := sources.ModrinthNewMod(project, version, "mods", pack.GetCompatibleLoaders(), "")
pack.SetMod(mod)

// persist `mod` fields into your own storage instead of calling fileio.WriteAll
```

This is the pattern `packwiz-web`'s backend uses: it builds a `core.Pack` from
database rows on demand, resolves mods via `sources.*`, and writes the
resulting `*core.Mod` fields into its own tables rather than to a `pack.toml`
file. `fileio.CreateDownloadSession`/`core.UpdateAllMods` still work the same
way against a hand-built `*core.Pack` — they don't require it to have come
from disk.

## Concurrency

- `core.Registry` is safe for concurrent use (internally mutex-guarded).
- `core.DefaultRegistry` is a single process-wide instance. If your program
  handles multiple independent packs/requests concurrently and needs
  isolation (e.g. different logger per request via `reg.SetLogger`), construct
  a separate `core.NewRegistry()` + `sources.RegisterAll(reg)` per logical
  caller instead of sharing `DefaultRegistry`.
