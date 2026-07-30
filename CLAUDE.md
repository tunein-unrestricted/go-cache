# CLAUDE.md

Guidance for working in this repo. Keep it short; update it when the architecture or workflow changes.

## What this is

`github.com/tunein/go-cache` — a goroutine-safe, generic, in-memory cache with TTL expiration, lazy loading, and duplicate-call suppression. Single package (`package cache`), no subpackages, zero non-test dependencies (testify is test-only). Public API lives behind the `Cacher[TKey, TValue]` interface in [interface.go](interface.go).

## Commands

```bash
go build ./...
go test -race ./...          # -race is mandatory; CI runs it and this code is concurrency-heavy
go test -cover ./...
golangci-lint run            # config in .golangci.yml; CI pins golangci-lint v2.11 (Go 1.24 mode)
go vet ./...
```

CI ([.github/workflows/workflow.yml](.github/workflows/workflow.yml)) runs build + `go test -race`, golangci-lint, and a Trivy scan on push/PR to `master`. Match it locally before pushing.

## Files

- [cache.go](cache.go) — `Cache` struct, all core operations, locking, background expiry sweep.
- [cacheitem.go](cacheitem.go) — `cacheItem` value type + `expired()`. TTL `<= 0` means never expire.
- [funcs.go](funcs.go) — `LoaderFunc` / `LoaderExpireFunc` / `AddedFunc` hook types and their fluent setters (return `*Cache` for chaining).
- [singlecall.go](singlecall.go) — `Group`, a singleflight-style duplicate-call suppressor used by the loader path.
- [interface.go](interface.go) — the `Cacher` interface (the public contract).

## Concurrency model — read before touching cache.go

This is the part that's easy to break. There are **two** mutexes with distinct jobs:

- `mtx sync.RWMutex` guards the `items` map and `cleanedAt`. Every read/write of `items` must hold it.
- `umtx sync.RWMutex` exists solely to make `Update`/`UpdateWithExpire` atomic (read-modify-write). `Update` takes `umtx.Lock()`; plain sets take `umtx.RLock()` via `setWithUpdateMutex`. Do not merge these two mutexes — `umtx` is a coarser lock layered on top of `mtx`.

Other invariants that already caused bugs (see git history) — preserve them:

- **Expiry-on-read deletes under a re-check.** `get` finds an expired item under `RLock`, then re-reads under `Lock` and only deletes if it's *still* expired — a concurrent `Set` may have refreshed it. Keep that double-check.
- **Cleanup sweep is triggered from `set`, not a ticker.** When a default TTL `>= 1ms` is set and a full TTL has elapsed since `cleanedAt`, `set` stamps `cleanedAt` inside the lock and spawns one `go deleteExpired()`. Stamping inside the lock is what prevents concurrent `Set`s from each spawning a sweep goroutine — don't move it out.
- **Loader calls go through `loadGroup.Do`** so concurrent misses for the same key collapse into one loader invocation; the rest wait and share the result. `Group.Do` currently always runs the loader synchronously — there is intentionally no goroutine-spawning path (a dead `isWait=false` branch was removed to fix a goroutine leak). Don't reintroduce one.
- Sentinel error is `ErrNotFound` (exported). Compare loader misses against it.

When changing anything here, add/adjust a test that fails under `-race` without the fix.

## Testing conventions

- **All tests are table-driven** and built on `github.com/stretchr/testify/suite`. Define a suite `struct`, register cases as a table, and run them via `suite.Run`. Match the existing style in [cache_test.go](cache_test.go).
- **One suite per logical unit** — a struct, a file, or a package. Don't spread a single type's tests across multiple suites, and don't mix unrelated units into one suite.
- Tests must pass under `go test -race` and comply with the linter config just like production code.

## Conventions

- Every `.go` file starts with the TuneIn copyright + Apache-2.0 header block. Copy it into new files.
- **Keep every file ≤ 500 lines.** Split by logical unit before it grows past that.
- All code (production and test) must comply with the existing [.golangci.yml](.golangci.yml) config — see limits below.
- Go 1.25 toolchain (`go.mod`), but golangci-lint runs in Go 1.24 compatibility mode.
- Lint config is strict (see [.golangci.yml](.golangci.yml)): funlen ≤100 lines/50 statements, gocyclo ≤20, `lll` 175 cols, gofumpt + goimports with `github.com/tunein/go-cache` as the local prefix. Prefer fixing over `//nolint` — `nolintlint` rejects unused directives and requires specific linter names.
- Keep the public surface in sync with the `Cacher` interface; note it also lists the unexported `get`, so the interface is package-internal-aware by design.
- Update [CHANGELOG.md](CHANGELOG.md) for user-visible changes.
