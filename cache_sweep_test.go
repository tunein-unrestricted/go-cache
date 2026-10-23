// Copyright 2025 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"slices"
	"time"
)

// TestSweepIntervalCappedAtMaxSweepInterval covers the scenario the cap exists
// for: a default TTL of one hour and a short-TTL item that expires after a
// sweep has already run must still be swept within maxSweepInterval.
func (s *CacheSuite) TestSweepIntervalCappedAtMaxSweepInterval() {
	testCases := []struct {
		title      string
		defaultTTL time.Duration
		itemTTL    time.Duration
	}{
		{title: "HourDefaultTTLSweptWithinMaxInterval", defaultTTL: time.Hour, itemTTL: 20 * time.Millisecond},
		{title: "NoDefaultTTLSweptWithinMaxInterval", defaultTTL: 0, itemTTL: 20 * time.Millisecond},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			require := s.Require()
			cc := New[int, int](tc.defaultTTL)
			swept := func(key int) func() bool {
				return func() bool { return !slices.Contains(cc.Keys(false), key) }
			}

			// The first sweep stamps cleanedAt.
			cc.SetWithExpire(1, 1, tc.itemTTL)
			time.Sleep(2 * tc.itemTTL)
			cc.SetWithExpire(2, 2, time.Hour)
			require.Eventually(swept(1), time.Second, time.Millisecond)
			sweptAt := time.Now()

			// A second item expires after that sweep; Set must not sweep it
			// before the interval has passed.
			cc.SetWithExpire(3, 3, tc.itemTTL)
			time.Sleep(2 * tc.itemTTL)
			cc.SetWithExpire(4, 4, time.Hour)
			time.Sleep(2 * tc.itemTTL)
			require.Contains(cc.Keys(false), 3)

			time.Sleep(maxSweepInterval - time.Since(sweptAt) + 10*time.Millisecond)
			cc.SetWithExpire(5, 5, time.Hour)
			require.Eventually(swept(3), time.Second, time.Millisecond)
		})
	}
}

func (s *CacheSuite) TestSweepInterval() {
	testCases := []struct {
		title        string
		defaultTTL   time.Duration
		sweepCost    time.Duration
		wantInterval time.Duration
	}{
		{title: "NoDefaultTTLUsesMaxInterval", defaultTTL: 0, wantInterval: maxSweepInterval},
		{title: "ShortDefaultTTLUsesTTL", defaultTTL: 100 * time.Millisecond, wantInterval: 100 * time.Millisecond},
		{title: "LongDefaultTTLUsesMaxInterval", defaultTTL: time.Hour, wantInterval: maxSweepInterval},
		{title: "CheapSweepKeepsBaseInterval", defaultTTL: time.Hour, sweepCost: time.Millisecond, wantInterval: maxSweepInterval},
		{title: "ExpensiveSweepStretchesInterval", defaultTTL: time.Hour, sweepCost: 20 * time.Millisecond, wantInterval: 2 * time.Second},
		{title: "ExpensiveSweepStretchesShortTTLInterval", defaultTTL: 100 * time.Millisecond, sweepCost: 10 * time.Millisecond, wantInterval: time.Second},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			cc := New[int, int](tc.defaultTTL)
			cc.sweepCost = tc.sweepCost
			s.Equal(tc.wantInterval, cc.sweepInterval())
		})
	}
}

func (s *CacheSuite) TestSweepRecordsCost() {
	cc := New[int, int](time.Hour)
	for i := range 10_000 {
		cc.Set(i, i)
	}

	cc.deleteExpired()

	cc.mtx.RLock()
	defer cc.mtx.RUnlock()
	s.Positive(cc.sweepCost)
}
