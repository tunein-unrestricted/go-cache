// Package cache provides an in-memory cache whose items expire by TTL, with lazy loading and duplicate-call suppression. It has no size limit.
//
//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import "time"

type cacheItem[V any] struct {
	val   V
	ttl   time.Duration
	added time.Time
}

func (c cacheItem[V]) expired() bool {
	if c.ttl <= 0 {
		return false
	}

	return time.Since(c.added) > c.ttl
}
