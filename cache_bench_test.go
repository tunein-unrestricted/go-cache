// Copyright 2025 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"fmt"
	"testing"
	"time"
)

var benchSizes = []int{10_000, 100_000, 1_000_000}

func newFilledCache(size int, defaultTTL time.Duration) *Cache[int, int] {
	cc := New[int, int](defaultTTL)
	for i := range size {
		cc.Set(i, i)
	}
	return cc
}

func BenchmarkSet(b *testing.B) {
	for _, ttl := range []time.Duration{0, time.Hour} {
		b.Run(fmt.Sprintf("defaultTTL=%s", ttl), func(b *testing.B) {
			cc := newFilledCache(100_000, ttl)
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					cc.Set(i%100_000, i)
					i++
				}
			})
		})
	}
}

func BenchmarkGet(b *testing.B) {
	cc := newFilledCache(100_000, time.Hour)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = cc.Get(i % 100_000)
			i++
		}
	})
}

// BenchmarkSweep reports how long one sweep holds the write lock for a cache
// of the given size; every item is live, so nothing is deleted.
func BenchmarkSweep(b *testing.B) {
	for _, size := range benchSizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			cc := newFilledCache(size, time.Hour)
			b.ResetTimer()
			for range b.N {
				cc.deleteExpired()
			}
		})
	}
}

// BenchmarkSetChurnWithItemTTL writes unique keys with a short per-item TTL
// under a long default TTL and reports how many items are still in memory at the end.
func BenchmarkSetChurnWithItemTTL(b *testing.B) {
	for _, defaultTTL := range []time.Duration{0, time.Hour} {
		b.Run(fmt.Sprintf("defaultTTL=%s", defaultTTL), func(b *testing.B) {
			cc := New[int, int](defaultTTL)
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				cc.SetWithExpire(i, i, time.Millisecond)
			}
			b.StopTimer()
			b.ReportMetric(float64(cc.Len(false)), "items-in-memory")
		})
	}
}

// BenchmarkGetDuringSweepChurn measures Get latency while a writer keeps
// replacing short-TTL items, so sweeps run in the background.
func BenchmarkGetDuringSweepChurn(b *testing.B) {
	for _, size := range benchSizes[:2] {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			cc := newFilledCache(size, time.Hour)
			stop := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				for i := 0; ; i++ {
					select {
					case <-stop:
						return
					default:
						cc.SetWithExpire(size+i, i, time.Millisecond)
					}
				}
			}()
			b.ResetTimer()
			for i := range b.N {
				_, _ = cc.Get(i % size)
			}
			b.StopTimer()
			close(stop)
			<-done
		})
	}
}
