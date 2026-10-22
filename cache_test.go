// Copyright 2025 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const testKey = "test"

type CacheSuite struct {
	suite.Suite
}

func TestCacheSuite(t *testing.T) {
	t.Parallel()
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
			key:   testKey,
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
			key:   testKey,
			exp:   1 * time.Second,
			loader: func(s string) (float32, error) {
				if s == testKey {
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
	testCases := []struct {
		title string
		keys  []int
		ttl   time.Duration
	}{
		{title: "ExpiredKeysListedOnlyWithoutExpiryCheck", keys: []int{1, 10, 100, 1000, 10000}, ttl: 20 * time.Millisecond},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, bool](tc.ttl)
			for _, k := range tc.keys {
				cc.Set(k, true)
			}
			time.Sleep(tc.ttl + 10*time.Millisecond)

			validate.ElementsMatch(tc.keys, cc.Keys(false))
			validate.Empty(cc.Keys(true))
		})
	}
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
	testCases := []struct {
		title string
		keys  []int
		ttl   time.Duration
	}{
		{title: "ExpiredKeysCountedOnlyWithoutExpiryCheck", keys: []int{1, 10, 100, 1000, 10000}, ttl: 20 * time.Millisecond},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, bool](tc.ttl)
			for _, k := range tc.keys {
				cc.Set(k, true)
			}
			time.Sleep(tc.ttl + 10*time.Millisecond)

			validate.Equal(len(tc.keys), cc.Len(false))
			validate.Equal(0, cc.Len(true))
		})
	}
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
	testCases := []struct {
		title string
		keys  []int
	}{
		{title: "RemovesAllItems", keys: []int{1, 10, 100, 1000, 10000}},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, bool](time.Minute)
			for _, k := range tc.keys {
				cc.Set(k, true)
			}
			validate.Equal(len(tc.keys), cc.Len(false))

			cc.Purge()
			validate.Empty(cc.Len(false))
		})
	}
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

// TestCleanupOnSet verifies that Set removes all expired entries once per TTL
// period to prevent the cache from growing uncontrollably over time.
func (s *CacheSuite) TestCleanupOnSet() {
	const (
		ttl   = 100 * time.Millisecond
		items = 10
	)
	require := s.Require()
	cc := New[int, int](ttl)

	// Fill the cache with items that will all expire.
	for i := range items {
		cc.Set(i, i)
	}
	require.Equal(items, cc.Len(false), "all items must be present before expiry")

	// Wait for every entry to expire, then trigger a Set.
	time.Sleep(ttl + 10*time.Millisecond)
	cc.Set(99, 99)

	// The sweep runs in a background goroutine; give it time to finish.
	time.Sleep(10 * time.Millisecond)
	require.Equal(1, cc.Len(false), "Set must remove all expired entries")
	v, err := cc.Get(99)
	require.NoError(err)
	require.Equal(99, v)
}

// TestCleanupOnSetRateLimit verifies that sweeps run at most once per sweep
// interval: an entry that expires right after a sweep stays until the next one.
func (s *CacheSuite) TestCleanupOnSetRateLimit() {
	testCases := []struct {
		title      string
		defaultTTL time.Duration
		itemTTL    time.Duration
	}{
		{title: "ExpiredItemWaitsForNextSweepInterval", defaultTTL: 300 * time.Millisecond, itemTTL: 10 * time.Millisecond},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			cc := New[int, int](tc.defaultTTL)
			swept := func(key int) func() bool {
				return func() bool { return !slices.Contains(cc.Keys(false), key) }
			}

			cc.SetWithExpire(1, 1, tc.itemTTL)
			time.Sleep(2 * tc.itemTTL)
			cc.SetWithExpire(2, 2, time.Hour)
			require.Eventually(swept(1), time.Second, time.Millisecond)

			cc.SetWithExpire(3, 3, tc.itemTTL)
			time.Sleep(2 * tc.itemTTL)
			cc.SetWithExpire(4, 4, time.Hour)
			time.Sleep(2 * tc.itemTTL)
			require.Contains(cc.Keys(false), 3)

			time.Sleep(tc.defaultTTL)
			cc.SetWithExpire(5, 5, time.Hour)
			require.Eventually(swept(3), time.Second, time.Millisecond)
		})
	}
}

func (s *CacheSuite) TestSweepRemovesPerItemExpired() {
	testCases := []struct {
		title      string
		defaultTTL time.Duration
		itemTTL    time.Duration
		items      int
	}{
		{title: "NoDefaultTTL", defaultTTL: 0, itemTTL: 20 * time.Millisecond, items: 100},
		{title: "DefaultTTLLongerThanItemTTL", defaultTTL: time.Hour, itemTTL: 20 * time.Millisecond, items: 100},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			cc := New[int, int](tc.defaultTTL)
			for i := range tc.items {
				cc.SetWithExpire(i, i, tc.itemTTL)
			}
			time.Sleep(tc.itemTTL + 10*time.Millisecond)

			cc.SetWithExpire(tc.items, tc.items, time.Hour)
			require.Eventually(func() bool { return cc.Len(false) == 1 }, time.Second, 5*time.Millisecond)
		})
	}
}

// TestAddedFuncCanCallSet verifies that an addedFunc registered on the cache
// may safely call Set on the same cache without deadlocking.  Previously,
// addedFunc was invoked while the caller still held umtx (RLock from Set /
// Lock from Update), so any re-entrant Set/Update inside the callback would
// self-deadlock.
func (s *CacheSuite) TestAddedFuncCanCallSet() {
	require := s.Require()

	cc := New[int, int](time.Second)
	done := make(chan struct{})

	cc.AddedFunc(func(key, _ int) {
		if key == 1 {
			// Calling Set from within an addedFunc must not deadlock.
			cc.Set(2, 200)
			close(done)
		}
	})

	cc.Set(1, 100)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("deadlock: addedFunc calling Set never completed")
	}

	v, err := cc.Get(2)
	require.NoError(err)
	require.Equal(200, v)
}

// TestAddedFuncCanCallUpdateFromSet verifies that an addedFunc triggered by Set
// can call Update without deadlocking (previously deadlocked because Set held
// umtx.RLock and Update tries umtx.Lock).
func (s *CacheSuite) TestAddedFuncCanCallUpdateFromSet() {
	require := s.Require()

	cc := New[int, int](time.Second)
	cc.Set(2, 10) // seed value for Update to read

	done := make(chan struct{})
	cc.AddedFunc(func(key, _ int) {
		if key == 1 {
			cc.Update(2, func(v int) int { return v + 1 })
			close(done)
		}
	})

	cc.Set(1, 100)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("deadlock: addedFunc calling Update never completed")
	}

	v, err := cc.Get(2)
	require.NoError(err)
	require.Equal(11, v)
}

// TestAddedFuncCanCallSetFromUpdate verifies that an addedFunc triggered by
// Update can call Set without deadlocking (previously deadlocked because Update
// held umtx.Lock and Set tries umtx.RLock).
func (s *CacheSuite) TestAddedFuncCanCallSetFromUpdate() {
	require := s.Require()

	cc := New[int, int](time.Second)
	cc.Set(1, 0)

	done := make(chan struct{})
	cc.AddedFunc(func(key, val int) {
		if key == 1 && val > 0 {
			cc.Set(2, 999)
			close(done)
		}
	})

	cc.Update(1, func(v int) int { return v + 1 })

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("deadlock: addedFunc calling Set from Update never completed")
	}

	v, err := cc.Get(2)
	require.NoError(err)
	require.Equal(999, v)
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
	for range 10 {
		go func() {
			cc.Update(k1, calc)
		}()
	}
	time.Sleep(20 * time.Millisecond)
	res, err := cc.Get(k1)
	validate.NoError(err)
	validate.Equal(1001, res)
}

func (s *CacheSuite) TestLoaderPanic() {
	testCases := []struct {
		title string
		key   int
		want  int
	}{
		{title: "NextGetCallsLoaderAgainAfterPanic", key: 7, want: 70},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			var calls atomic.Int32
			cc := New[int, int](time.Minute)
			cc.LoaderFunc(func(k int) (int, error) {
				if calls.Add(1) == 1 {
					panic("loader failure")
				}
				return k * 10, nil
			})

			require.Panics(func() { _, _ = cc.Get(tc.key) })

			type result struct {
				val int
				err error
			}
			done := make(chan result, 1)
			go func() {
				v, err := cc.Get(tc.key)
				done <- result{val: v, err: err}
			}()

			select {
			case r := <-done:
				require.NoError(r.err)
				require.Equal(tc.want, r.val)
			case <-time.After(time.Second):
				s.Fail("Get blocked after the loader panicked")
			}
		})
	}
}

func (s *CacheSuite) TestPurgeDuringLoads() {
	testCases := []struct {
		title   string
		getters int
		keys    int
		purges  int
	}{
		{title: "ConcurrentGetWithLoaderAndPurge", getters: 8, keys: 200, purges: 200},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, int](time.Minute)
			cc.LoaderFunc(func(k int) (int, error) {
				return k * 2, nil
			})

			var wg sync.WaitGroup
			for g := range tc.getters {
				wg.Go(func() {
					for k := range tc.keys {
						key := g*tc.keys + k
						v, err := cc.Get(key)
						validate.NoError(err)
						validate.Equal(key*2, v)
					}
				})
			}
			wg.Go(func() {
				for range tc.purges {
					cc.Purge()
				}
			})
			wg.Wait()

			v, err := cc.Get(1)
			validate.NoError(err)
			validate.Equal(2, v)
		})
	}
}
