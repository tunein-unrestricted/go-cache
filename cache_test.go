// Copyright 2025 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	testKey    = "test"
	testKeyInt = 1
)

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
			exp:   100 * time.Millisecond,
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

			time.Sleep(tc.exp + 20*time.Millisecond)
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
	testCases := []struct {
		title      string
		defaultTTL time.Duration
		itemTTL    time.Duration
	}{
		{title: "ItemTTLLongerThanDefault", defaultTTL: time.Nanosecond, itemTTL: 100 * time.Millisecond},
		{title: "ItemTTLShorterThanDefault", defaultTTL: time.Hour, itemTTL: 100 * time.Millisecond},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, bool](tc.defaultTTL)

			cc.SetWithExpire(1, true, tc.itemTTL)
			val, err := cc.Get(1)
			validate.NoError(err)
			validate.True(val)

			time.Sleep(tc.itemTTL + 20*time.Millisecond)
			val, err = cc.Get(1)
			validate.Error(err)
			validate.False(val)
		})
	}
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
	testCases := []struct {
		title      string
		key        int
		missingKey int
		ttl        time.Duration
	}{
		{title: "PresentUntilExpired", key: 280, missingKey: 2, ttl: 100 * time.Millisecond},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, bool](tc.ttl)

			cc.Set(tc.key, true)
			validate.True(cc.Has(tc.key))
			validate.False(cc.Has(tc.missingKey))

			time.Sleep(tc.ttl + 20*time.Millisecond)
			validate.False(cc.Has(tc.key))
		})
	}
}

// TestRemove ensures item is removed from the cache
func (s *CacheSuite) TestRemove() {
	testCases := []struct {
		title string
		key   int
	}{
		{title: "RemovesPresentKey", key: 280},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, bool](time.Minute)

			cc.Set(tc.key, true)
			validate.True(cc.Has(tc.key))
			validate.Equal(1, cc.Len(false))

			cc.Remove(tc.key)
			validate.False(cc.Has(tc.key))
			validate.Equal(0, cc.Len(false))
		})
	}
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
	testCases := []struct {
		title  string
		writes []int
		want   int
	}{
		{title: "SingleWrite", writes: []int{10}, want: 10},
		{title: "OverwriteKeepsLastValue", writes: []int{10, 20}, want: 20},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			cc := New[string, int](time.Minute)
			for _, w := range tc.writes {
				cc.Set(testKey, w)
			}

			v, err := cc.Get(testKey)
			require.NoError(err)
			require.Equal(tc.want, v)
		})
	}
}

func (s *CacheSuite) TestCalcSet() {
	testCases := []struct {
		title string
		seed  *int
		calc  func(v int) int
		want  int
	}{
		{title: "UpdatesExistingValue", seed: new(5), calc: func(v int) int { return v * v }, want: 25},
		{title: "MissingKeyStartsFromZeroValue", seed: nil, calc: func(v int) int { return v + 7 }, want: 7},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			cc := New[string, int](time.Minute)
			if tc.seed != nil {
				cc.Set(testKey, *tc.seed)
			}

			cc.Update(testKey, tc.calc)

			v, err := cc.Get(testKey)
			require.NoError(err)
			require.Equal(tc.want, v)
		})
	}
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

// TestCleanupOnSetRateLimit verifies that the sweep is rate-limited to once per
// TTL period: expired entries written between two sweeps accumulate until the
// next sweep window opens.
func (s *CacheSuite) TestCleanupOnSetRateLimit() {
	const ttl = 200 * time.Millisecond
	require := s.Require()
	cc := New[int, int](ttl)

	// First Set triggers an immediate sweep (cleanedAt is zero).
	cc.Set(1, 1)

	// These items expire after ttl, but the sweep window has just been reset,
	// so no cleanup runs for the next <ttl interval.
	time.Sleep(ttl + 10*time.Millisecond) // items are now expired
	cc.Set(2, 2)                          // sweep fires: cleans up key 1, writes key 2
	time.Sleep(10 * time.Millisecond)     // wait for the background goroutine to finish
	require.Equal(1, cc.Len(false), "second sweep must remove the expired entry")

	// Key 2 must still be alive and readable.
	v, err := cc.Get(2)
	require.NoError(err)
	require.Equal(2, v)
}

func (s *CacheSuite) TestAddedFuncReentrancy() {
	increment := func(v int) int { return v + 1 }
	testCases := []struct {
		title   string
		seed    func(cc *Cache[int, int])
		reenter func(cc *Cache[int, int], key, val int) bool
		trigger func(cc *Cache[int, int])
		want    int
	}{
		{
			title: "SetCallbackCallsSet",
			seed:  func(*Cache[int, int]) {},
			reenter: func(cc *Cache[int, int], key, _ int) bool {
				if key != 1 {
					return false
				}
				cc.Set(2, 200)
				return true
			},
			trigger: func(cc *Cache[int, int]) { cc.Set(1, 100) },
			want:    200,
		},
		{
			title: "SetCallbackCallsUpdate",
			seed:  func(cc *Cache[int, int]) { cc.Set(2, 10) },
			reenter: func(cc *Cache[int, int], key, _ int) bool {
				if key != 1 {
					return false
				}
				cc.Update(2, increment)
				return true
			},
			trigger: func(cc *Cache[int, int]) { cc.Set(1, 100) },
			want:    11,
		},
		{
			title: "UpdateCallbackCallsSet",
			seed:  func(cc *Cache[int, int]) { cc.Set(1, 0) },
			reenter: func(cc *Cache[int, int], key, val int) bool {
				if key != 1 || val == 0 {
					return false
				}
				cc.Set(2, 999)
				return true
			},
			trigger: func(cc *Cache[int, int]) { cc.Update(1, increment) },
			want:    999,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			cc := New[int, int](time.Minute)
			tc.seed(cc)

			done := make(chan struct{})
			var once sync.Once
			cc.AddedFunc(func(key, val int) {
				if tc.reenter(cc, key, val) {
					once.Do(func() { close(done) })
				}
			})
			go tc.trigger(cc)

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				require.Fail("deadlock: re-entrant call from addedFunc never completed")
			}

			v, err := cc.Get(2)
			require.NoError(err)
			require.Equal(tc.want, v)
		})
	}
}

func (s *CacheSuite) TestConcurrentUpdate() {
	testCases := []struct {
		title    string
		initial  int
		step     int
		updaters int
		want     int
	}{
		{title: "EveryUpdateApplied", initial: 1, step: 100, updaters: 10, want: 1001},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			cc := New[int, int](time.Minute)
			cc.Set(testKeyInt, tc.initial)

			var wg sync.WaitGroup
			for range tc.updaters {
				wg.Go(func() {
					cc.Update(testKeyInt, func(v int) int { return v + tc.step })
				})
			}
			wg.Wait()

			res, err := cc.Get(testKeyInt)
			validate.NoError(err)
			validate.Equal(tc.want, res)
		})
	}
}

func (s *CacheSuite) TestStoredTTL() {
	const defaultTTL = time.Minute
	loadWith := func(exp *time.Duration) func(cc *Cache[string, int]) {
		return func(cc *Cache[string, int]) {
			cc.LoaderExpireFunc(func(string) (int, *time.Duration, error) {
				return 1, exp, nil
			})
			_, _ = cc.Get(testKey)
		}
	}
	testCases := []struct {
		title string
		write func(cc *Cache[string, int])
		want  time.Duration
	}{
		{title: "SetUsesDefault", write: func(cc *Cache[string, int]) { cc.Set(testKey, 1) }, want: defaultTTL},
		{title: "SetWithExpireZeroNeverExpires", write: func(cc *Cache[string, int]) { cc.SetWithExpire(testKey, 1, 0) }, want: 0},
		{title: "SetWithExpireNegativeUsesDefault", write: func(cc *Cache[string, int]) { cc.SetWithExpire(testKey, 1, -1) }, want: defaultTTL},
		{title: "SetWithExpireCustom", write: func(cc *Cache[string, int]) { cc.SetWithExpire(testKey, 1, 5*time.Second) }, want: 5 * time.Second},
		{title: "UpdateUsesDefault", write: func(cc *Cache[string, int]) { cc.Update(testKey, func(v int) int { return v + 1 }) }, want: defaultTTL},
		{title: "UpdateWithExpireZeroNeverExpires", write: func(cc *Cache[string, int]) { cc.UpdateWithExpire(testKey, func(v int) int { return v + 1 }, 0) }, want: 0},
		{title: "LoaderNilDurationUsesDefault", write: loadWith(nil), want: defaultTTL},
		{title: "LoaderZeroDurationNeverExpires", write: loadWith(new(time.Duration(0))), want: 0},
		{title: "LoaderNegativeDurationUsesDefault", write: loadWith(new(-time.Second)), want: defaultTTL},
		{title: "LoaderCustomDuration", write: loadWith(new(5 * time.Second)), want: 5 * time.Second},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			cc := New[string, int](defaultTTL)
			tc.write(cc)

			cc.mtx.RLock()
			item, ok := cc.items[testKey]
			cc.mtx.RUnlock()
			require.True(ok)
			require.Equal(tc.want, item.ttl)
		})
	}
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
