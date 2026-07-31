// Copyright 2026 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package lru

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type fakeMetrics struct {
	mu        sync.Mutex
	getCalls  int
	setCalls  int
	hits      int
	misses    int
	evictions int
	itemCount int
}

func (f *fakeMetrics) ObserveGetLatency(time.Duration) {
	f.mu.Lock()
	f.getCalls++
	f.mu.Unlock()
}

func (f *fakeMetrics) ObserveSetLatency(time.Duration) {
	f.mu.Lock()
	f.setCalls++
	f.mu.Unlock()
}

func (f *fakeMetrics) RecordHit() {
	f.mu.Lock()
	f.hits++
	f.mu.Unlock()
}

func (f *fakeMetrics) RecordMiss() {
	f.mu.Lock()
	f.misses++
	f.mu.Unlock()
}

func (f *fakeMetrics) RecordEviction() {
	f.mu.Lock()
	f.evictions++
	f.mu.Unlock()
}

func (f *fakeMetrics) SetItemCount(n int) {
	f.mu.Lock()
	f.itemCount = n
	f.mu.Unlock()
}

func (f *fakeMetrics) snapshot() fakeMetrics {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fakeMetrics{
		getCalls:  f.getCalls,
		setCalls:  f.setCalls,
		hits:      f.hits,
		misses:    f.misses,
		evictions: f.evictions,
		itemCount: f.itemCount,
	}
}

type LRUSuite struct {
	suite.Suite
}

func TestLRUSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &LRUSuite{})
}

func mustNew(size int, opts ...Option[int, int]) *Cache[int, int] {
	c, err := New(size, opts...)
	if err != nil {
		panic(err)
	}
	return c
}

func single(size int, opts ...Option[int, int]) *Cache[int, int] {
	return mustNew(size, append([]Option[int, int]{WithShards[int, int](1)}, opts...)...)
}

func (s *LRUSuite) TestNew() {
	testCases := []struct {
		title     string
		size      int
		expectErr bool
	}{
		{title: "PositiveSize", size: 4, expectErr: false},
		{title: "SizeOne", size: 1, expectErr: false},
		{title: "ZeroSize", size: 0, expectErr: true},
		{title: "NegativeSize", size: -3, expectErr: true},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c, err := New[int, int](tc.size)
			if tc.expectErr {
				validate.ErrorIs(err, ErrInvalidSize)
				validate.Nil(c)
				return
			}
			validate.NoError(err)
			validate.NotNil(c)
			validate.Equal(tc.size, c.Cap())
		})
	}
}

func (s *LRUSuite) TestShardCount() {
	testCases := []struct {
		title     string
		shards    int
		expectErr bool
	}{
		{title: "Min", shards: 1, expectErr: false},
		{title: "Max", shards: 1024, expectErr: false},
		{title: "Zero", shards: 0, expectErr: true},
		{title: "TooMany", shards: 1025, expectErr: true},
		{title: "Negative", shards: -1, expectErr: true},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c, err := New[int, int](512, WithShards[int, int](tc.shards))
			if tc.expectErr {
				validate.ErrorIs(err, ErrInvalidShardCount)
				validate.Nil(c)
				return
			}
			validate.NoError(err)
			validate.NotNil(c)
			validate.Equal(512, c.Cap())
		})
	}
}

func (s *LRUSuite) TestShardCountClampedToSize() {
	testCases := []struct {
		title      string
		size       int
		shards     int
		wantShards int
	}{
		{title: "MoreShardsThanSize", size: 3, shards: 64, wantShards: 3},
		{title: "FewerShardsThanSize", size: 100, shards: 8, wantShards: 8},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := mustNew(tc.size, WithShards[int, int](tc.shards))
			validate.Equal(tc.wantShards, c.Shards())
			validate.Equal(tc.size, c.Cap())
		})
	}
}

func (s *LRUSuite) TestAddGet() {
	testCases := []struct {
		title  string
		writes map[int]int
		key    int
		wantOK bool
		want   int
	}{
		{title: "Hit", writes: map[int]int{1: 10, 2: 20}, key: 2, wantOK: true, want: 20},
		{title: "Miss", writes: map[int]int{1: 10}, key: 99, wantOK: false, want: 0},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := single(8)
			for k, v := range tc.writes {
				c.Add(k, v)
			}
			got, ok := c.Get(tc.key)
			validate.Equal(tc.wantOK, ok)
			validate.Equal(tc.want, got)
		})
	}
}

func (s *LRUSuite) TestUpdateExisting() {
	testCases := []struct {
		title string
		first int
		next  int
	}{
		{title: "Overwrite", first: 1, next: 2},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := single(4)
			evicted := c.Add(1, tc.first)
			validate.False(evicted)
			evicted = c.Add(1, tc.next)
			validate.False(evicted)
			validate.Equal(1, c.Len())
			got, ok := c.Get(1)
			validate.True(ok)
			validate.Equal(tc.next, got)
		})
	}
}

func (s *LRUSuite) TestEvictionOrder() {
	testCases := []struct {
		title    string
		size     int
		inserts  []int
		wantKeys []int
		wantEvic bool
	}{
		{
			title:    "EvictsLeastRecentlyUsed",
			size:     3,
			inserts:  []int{1, 2, 3, 4},
			wantKeys: []int{2, 3, 4},
			wantEvic: true,
		},
		{
			title:    "NoEvictionUnderCapacity",
			size:     3,
			inserts:  []int{1, 2},
			wantKeys: []int{1, 2},
			wantEvic: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := single(tc.size)
			var lastEvicted bool
			for _, k := range tc.inserts {
				lastEvicted = c.Add(k, k*10)
			}
			validate.Equal(tc.wantEvic, lastEvicted)
			validate.Equal(tc.wantKeys, c.Keys())
			validate.Equal(len(tc.wantKeys), c.Len())
		})
	}
}

func (s *LRUSuite) TestGetUpdatesRecency() {
	testCases := []struct {
		title    string
		touch    int
		add      int
		wantKeys []int
	}{
		{title: "GetProtectsFromEviction", touch: 1, add: 4, wantKeys: []int{3, 1, 4}},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := single(3)
			c.Add(1, 10)
			c.Add(2, 20)
			c.Add(3, 30)
			c.Get(tc.touch)
			c.Add(tc.add, tc.add*10)
			validate.Equal(tc.wantKeys, c.Keys())
			validate.False(c.Contains(2))
		})
	}
}

func (s *LRUSuite) TestPeekDoesNotUpdateRecency() {
	testCases := []struct {
		title string
		peek  int
	}{
		{title: "PeekKeepsOrder", peek: 1},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := single(3)
			c.Add(1, 10)
			c.Add(2, 20)
			c.Add(3, 30)
			got, ok := c.Peek(tc.peek)
			validate.True(ok)
			validate.Equal(10, got)
			c.Add(4, 40)
			validate.False(c.Contains(tc.peek))
			validate.Equal([]int{2, 3, 4}, c.Keys())
		})
	}
}

func (s *LRUSuite) TestRemove() {
	testCases := []struct {
		title      string
		remove     int
		wantResult bool
		wantLen    int
	}{
		{title: "Present", remove: 2, wantResult: true, wantLen: 2},
		{title: "Absent", remove: 99, wantResult: false, wantLen: 3},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := single(4)
			c.Add(1, 10)
			c.Add(2, 20)
			c.Add(3, 30)
			validate.Equal(tc.wantResult, c.Remove(tc.remove))
			validate.Equal(tc.wantLen, c.Len())
			validate.False(c.Contains(tc.remove) && tc.wantResult)
		})
	}
}

func (s *LRUSuite) TestResize() {
	testCases := []struct {
		title       string
		inserts     []int
		newSize     int
		wantEvicted int
		wantKeys    []int
	}{
		{title: "Shrink", inserts: []int{1, 2, 3, 4}, newSize: 2, wantEvicted: 2, wantKeys: []int{3, 4}},
		{title: "Grow", inserts: []int{1, 2}, newSize: 5, wantEvicted: 0, wantKeys: []int{1, 2}},
		{title: "NonPositiveIgnored", inserts: []int{1, 2}, newSize: 0, wantEvicted: 0, wantKeys: []int{1, 2}},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := single(4)
			for _, k := range tc.inserts {
				c.Add(k, k*10)
			}
			evicted := c.Resize(tc.newSize)
			validate.Equal(tc.wantEvicted, evicted)
			validate.Equal(tc.wantKeys, c.Keys())
		})
	}
}

func (s *LRUSuite) TestPurge() {
	testCases := []struct {
		title   string
		inserts []int
	}{
		{title: "ClearsAll", inserts: []int{1, 2, 3}},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			var purged []int
			c := single(4, WithEvictionCallback[int, int](func(k, _ int) {
				purged = append(purged, k)
			}))
			for _, k := range tc.inserts {
				c.Add(k, k*10)
			}
			c.Purge()
			validate.Equal(0, c.Len())
			validate.Empty(c.Keys())
			validate.ElementsMatch(tc.inserts, purged)
		})
	}
}

func (s *LRUSuite) TestEvictionCallback() {
	testCases := []struct {
		title       string
		size        int
		inserts     []int
		wantEvicted []int
	}{
		{title: "CallbackReceivesEvicted", size: 2, inserts: []int{1, 2, 3, 4}, wantEvicted: []int{1, 2}},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			var evicted []int
			c := single(tc.size, WithEvictionCallback[int, int](func(k, _ int) {
				evicted = append(evicted, k)
			}))
			for _, k := range tc.inserts {
				c.Add(k, k*10)
			}
			validate.Equal(tc.wantEvicted, evicted)
		})
	}
}

func (s *LRUSuite) TestMetrics() {
	testCases := []struct {
		title         string
		size          int
		inserts       []int
		gets          []int
		wantHits      int
		wantMisses    int
		wantEvictions int
		wantItemCount int
	}{
		{
			title:         "CollectsAllSignals",
			size:          2,
			inserts:       []int{1, 2, 3},
			gets:          []int{3, 99},
			wantHits:      1,
			wantMisses:    1,
			wantEvictions: 1,
			wantItemCount: 2,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			m := &fakeMetrics{}
			c := single(tc.size, WithMetrics[int, int](m))
			for _, k := range tc.inserts {
				c.Add(k, k*10)
			}
			for _, k := range tc.gets {
				c.Get(k)
			}
			snap := m.snapshot()
			validate.Equal(len(tc.inserts), snap.setCalls)
			validate.Equal(len(tc.gets), snap.getCalls)
			validate.Equal(tc.wantHits, snap.hits)
			validate.Equal(tc.wantMisses, snap.misses)
			validate.Equal(tc.wantEvictions, snap.evictions)
			validate.Equal(tc.wantItemCount, snap.itemCount)
		})
	}
}

func (s *LRUSuite) TestShardedCapacityNeverExceeded() {
	testCases := []struct {
		title   string
		size    int
		shards  int
		inserts int
	}{
		{title: "HeavyChurn", size: 100, shards: 8, inserts: 500},
		{title: "SingleShard", size: 50, shards: 1, inserts: 500},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := mustNew(tc.size, WithShards[int, int](tc.shards))
			for i := range tc.inserts {
				c.Add(i, i)
			}
			validate.Equal(tc.size, c.Cap())
			validate.LessOrEqual(c.Len(), c.Cap())
			validate.Equal(len(c.Keys()), c.Len())
		})
	}
}

func (s *LRUSuite) TestShardedRetrieval() {
	testCases := []struct {
		title  string
		size   int
		shards int
		count  int
	}{
		{title: "AllPresentUnderCapacity", size: 4096, shards: 16, count: 500},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := mustNew(tc.size, WithShards[int, int](tc.shards))
			for i := range tc.count {
				c.Add(i, i*2)
			}
			validate.Equal(tc.count, c.Len())
			for i := range tc.count {
				got, ok := c.Get(i)
				validate.True(ok)
				validate.Equal(i*2, got)
			}
		})
	}
}

func (s *LRUSuite) TestConcurrentAccess() {
	testCases := []struct {
		title      string
		size       int
		shards     int
		goroutines int
		iterations int
	}{
		{title: "RaceFree", size: 64, shards: 16, goroutines: 8, iterations: 500},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := mustNew(tc.size, WithShards[int, int](tc.shards), WithMetrics[int, int](&fakeMetrics{}))
			var wg sync.WaitGroup
			for g := range tc.goroutines {
				wg.Add(1)
				go func(base int) {
					defer wg.Done()
					for i := range tc.iterations {
						key := (base + i) % 128
						c.Add(key, i)
						c.Get(key)
						if i%7 == 0 {
							c.Remove(key)
						}
					}
				}(g * tc.iterations)
			}
			wg.Wait()
			validate.LessOrEqual(c.Len(), c.Cap())
		})
	}
}

func (s *LRUSuite) TestConcurrentPurge() {
	testCases := []struct {
		title      string
		size       int
		shards     int
		goroutines int
		iterations int
		purges     int
		rounds     int
	}{
		{title: "LenStaysInSync", size: 1024, shards: 4, goroutines: 4, iterations: 500, purges: 200, rounds: 50},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			for range tc.rounds {
				c := mustNew(tc.size, WithShards[int, int](tc.shards))
				var wg sync.WaitGroup
				for g := range tc.goroutines {
					wg.Add(1)
					go func(base int) {
						defer wg.Done()
						for i := range tc.iterations {
							key := base*tc.iterations + i
							c.Add(key, i)
							c.Remove(key)
						}
					}(g)
				}
				wg.Go(func() {
					for range tc.purges {
						c.Purge()
					}
				})
				wg.Wait()

				// Every added key was removed by Remove or Purge, so the cache
				// must be empty and the counter must agree with the contents.
				validate.Equal(0, c.Len())
				validate.Empty(c.Keys())
			}
		})
	}
}

func (s *LRUSuite) TestEvictionCallbackReentrancy() {
	testCases := []struct {
		title     string
		trigger   func(c *Cache[int, int])
		wantCalls int
	}{
		{title: "PurgeCallbackPurgesAgain", trigger: func(c *Cache[int, int]) { c.Purge() }, wantCalls: 3},
		{title: "ResizeCallbackPurges", trigger: func(c *Cache[int, int]) { c.Resize(1) }, wantCalls: 3},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			var calls int
			var c *Cache[int, int]
			c = single(4, WithEvictionCallback[int, int](func(int, int) {
				calls++
				if calls == 1 {
					c.Purge() // must not deadlock: callbacks run outside internal locks
				}
			}))
			c.Add(1, 10)
			c.Add(2, 20)
			c.Add(3, 30)
			tc.trigger(c)
			validate.Equal(tc.wantCalls, calls)
			validate.Equal(0, c.Len())
		})
	}
}

func (s *LRUSuite) TestResizeCapacityFloor() {
	testCases := []struct {
		title   string
		size    int
		shards  int
		newSize int
		wantCap int
	}{
		{title: "BelowShardCountClampsToShards", size: 64, shards: 8, newSize: 3, wantCap: 8},
		{title: "AboveShardCountApplied", size: 64, shards: 8, newSize: 16, wantCap: 16},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			c := mustNew(tc.size, WithShards[int, int](tc.shards))
			c.Resize(tc.newSize)
			validate.Equal(tc.wantCap, c.Cap())
		})
	}
}
