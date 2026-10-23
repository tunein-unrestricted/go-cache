// Package cache provides an in-memory cache whose items expire by TTL, with lazy loading and duplicate-call suppression. It has no size limit.
//
//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"time"
)

// Cacher represents cache entity
type Cacher[TKey comparable, TValue any] interface {
	// Set inserts or updates the specified key-value pair.
	Set(key TKey, value TValue)
	// SetWithExpire inserts or updates the specified key-value pair with an expiration time.
	// An expiration of 0 means the item never expires; a negative expiration uses the cache's default TTL.
	SetWithExpire(key TKey, value TValue, expiration time.Duration)
	// Get returns the value for the specified key if it is present in the cache.
	// If the key is not present in the cache and the cache has LoaderFunc,
	// invoke the `LoaderFunc` function and inserts the key-value pair in the cache.
	// If the key is not present in the cache and the cache does not have a LoaderFunc,
	// return KeyNotFoundError.
	Get(key TKey) (TValue, error)
	get(key TKey) (TValue, error)
	// Remove removes the specified key from the cache if the key is present.
	Remove(key TKey)
	// Purge removes all key-value pairs from the cache.
	Purge()
	// Keys returns a slice containing all keys in the cache.
	Keys(checkExpired bool) []TKey
	// Len returns the number of items in the cache.
	Len(checkExpired bool) int
	// Has returns true if the key exists in the cache.
	Has(key TKey) bool
}
