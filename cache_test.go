// Copyright 2025 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
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
