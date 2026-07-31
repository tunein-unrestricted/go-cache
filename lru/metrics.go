// Package lru provides a fast, generic, fixed-capacity LRU cache with optional metrics collection.
//
//	Copyright 2026 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package lru

import "time"

// MetricsCollector receives operational metrics from a Cache instance. Its
// methods must be safe for concurrent use and are always invoked without the
// cache's internal lock held.
type MetricsCollector interface {
	// ObserveGetLatency reports the wall-clock duration of a single Get call.
	ObserveGetLatency(d time.Duration)
	// ObserveSetLatency reports the wall-clock duration of a single Add call.
	ObserveSetLatency(d time.Duration)
	// RecordHit records a Get call that found the requested key.
	RecordHit()
	// RecordMiss records a Get call that did not find the requested key.
	RecordMiss()
	// RecordEviction records a single entry removed because the cache was full.
	RecordEviction()
	// SetItemCount reports the current total number of entries in the cache.
	SetItemCount(n int)
}
