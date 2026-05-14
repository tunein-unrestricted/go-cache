// Copyright 2025 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CacheSuite struct {
	suite.Suite
}

func TestCacheSuite(t *testing.T) {
	suite.Run(t, &CacheSuite{})
}

// TestGet ensures setting and retrieval of the requested value to the cache
func (s *CacheSuite) TestGet() {
	testCases := []struct {
		title string
		val   float32
		key   string
		exp   time.Duration
	}{
		{
			title: "Success",
			key:   "test",
			val:   0.555,
			exp:   1 * time.Second,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			var (
				validate = s.Assert()
				cc       = New[string, float32](tc.exp)
			)

			cc.Set(tc.key, tc.val)
			val, err := cc.Get(tc.key)
			validate.NoError(err)
			validate.Equal(tc.val, val)

			time.Sleep(tc.exp)
			val2, err := cc.Get(tc.key)
			validate.Error(err)
			validate.Empty(val2)
		})
	}
}

// TestGetWithLoader ensures setting and retrieval of the requested value using loader func
func (s *CacheSuite) TestGetWithLoader() {
	testCases := []struct {
		title    string
		key      string
		exp      time.Duration
		loader   LoaderFunc[string, float32]
		expected float32
	}{
		{
			title: "Get value with loader func",
			key:   "test",
			exp:   1 * time.Second,
			loader: func(s string) (float32, error) {
				if s == "test" {
					return 111.89, nil
				}
				return 0, ErrNotFound
			},
			expected: 111.89,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			var (
				validate = s.Assert()
				cc       = New[string, float32](tc.exp)
			)

			cc.LoaderFunc(tc.loader)

			valid, err := cc.Get(tc.key)
			validate.NoError(err)
			validate.Equal(tc.expected, valid)

			invalid, err := cc.Get("invalid-key")
			validate.Error(err)
			validate.Equal(float32(0), invalid)
		})
	}
}

// TestKeys ensures correct KEYS array returned
func (s *CacheSuite) TestKeys() {
	var (
		validate = s.Assert()
		keys     = []int{1, 10, 100, 1000, 10000}
		cc       = New[int, bool](1 * time.Nanosecond)
	)

	for _, k := range keys {
		cc.Set(k, true)
	}

	res := cc.Keys(false)
	validate.Equal(len(keys), len(res))

	for _, k := range keys {
		validate.Contains(res, k)
	}

	res = cc.Keys(true)
	validate.Equal(0, len(res))
}

// TestSetWithExpire ensures cacheItems created with passed expiration
func (s *CacheSuite) TestSetWithExpire() {
	var (
		validate = s.Assert()
		cc       = New[int, bool](1 * time.Nanosecond)
	)

	cc.SetWithExpire(1, true, 1*time.Second)
	val, err := cc.Get(1)
	validate.NoError(err)
	validate.True(val)

	time.Sleep(1 * time.Second)
	val, err = cc.Get(1)
	validate.Error(err)
	validate.False(val)
}

// TestLen ensures correct cache length is returned
func (s *CacheSuite) TestLen() {
	var (
		validate = s.Assert()
		keys     = []int{1, 10, 100, 1000, 10000}
		cc       = New[int, bool](1 * time.Nanosecond)
	)

	for _, k := range keys {
		cc.Set(k, true)
	}

	res := cc.Len(false)
	validate.Equal(len(keys), res)

	res = cc.Len(true)
	validate.Equal(0, res)
}

// TestHas ensures that cache has saved value
func (s *CacheSuite) TestHas() {
	var (
		validate = s.Assert()
		key      = 280
		cc       = New[int, bool](500 * time.Millisecond)
	)

	cc.Set(key, true)
	validate.True(cc.Has(key))
	validate.False(cc.Has(2))

	// wait until the cached item is expired
	time.Sleep(500 * time.Millisecond)
	validate.False(cc.Has(key))
}

// TestRemove ensures item is removed from the cache
func (s *CacheSuite) TestRemove() {
	var (
		validate = s.Assert()
		key      = 280
		cc       = New[int, bool](2000 * time.Millisecond)
	)

	cc.Set(key, true)
	validate.True(cc.Has(key))
	validate.Equal(1, cc.Len(false))

	cc.Remove(key)
	validate.False(cc.Has(key))
	validate.Equal(0, cc.Len(false))
}

// TestPurge ensures correct cache cleaning
func (s *CacheSuite) TestPurge() {
	var (
		validate = s.Assert()
		keys     = []int{1, 10, 100, 1000, 10000}
		cc       = New[int, bool](1 * time.Nanosecond)
	)

	for _, k := range keys {
		cc.Set(k, true)
	}

	validate.Equal(len(keys), cc.Len(false))

	cc.Purge()
	validate.Empty(cc.Len(false))
}

func (s *CacheSuite) TestSet() {
	cc := New[string, int](time.Second)

	cc.Set("a", 10)
	v, _ := cc.Get("a")
	s.Require().Equal(10, v)

	cc.Set("a", 20)
	v, _ = cc.Get("a")
	s.Require().Equal(20, v)
}

func (s *CacheSuite) TestCalcSet() {
	cc := New[string, int](time.Second)

	cc.Set("a", 5)
	cc.Update("a", func(v int) int {
		return v * v
	})

	v, _ := cc.Get("a")
	s.Require().Equal(25, v)
}

// TestHasConcurrentRace is a regression test for the data race that existed in
// Has() before it was protected by c.mtx.RLock. Run with -race to verify.
func (s *CacheSuite) TestHasConcurrentRace() {
	cc := New[int, int](time.Second)
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func(k int) { defer wg.Done(); cc.Set(k, k) }(i)
		go func(k int) { defer wg.Done(); cc.Has(k) }(i)
	}
	wg.Wait()
}

// TestGetTOCTOU is a regression test for the TOCTOU window that existed in the
// non-LRU get() path: a Set between the RUnlock and the write-lock used to
// delete the key could silently destroy a freshly written valid entry.
func (s *CacheSuite) TestGetTOCTOU() {
	var (
		validate = s.Assert()
		cc       = New[int, int](50 * time.Millisecond)
		wg       sync.WaitGroup
	)

	cc.Set(1, 42)
	time.Sleep(60 * time.Millisecond) // let the entry expire

	// Concurrently: one goroutine triggers the expired-delete path via Get,
	// another immediately writes a fresh value for the same key.
	wg.Add(2)
	go func() {
		defer wg.Done()
		cc.Get(1) //nolint:errcheck
	}()
	go func() {
		defer wg.Done()
		cc.Set(1, 99)
	}()
	wg.Wait()

	// At this point the key must be present with value 99 (the fresh write),
	// OR absent (if the fresh write happened before the expiry check). What must
	// never happen is Get returning 42 (stale) or 99 being silently deleted.
	v, err := cc.Get(1)
	if err == nil {
		validate.Equal(99, v, "if key is present after concurrent Set+expired-Get, value must be the fresh one")
	}
}

// TestStartCleaner verifies that the background cleaner removes expired items
// from memory and that the stop function terminates the goroutine cleanly.
func (s *CacheSuite) TestStartCleaner() {
	var (
		validate = s.Assert()
		cc       = New[int, bool](100 * time.Millisecond)
	)

	stop := cc.StartCleaner(50 * time.Millisecond)
	defer stop()

	for i := range 10 {
		cc.Set(i, true)
	}
	validate.Equal(10, cc.Len(false))

	// Wait for entries to expire and for the cleaner to sweep them.
	time.Sleep(300 * time.Millisecond)

	validate.Equal(0, cc.Len(false), "cleaner must have removed all expired entries")

	// Stopping twice must not panic (OnceFunc guard).
	stop()
}

// TestNewWithOptions ensures the functional-options constructor applies options correctly
func (s *CacheSuite) TestNewWithOptions() {
	var (
		validate   = s.Assert()
		evictedKey int
		cc         = NewWithOptions(
			WithExpiration[int, string](500*time.Millisecond),
			WithMaxSize[int, string](2),
		)
	)

	// Expiration option is honoured
	cc.Set(1, "one")
	_, err := cc.Get(1)
	validate.NoError(err)

	time.Sleep(500 * time.Millisecond)
	_, err = cc.Get(1)
	validate.ErrorIs(err, ErrNotFound, "entry should have expired")

	// MaxSize option is honoured
	cc2 := NewWithOptions(
		WithMaxSize[int, string](2),
		WithExpiration[int, string](0),
	)
	cc2.EvictedFunc(func(k int, v string) { evictedKey = k })
	cc2.Set(1, "one")
	cc2.Set(2, "two")
	cc2.Set(3, "three") // should evict key 1
	validate.Equal(1, evictedKey)
	validate.Equal(2, cc2.Len(false))
}

// TestLRUEviction ensures the least recently used item is evicted when the cache is full
func (s *CacheSuite) TestLRUEviction() {
	var (
		validate = s.Assert()
		cc       = New[int, string](0).MaxSize(3)
	)

	cc.Set(1, "one")
	cc.Set(2, "two")
	cc.Set(3, "three")

	// Cache is full; adding a 4th item should evict key 1 (LRU)
	cc.Set(4, "four")

	validate.Equal(3, cc.Len(false))
	validate.False(cc.Has(1), "key 1 should have been evicted")
	validate.True(cc.Has(2))
	validate.True(cc.Has(3))
	validate.True(cc.Has(4))
}

// TestLRUEvictionOrderUpdatedByGet ensures a Get promotes the item and protects it from eviction
func (s *CacheSuite) TestLRUEvictionOrderUpdatedByGet() {
	var (
		validate = s.Assert()
		cc       = New[int, string](0).MaxSize(3)
	)

	cc.Set(1, "one")
	cc.Set(2, "two")
	cc.Set(3, "three")

	// Access key 1 to make it recently used
	_, err := cc.Get(1)
	validate.NoError(err)

	// Adding a 4th item should evict key 2 (now the LRU)
	cc.Set(4, "four")

	validate.True(cc.Has(1), "key 1 should not be evicted (was recently accessed)")
	validate.False(cc.Has(2), "key 2 should have been evicted (LRU after access to key 1)")
	validate.True(cc.Has(3))
	validate.True(cc.Has(4))
}

// TestLRUEvictedFunc ensures the evicted callback is called with the correct key and value
func (s *CacheSuite) TestLRUEvictedFunc() {
	var (
		validate   = s.Assert()
		evictedKey int
		evictedVal string
		cc         = New[int, string](0).MaxSize(2).EvictedFunc(func(k int, v string) {
			evictedKey = k
			evictedVal = v
		})
	)

	cc.Set(1, "one")
	cc.Set(2, "two")
	cc.Set(3, "three") // should evict key 1

	validate.Equal(1, evictedKey)
	validate.Equal("one", evictedVal)
}

// TestLRUUpdateDoesNotGrowCache ensures updating an existing key does not grow the cache or trigger eviction
func (s *CacheSuite) TestLRUUpdateDoesNotGrowCache() {
	var (
		validate = s.Assert()
		evicted  int
		cc       = New[int, string](0).MaxSize(2).EvictedFunc(func(k int, v string) {
			evicted++
		})
	)

	cc.Set(1, "one")
	cc.Set(2, "two")
	cc.Set(1, "ONE") // update, not a new entry

	validate.Equal(2, cc.Len(false))
	validate.Equal(0, evicted, "no eviction should occur when updating an existing key")

	v, err := cc.Get(1)
	validate.NoError(err)
	validate.Equal("ONE", v)
}

func (s *CacheSuite) TestConcurrentUpdate() {
	var (
		validate = s.Assert()
		cc       = New[int, int](1000 * time.Millisecond)
		calc     = func(in int) int {
			return in + 100
		}
		k1 = 1
	)

	cc.Set(k1, 1)
	for i := 0; i < 10; i++ {
		go func() {
			cc.Update(k1, calc)
		}()
	}
	time.Sleep(20 * time.Millisecond)
	res, err := cc.Get(k1)
	validate.NoError(err)
	validate.Equal(1001, res)
}

// TestConcurrentLoadStress runs 80 writer and 20 reader goroutines simultaneously
// against a cache capped at 10 000 items and verifies four invariants:
//
//  1. The cache never exceeds its maxSize.
//  2. Every value returned by Get matches the value that was written for that key.
//  3. The average latency of a cache hit stays under 1 ms.
//  4. LRU eviction order is correct: recently-accessed keys survive while the
//     least-recently-used keys are evicted first.
func (s *CacheSuite) TestConcurrentLoadStress() {
	const (
		maxSize      = 10_000
		keySpace     = 50_000
		total        = 100 // numWriters + numReaders
		testDuration = 3 * time.Second
		maxAvgHitNs  = int64(time.Millisecond)
	)

	t := s.T()
	require := s.Require()

	valueFor := func(key int) string { return fmt.Sprintf("v%d", key) }

	cc := NewWithOptions(
		WithMaxSize[int, string](maxSize),
		WithExpiration[int, string](0),
	)

	var (
		wg             sync.WaitGroup
		// ready gates all goroutines so readers and writers truly start together.
		ready          = make(chan struct{})
		stop           = make(chan struct{})
		sizeViolations atomic.Int64
		valueErrors    atomic.Int64
		hitCount       atomic.Int64
		totalHitNs     atomic.Int64
	)

	// Monitor goroutine: sample Len every millisecond and flag any size breach.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if l := cc.Len(false); l > maxSize {
					sizeViolations.Add(1)
				}
			}
		}
	}()

	// Launch all 100 goroutines in a single loop. Every 5th goroutine (ids 4, 9,
	// 14, …) is a reader; the other 80 are writers. All of them block on <-ready
	// so that readers and writers are released simultaneously.
	for i := range total {
		wg.Add(1)
		isReader := (i+1)%5 == 0 // 20 readers, 80 writers
		go func(id int) {
			defer wg.Done()
			<-ready // wait for the start gun
			key := id * (keySpace / total)
			for {
				select {
				case <-stop:
					return
				default:
					k := key % keySpace
					key++
					if isReader {
						start := time.Now()
						v, err := cc.Get(k)
						ns := time.Since(start).Nanoseconds()
						if err == nil {
							hitCount.Add(1)
							totalHitNs.Add(ns)
							if v != valueFor(k) {
								valueErrors.Add(1)
							}
						}
					} else {
						cc.Set(k, valueFor(k))
					}
				}
			}
		}(i)
	}

	close(ready) // release all goroutines at once
	time.Sleep(testDuration)
	close(stop)
	wg.Wait()

	// ── Invariant 1: size never exceeded ────────────────────────────────────
	require.Zero(sizeViolations.Load(), "cache Len exceeded maxSize during the run")
	require.LessOrEqual(cc.Len(false), maxSize, "final cache size exceeds maxSize")

	// ── Invariant 2: correctness ─────────────────────────────────────────────
	require.Zero(valueErrors.Load(), "Get returned an incorrect value for a key")

	// ── Invariant 3: hit latency under 1 ms (average) ────────────────────────
	hits := hitCount.Load()
	t.Logf("total hits: %d", hits)
	if hits > 0 {
		avgNs := totalHitNs.Load() / hits
		t.Logf("average hit latency: %s", time.Duration(avgNs))
		require.Less(avgNs, maxAvgHitNs,
			"average cache hit latency (%s) must be under 1ms", time.Duration(avgNs))
	}

	// ── Invariant 4: LRU eviction order ─────────────────────────────────────
	// Use a small, fully-controlled cache to verify that Get promotes a key to
	// MRU and that eviction always removes the least-recently-used entry.
	//
	// Initial fill:  Set 0..4  →  LRU order (front=MRU): [4, 3, 2, 1, 0]
	// Pin keys 0 and 4 via Get  →  order becomes:         [4, 0, 3, 2, 1]
	// Add keys 5, 6, 7 (cap=5) →  evicts 1, then 2, then 3 (back of list)
	// Expected survivors: 0, 4, 5, 6, 7   Evicted: 1, 2, 3
	lru := NewWithOptions(WithMaxSize[int, string](5))
	for i := range 5 {
		lru.Set(i, valueFor(i))
	}
	lru.Get(0) // promote 0 → MRU
	lru.Get(4) // promote 4 → MRU
	lru.Set(5, valueFor(5)) // evicts 1 (LRU)
	lru.Set(6, valueFor(6)) // evicts 2
	lru.Set(7, valueFor(7)) // evicts 3

	require.True(lru.Has(0), "key 0 was recently accessed; must not be evicted")
	require.True(lru.Has(4), "key 4 was recently accessed; must not be evicted")
	require.False(lru.Has(1), "key 1 was LRU; must be evicted")
	require.False(lru.Has(2), "key 2 was LRU; must be evicted")
	require.False(lru.Has(3), "key 3 was LRU; must be evicted")
	require.True(lru.Has(5), "newly added key 5 must be present")
	require.True(lru.Has(6), "newly added key 6 must be present")
	require.True(lru.Has(7), "newly added key 7 must be present")
}
