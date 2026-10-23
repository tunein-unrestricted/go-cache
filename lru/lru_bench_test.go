// Copyright 2026 TuneIn, Inc. All rights reserved.
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package lru

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

var benchSizes = []int{256, 1024, 4096, 16384, 65536}

// benchShardSettings compares the size-derived default against the previous
// fixed default of 64 shards.
var benchShardSettings = []struct {
	name string
	opts func() []Option[int, int]
}{
	{name: "default", opts: func() []Option[int, int] { return nil }},
	{name: "shards=64", opts: func() []Option[int, int] { return []Option[int, int]{WithShards[int, int](DefaultShards)} }},
}

func newFilledBenchCache(b *testing.B, size int, opts []Option[int, int]) *Cache[int, int] {
	b.Helper()

	c, err := New(size, opts...)
	if err != nil {
		b.Fatal(err)
	}
	for i := range size {
		c.Add(i, i)
	}
	return c
}

func runParallelBench(b *testing.B, op func(c *Cache[int, int], key int, r *rand.Rand)) {
	b.Helper()

	for _, size := range benchSizes {
		for _, setting := range benchShardSettings {
			b.Run(fmt.Sprintf("size=%d/%s", size, setting.name), func(b *testing.B) {
				c := newFilledBenchCache(b, size, setting.opts())
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					//nolint:gosec // benchmark keys do not need a cryptographic source
					r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
					for pb.Next() {
						op(c, r.IntN(size), r)
					}
				})
			})
		}
	}
}

func BenchmarkParallelGet(b *testing.B) {
	runParallelBench(b, func(c *Cache[int, int], key int, _ *rand.Rand) {
		c.Get(key)
	})
}

func BenchmarkParallelAdd(b *testing.B) {
	runParallelBench(b, func(c *Cache[int, int], key int, _ *rand.Rand) {
		c.Add(key, key)
	})
}

// BenchmarkParallelMixed runs 90% Get and 10% Add.
func BenchmarkParallelMixed(b *testing.B) {
	runParallelBench(b, func(c *Cache[int, int], key int, r *rand.Rand) {
		if r.IntN(10) == 0 {
			c.Add(key, key)
			return
		}
		c.Get(key)
	})
}
