# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

`node-package-updater` (binary name: `npu`) — a Go CLI that upgrades `package.json` dependencies to their latest versions, much faster than `ncu`. Distributed as a Go binary, via `go install`, and as an npm package that shells out to a prebuilt binary.

- Module: `github.com/GNURub/node-package-updater`
- Go: **1.26.0** (see `go.mod`)
- Entrypoint: `cmd/npu/main.go` → calls `cmd.Exec()` (Cobra root in `cmd/npu.go`)

## Build & Run

```bash
# Run from source
go run ./cmd/npu -- <flags>

# Build local binary
go build -o npu ./cmd/npu

# Build with version injected (matches CI)
go build -ldflags="-s -w -X 'github.com/GNURub/node-package-updater/internal/version.Version=X.Y.Z'" -o npu ./cmd/npu/main.go
```

**Do NOT run `go build` as a verification step after edits.** Per user preference, never build after changes — rely on `go vet` / tests when verification is needed.

## Tests

```bash
go test ./...
go test ./pkg/checkdeps/... -run TestName
```

Only `pkg/checkdeps` currently has tests (`checkdeps_test.go`). Test patterns:

- Use `os.MkdirTemp` + cleanup closures (see `setupTestFiles`).
- Unexported functions are exercised via `reflect.ValueOf(fn).Call(...)` — keep this in mind before renaming/moving helpers, reflection breaks silently.

When adding Bubble Tea / TUI tests, follow `~/.claude/skills/go-testing/SKILL.md` (teatest patterns).

## Architecture

Standard Go layout with hard boundaries between `cmd/`, `internal/`, and `pkg/`.

```
cmd/
├── npu/main.go          # binary entry (prints upgrade banner, calls cmd.Exec)
├── npu.go               # root Cobra command + all flags
├── global.go            # `npu global`
├── checkdeps.go         # `npu checkdeps`
├── upgrade.go           # `npu upgrade` (self-update)
└── version.go           # `npu version`

internal/                # private packages (not importable)
├── cli/                 # Flags struct + ValidateFlags (shared by commands)
├── packagejson/         # load/parse/write package.json, workspaces, depth walking
├── dependency/          # fetch versions from npm registry (fasthttp), gob cache
├── packagemanager/      # npm/yarn/pnpm/bun/deno detection
├── cache/               # bitcask-backed on-disk cache
├── semver/              # semver comparison/range logic
├── updater/             # runs the actual package-manager install command
├── ui/                  # Bubble Tea components (spinner, selector, progress)
├── styles/              # lipgloss style definitions
├── gitignore/           # respects .gitignore when scanning
├── constants/           # repo owner/name, misc constants
└── version/             # Version var (overridden via -ldflags at build time)

pkg/                     # public packages (importable by external consumers)
├── upgrade/             # self-update logic (checks GitHub releases)
├── global/              # global deps update
├── checkdeps/           # unused-deps detection (uses esbuild as parser)
└── unused/              # (empty)
```

Key flow for the default `npu` command (`cmd/npu.go` → `Exec`):

1. Parse flags into `cli.Flags`.
2. Open bitcask cache via `cache.NewCache()`.
3. `packagejson.LoadPackageJSON(baseDir, options...)` — walks workspaces if enabled.
4. `pkg.ProcessDependencies(flags)` — fetches latest versions from the registry (respecting `--minor`, `--patch`, `--pre`, `--semanticVersion`, `--keepRange`), shows a Bubble Tea selector unless `--nonInteractive`, writes `package.json`, optionally runs installer.

### Gotchas

- **`--cpus` ≠ HTTP worker count.** The flag only sets `GOMAXPROCS`. The registry-fetch pool in `internal/updater.FetchNewVersions` is hardcoded to `2 * runtime.NumCPU()` (capped to `len(deps)`), regardless of `--cpus`.
- **Cache is in `$TMPDIR/.npu-cache`.** It's bitcask on disk + a `sync.Map` in memory (read-through). Because it lives in tmp, the OS can wipe it between reboots — that's by design. `-C/--cleanCache` calls `bitcask.DeleteAll()` _and_ resets the in-memory `sync.Map`.
- **Non-blocking progress channels.** `FetchNewVersions` sends on `processed` and `currentPackage` with `select { default: }`, so updates can be dropped if the UI consumer is slow. Don't rely on those channels for counting — use them only for progress display.
- **`npu upgrade` verify is shallow.** `verifyNewBinary` only checks the file exists and has exec perms — it does NOT actually run the new binary. The backup/restore flow (`.bak` via `os.Rename`) will still catch a failed move, but a binary that's corrupt-but-executable will slip through.
- **`package.json` key order.** Reads/writes go through `iancoleman/orderedmap` (`packagejson.go:393`) specifically to preserve user-authored key order. If you bypass this (e.g. decode into `map[string]any`), you'll silently reshuffle the file.
- **Unexported-fn tests via reflection.** `pkg/checkdeps/checkdeps_test.go` calls `findFilesToScan`, `findDependenciesInJSON`, `analyzePackageDependencies`, and `normalizePackageName` via `reflect.ValueOf(fn).Call(...)`. Renames/signature changes break at _runtime_, not compile time.

### Notable dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/{bubbletea,bubbles,lipgloss}` — TUI
- `github.com/valyala/fasthttp` — registry HTTP client (chosen for speed)
- `git.mills.io/prologic/bitcask` — embedded KV cache
- `github.com/evanw/esbuild` — used as a parser to detect imports in `checkdeps`
- `github.com/iancoleman/orderedmap` — preserves `package.json` key order on write

## Conventions

- **Commits**: conventional commits only (`feat:`, `fix:`, `chore:`, etc.), Spanish commit messages are the norm here (see `git log`). No `Co-Authored-By` trailers.
- **Error handling**: surface errors to the user via `styles.ErrorStyle.Render(err.Error())` and return — commands don't panic.
- **Flag additions**: add the field to `internal/cli/Flags`, register it in `cmd/npu.go:init()`, and validate in `Flags.ValidateFlags()` if it has constraints.
- **Editing `package.json`**: always go through `internal/packagejson` so the `orderedmap`-based writer preserves key order and formatting.

## Release & npm distribution

Pushing a tag matching `X.Y.Z` (no `v` prefix) triggers `.github/workflows/release.yml`, which cross-compiles for linux/darwin/windows × amd64/arm64 (excluding windows/arm64) and uploads binaries named `npu_<os>_<arch>` to the GitHub release.

The npm package has a tricky `postinstall` lifecycle:

1. `package.json` lists only `scripts/` in `files`, with `"postinstall": "node scripts/install.js"`.
2. `scripts/install.js` queries the GitHub releases API, downloads the correct `npu_<platform>_<arch>` binary into `scripts/`, and **writes `scripts/run.js` dynamically** with the binary name baked in.
3. `"bin": { "npu": "scripts/run.js" }` then points at that generated wrapper.

Consequence: `.gitignore` excludes `scripts/*` except `install.js`. Any `scripts/run.js` or `scripts/npu_linux_amd64` you see locally is build output from a prior `npm install` — **don't edit them, they're regenerated**.

## Tooling Notes

- Use `rg`/`fd`/`bat` over `grep`/`find`/`cat`.
- Never skip git hooks (`--no-verify`) or amend published commits.
