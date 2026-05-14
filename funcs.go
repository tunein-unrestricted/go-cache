// Package cache provides in-memory lru (discards the least recently used items first) cache functionality
//
//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"time"
)

type (
	// Option is a functional option for configuring a Cache at construction time via NewWithOptions.
	Option[TKey comparable, TValue any] func(*Cache[TKey, TValue])
	// AddedFunc function that should be called when new item is added to the cache
	AddedFunc[TKey comparable, TValue any] func(TKey, TValue)
	// EvictedFunc function that should be called when an item is evicted from the cache due to capacity limits
	EvictedFunc[TKey comparable, TValue any] func(TKey, TValue)
	// LoaderFunc function that is used to load missing or expired cache entry
	LoaderFunc[TKey comparable, TValue any] func(TKey) (TValue, error)
	// LoaderExpireFunc is called when item is expired
	LoaderExpireFunc[TKey comparable, TValue any] func(TKey) (TValue, *time.Duration, error)
)

// WithExpiration returns an Option that sets the default TTL for cache entries.
// A value of 0 means entries never expire.
func WithExpiration[TKey comparable, TValue any](d time.Duration) Option[TKey, TValue] {
	return func(c *Cache[TKey, TValue]) {
		c.ttl = d
	}
}

// WithMaxSize returns an Option that sets the maximum number of items the cache can hold.
// When the cache is full, the least recently used item is evicted to make room.
// A value of 0 (default) disables eviction.
func WithMaxSize[TKey comparable, TValue any](size int) Option[TKey, TValue] {
	return func(c *Cache[TKey, TValue]) {
		c.maxSize = size
	}
}

// LoaderFunc: create a new value with this function if cached value is expired.
func (c *Cache[TKey, TValue]) LoaderFunc(loaderFunc LoaderFunc[TKey, TValue]) *Cache[TKey, TValue] {
	c.loaderExpireFunc = func(k TKey) (TValue, *time.Duration, error) {
		v, err := loaderFunc(k)
		return v, nil, err
	}
	return c
}

// LoaderExpireFunc - loader function with expiration, create a new value with this function if cached value is expired.
// If nil returned instead of time.Duration from loaderExpireFunc than value will never expire.
func (c *Cache[TKey, TValue]) LoaderExpireFunc(loaderExpireFunc LoaderExpireFunc[TKey, TValue]) *Cache[TKey, TValue] {
	c.loaderExpireFunc = loaderExpireFunc
	return c
}

// AddedFunc - if provided, this function will be called after each new value is added to the cache
func (c *Cache[TKey, TValue]) AddedFunc(addedFunc AddedFunc[TKey, TValue]) *Cache[TKey, TValue] {
	c.addedFunc = addedFunc
	return c
}

// MaxSize sets the maximum number of items the cache can hold.
// When the cache is full, the least recently used item is evicted to make room.
// A value of 0 (default) disables eviction.
func (c *Cache[TKey, TValue]) MaxSize(size int) *Cache[TKey, TValue] {
	c.maxSize = size
	return c
}

// EvictedFunc - if provided, this function will be called each time an item is evicted due to capacity limits
func (c *Cache[TKey, TValue]) EvictedFunc(evictedFunc EvictedFunc[TKey, TValue]) *Cache[TKey, TValue] {
	c.evictedFunc = evictedFunc
	return c
}
