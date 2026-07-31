# Changelog

This changelog keeps track of changes made in Go Cache library.

## History

- [Changelog](#changelog)
  - [History](#history)
    - [v1.0.0](#v100)
    - [v0.9.2](#v092)
    - [v0.9.1](#v091)
    - [v0.9.0](#v090)
    - [v0.5.0](#v050)

### v1.0.0

 - Added a new `lru` package: a fast, generic, fixed-capacity LRU cache
 - Sharded design with one `sync.RWMutex` per shard (default 64 shards, configurable via `WithShards` in the range 1–1024) to minimize lock contention under concurrent access
 - Shard selection via `hash/maphash.Comparable`; each shard is an intrusive doubly-linked list (no `container/list`, no `interface{}` boxing) that reuses the evicted node on insertion
 - Optional `MetricsCollector` reporting get/set latency, hit/miss counts, evictions and the live item count
 - Convenient API: `Add`, `Get`, `Peek`, `Contains`, `Remove`, `Keys`, `Values`, `Len`, `Cap`, `Shards`, `Resize`, `Purge`, plus `WithMetrics` and `WithEvictionCallback` options
 - Eviction callbacks and metrics are always invoked outside the cache's internal locks, so callbacks may safely call back into the cache
 - `Purge` keeps `Len` consistent with the cache contents even when it races with concurrent `Add`/`Remove` calls
 - `Resize` keeps at least one entry of capacity per shard, so the effective capacity never drops below the shard count (`Cap` reports the applied value)

### v0.9.2

_Released 2026-05-19_

 - Removal of expired entries on `Set` once enough time has passed since the last cleanup
 - Fixed a potential goroutine leak via a dead-code path in `singlecall.go`

### v0.9.1

_Released 2025-12-12_

 - Fixed a data race
 - Restored the Go Report Card badge (broken by private-repo scanning)

### v0.9.0

_Released 2025-08-27_

 - First public release
 - Added TuneIn copyright headers to all Go files
 - Added `DEPENDENCIES.md` and contributing guidelines
 - Added the Trivy security scanner to CI and moved to a public runner
 - README improvements and license badge fix

### v0.5.0

_Released 2023-06-23_

 - More granular locking
 - Added separate mutex to ensure atomicity of Update operations
 - Added GHA

