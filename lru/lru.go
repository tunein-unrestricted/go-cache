// Package lru provides a fast, generic, fixed-capacity LRU cache with optional metrics collection.
//
//	Copyright 2026 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package lru

import (
	"errors"
	"hash/maphash"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultShards is the number of shards a Cache uses when WithShards is not set.
const DefaultShards = 64

const (
	minShards = 1
	maxShards = 1024
)

// ErrInvalidSize is returned by New when the requested capacity is not positive.
var ErrInvalidSize = errors.New("lru: cache size must be positive")

// ErrInvalidShardCount is returned by New when the requested shard count is
// outside the range [1, 1024].
var ErrInvalidShardCount = errors.New("lru: shard count must be between 1 and 1024")

type kv[K comparable, V any] struct {
	key K
	val V
}

type addOutcome[K comparable, V any] struct {
	evicted   bool
	evictKey  K
	evictVal  V
	sizeDelta int
}

// Cache is a fixed-capacity, goroutine-safe LRU cache. Its entries are spread
// across independently locked shards so that concurrent operations on different
// keys rarely contend. Recency is tracked per shard, so eviction is local to the
// shard a key maps to rather than global across the whole cache.
type Cache[K comparable, V any] struct {
	shards   []*shard[K, V]
	seed     maphash.Seed
	total    atomic.Int64
	capacity atomic.Int64
	structMu sync.Mutex
	onEvict  func(K, V)
	metrics  MetricsCollector
}

type config[K comparable, V any] struct {
	shards  int
	onEvict func(K, V)
	metrics MetricsCollector
}

// Option configures a Cache at construction time.
type Option[K comparable, V any] func(*config[K, V])

// WithMetrics registers a MetricsCollector that receives operational metrics for
// the cache instance. A nil collector disables metrics. Beware of passing a nil
// pointer of a concrete collector type: it produces a non-nil interface value,
// so the cache will invoke methods on the nil receiver instead of disabling
// metrics.
func WithMetrics[K comparable, V any](m MetricsCollector) Option[K, V] {
	return func(c *config[K, V]) {
		c.metrics = m
	}
}

// WithEvictionCallback registers a function invoked for every entry removed from
// the cache, whether by capacity eviction, Remove, Resize or Purge. It runs
// without any internal lock held, so it may safely call back into the cache.
func WithEvictionCallback[K comparable, V any](fn func(K, V)) Option[K, V] {
	return func(c *config[K, V]) {
		c.onEvict = fn
	}
}

// WithShards sets the number of shards the cache is split into. The count must
// be in the range [1, 1024]; New returns ErrInvalidShardCount otherwise. The
// effective count never exceeds the cache size.
func WithShards[K comparable, V any](n int) Option[K, V] {
	return func(c *config[K, V]) {
		c.shards = n
	}
}

// New creates a Cache holding at most size entries. It returns ErrInvalidSize
// when size is not positive and ErrInvalidShardCount when the configured shard
// count is outside [1, 1024].
func New[K comparable, V any](size int, opts ...Option[K, V]) (*Cache[K, V], error) {
	if size <= 0 {
		return nil, ErrInvalidSize
	}

	cfg := config[K, V]{shards: DefaultShards}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.shards < minShards || cfg.shards > maxShards {
		return nil, ErrInvalidShardCount
	}

	n := min(cfg.shards, size)

	c := &Cache[K, V]{
		shards:  make([]*shard[K, V], n),
		seed:    maphash.MakeSeed(),
		onEvict: cfg.onEvict,
		metrics: cfg.metrics,
	}
	var capacity int
	for i := range c.shards {
		shardSize := shardCapacity(size, n, i)
		c.shards[i] = newShard[K, V](shardSize)
		capacity += shardSize
	}
	c.capacity.Store(int64(capacity))
	return c, nil
}

// Add inserts or updates the value for key and marks it most recently used. It
// reports whether inserting the key evicted the least recently used entry in the
// key's shard.
func (c *Cache[K, V]) Add(key K, value V) (evicted bool) {
	var start time.Time
	if c.metrics != nil {
		start = time.Now()
	}

	res := c.shardFor(key).add(key, value)

	var elapsed time.Duration
	if c.metrics != nil {
		elapsed = time.Since(start)
	}
	if res.sizeDelta != 0 {
		c.total.Add(int64(res.sizeDelta))
	}

	if res.evicted && c.onEvict != nil {
		c.onEvict(res.evictKey, res.evictVal)
	}
	if c.metrics != nil {
		c.metrics.ObserveSetLatency(elapsed)
		if res.evicted {
			c.metrics.RecordEviction()
		}
		c.metrics.SetItemCount(int(c.total.Load()))
	}
	return res.evicted
}

// Get returns the value for key and marks it most recently used.
func (c *Cache[K, V]) Get(key K) (value V, ok bool) {
	var start time.Time
	if c.metrics != nil {
		start = time.Now()
	}

	value, ok = c.shardFor(key).get(key)

	if c.metrics != nil {
		c.metrics.ObserveGetLatency(time.Since(start))
		if ok {
			c.metrics.RecordHit()
		} else {
			c.metrics.RecordMiss()
		}
	}
	return value, ok
}

// Peek returns the value for key without changing its recency.
func (c *Cache[K, V]) Peek(key K) (value V, ok bool) {
	return c.shardFor(key).peek(key)
}

// Contains reports whether key is present without changing its recency.
func (c *Cache[K, V]) Contains(key K) bool {
	return c.shardFor(key).contains(key)
}

// Remove deletes key from the cache and reports whether it was present.
func (c *Cache[K, V]) Remove(key K) (present bool) {
	value, present := c.shardFor(key).remove(key)
	if !present {
		return false
	}
	c.total.Add(-1)

	if c.onEvict != nil {
		c.onEvict(key, value)
	}
	if c.metrics != nil {
		c.metrics.SetItemCount(int(c.total.Load()))
	}
	return true
}

// Keys returns every key in the cache. Order is undefined across shards; keys
// from the same shard appear from least to most recently used.
func (c *Cache[K, V]) Keys() []K {
	keys := make([]K, 0, c.total.Load())
	for _, sh := range c.shards {
		sh.appendKeys(&keys)
	}
	return keys
}

// Values returns every value in the cache. Order matches Keys.
func (c *Cache[K, V]) Values() []V {
	values := make([]V, 0, c.total.Load())
	for _, sh := range c.shards {
		sh.appendValues(&values)
	}
	return values
}

// Len returns the number of entries currently in the cache.
func (c *Cache[K, V]) Len() int {
	return int(c.total.Load())
}

// Cap returns the maximum number of entries the cache can hold.
func (c *Cache[K, V]) Cap() int {
	return int(c.capacity.Load())
}

// Shards returns the number of shards the cache is split into.
func (c *Cache[K, V]) Shards() int {
	return len(c.shards)
}

// Resize changes the total cache capacity, redistributing it across the existing
// shards and evicting the oldest entries in any shard that shrinks. Every shard
// keeps a capacity of at least one entry, so the effective capacity never drops
// below the shard count; Cap reports the capacity actually applied. It returns
// the number of entries evicted and ignores a non-positive size.
func (c *Cache[K, V]) Resize(size int) (evicted int) {
	if size <= 0 {
		return 0
	}

	c.structMu.Lock()
	n := len(c.shards)
	var capacity int
	var removed []kv[K, V]
	for i, sh := range c.shards {
		shardSize := shardCapacity(size, n, i)
		capacity += shardSize
		removed = append(removed, sh.resize(shardSize)...)
	}
	c.capacity.Store(int64(capacity))
	evicted = len(removed)
	if evicted > 0 {
		c.total.Add(-int64(evicted))
	}
	c.structMu.Unlock()

	if c.onEvict != nil {
		for _, e := range removed {
			c.onEvict(e.key, e.val)
		}
	}
	if c.metrics != nil {
		for range removed {
			c.metrics.RecordEviction()
		}
		c.metrics.SetItemCount(int(c.total.Load()))
	}
	return evicted
}

// Purge removes every entry from the cache.
func (c *Cache[K, V]) Purge() {
	collect := c.onEvict != nil

	c.structMu.Lock()
	var removed []kv[K, V]
	var purged int64
	for _, sh := range c.shards {
		entries, n := sh.purge(collect)
		removed = append(removed, entries...)
		purged += int64(n)
	}
	if purged > 0 {
		c.total.Add(-purged)
	}
	c.structMu.Unlock()

	if c.onEvict != nil {
		for _, e := range removed {
			c.onEvict(e.key, e.val)
		}
	}
	if c.metrics != nil {
		c.metrics.SetItemCount(int(c.total.Load()))
	}
}

func (c *Cache[K, V]) shardFor(key K) *shard[K, V] {
	return c.shards[maphash.Comparable(c.seed, key)%uint64(len(c.shards))]
}

func shardCapacity(total, shards, index int) int {
	size := total / shards
	if index < total%shards {
		size++
	}
	if size < 1 {
		size = 1
	}
	return size
}
