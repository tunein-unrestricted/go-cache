# Changelog

This changelog keeps track of changes made in Go Cache library.

## History

- [Changelog](#changelog)
  - [History](#history)
    - [v1.0.2](#v102)
    - [v1.0.1](#v101)
    - [v1.0.0](#v100)
    - [v0.9.2](#v092)
    - [v0.9.1](#v091)
    - [v0.9.0](#v090)
    - [v0.5.0](#v050)

### v1.0.2

 - `lru`: the default shard count now depends on the cache size (a power of two up to `DefaultShards`, with at least 256 entries per shard) instead of always being 64. Small and mid-size caches no longer evict while far from full: a 128-entry cache now keeps all 128 distinct keys instead of about 72%, and a 1,024-entry cache keeps about 98% instead of 89%. `WithShards` still overrides the default. Trade-off: caches below 16,384 entries now get fewer locks, so they scale worse under parallel load. Parallel `Get`/`Add` on 16 cores (Apple M4 Max) takes about 175 ns/op instead of 28 ns/op at 256 entries, 155 ns/op instead of 29 ns/op at 1,024 entries and 60 ns/op instead of 29 ns/op at 4,096 entries, and is unchanged from 16,384 entries up. Callers that value throughput over hit rate on such caches can restore the old behavior with `WithShards(64)`
 - `lru`: each shard now counts its own entries, so concurrent writes no longer contend on one shared counter (parallel Add+Remove is about 2.9x faster with 8 goroutines). With a `MetricsCollector` set, the shared counter is kept so `SetItemCount` and `Len` stay O(1); without one, `Len` sums the shard counts

### v1.0.1

 - Fixed a goroutine leak when a loader panics: later `Get` calls for that key no longer block forever, and callers waiting on the panicking load receive an error
 - Fixed a data race between `Purge` and in-flight loader calls: `Purge` no longer replaces the duplicate-call suppression group, which could corrupt its mutex and run the loader twice for the same key
 - Updated minimum Go version to 1.27
 - Fixed `lru`: `Len` could briefly report a negative count, and `Keys`/`Values` could panic, when `Add` and `Remove` of the same key ran on different goroutines

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

