// Package cache provides an in-memory cache whose items expire by TTL, with lazy loading and duplicate-call suppression. It has no size limit.
//
//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound returned is nothing has been found by the provided key
var ErrNotFound = errors.New("item has not been found in the cache")

const maxSweepInterval = time.Second

// Cache is an in-memory cache whose items expire by TTL. It has no size limit; use the lru package for a fixed-capacity cache.
type Cache[TKey comparable, TValue any] struct {
	mtx              sync.RWMutex
	umtx             sync.RWMutex
	items            map[TKey]cacheItem[TValue]
	ttl              time.Duration
	cleanedAt        time.Time // last time expired entries were swept; guarded by mtx
	nextExpiry       time.Time
	sweeping         bool
	addedFunc        AddedFunc[TKey, TValue]
	loaderExpireFunc LoaderExpireFunc[TKey, TValue]
	loadGroup        Group[TKey, TValue]
}

// New returns reference to typed  in-memory cache instance
func New[TKey comparable, TValue any](exp time.Duration) *Cache[TKey, TValue] {
	cc := &Cache[TKey, TValue]{}
	cc.init(exp)
	return cc
}

// Set a new key-value pair
func (c *Cache[TKey, TValue]) Set(key TKey, value TValue) {
	c.setWithUpdateMutex(key, value, 0)
	if c.addedFunc != nil {
		c.addedFunc(key, value)
	}
}

// SetWithExpire a new key-value pair with an expiration time. 0 = never expire
func (c *Cache[TKey, TValue]) SetWithExpire(key TKey, value TValue, expiration time.Duration) {
	if expiration < 0 {
		expiration = c.ttl
	}
	c.setWithUpdateMutex(key, value, expiration)
	if c.addedFunc != nil {
		c.addedFunc(key, value)
	}
}

// Update atomically updates a value using the given function to calculate the new value
// Expiration time is updated once a value is updated
func (c *Cache[TKey, TValue]) Update(key TKey, calc func(v TValue) TValue) {
	c.UpdateWithExpire(key, calc, 0)
}

// UpdateWithExpire same as Update, but allows to set expiration
func (c *Cache[TKey, TValue]) UpdateWithExpire(key TKey, calc func(v TValue) TValue, expiration time.Duration) {
	if expiration < 0 {
		expiration = c.ttl
	}
	var newVal TValue
	func() {
		c.umtx.Lock()
		defer c.umtx.Unlock()
		v, _ := c.get(key)
		newVal = calc(v)
		c.set(key, newVal, expiration)
	}()
	if c.addedFunc != nil {
		c.addedFunc(key, newVal)
	}
}

// Get a value from cache pool using key if it exists.
// If it does not exists key and has LoaderFunc,
// generate a value using `LoaderFunc` method returns value.
func (c *Cache[TKey, TValue]) Get(key TKey) (TValue, error) {
	v, err := c.get(key)
	if errors.Is(err, ErrNotFound) {
		return c.getWithLoader(key)
	}
	return v, err
}

// Has checks if key exists in cache
func (c *Cache[TKey, TValue]) Has(key TKey) bool {
	c.mtx.RLock()
	item, ok := c.items[key]
	c.mtx.RUnlock()
	if !ok {
		return false
	}
	return !item.expired()
}

// Remove removes the provided key from the cache.
func (c *Cache[TKey, TValue]) Remove(key TKey) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	delete(c.items, key)
}

// Keys returns a slice of the keys in the cache.
func (c *Cache[TKey, TValue]) Keys(checkExpired bool) []TKey {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	keys := make([]TKey, 0, len(c.items))
	for k, item := range c.items {
		if !checkExpired || !item.expired() {
			keys = append(keys, k)
		}
	}
	return keys
}

// Len returns the number of items in the cache.
func (c *Cache[TKey, TValue]) Len(checkExpired bool) int {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	if !checkExpired {
		return len(c.items)
	}
	var length int
	for _, item := range c.items {
		if !item.expired() {
			length++
		}
	}
	return length
}

// Purge completely clears the cache
func (c *Cache[TKey, TValue]) Purge() {
	c.initItems()
}

func (c *Cache[TKey, TValue]) init(exp time.Duration) {
	c.initItems()
	c.ttl = exp
	c.loadGroup = Group[TKey, TValue]{
		c: c,
	}
}

func (c *Cache[TKey, TValue]) set(key TKey, value TValue, ttl time.Duration) {
	if ttl < 1 {
		ttl = c.ttl
	}
	now := time.Now()
	item := cacheItem[TValue]{
		ttl:   ttl,
		val:   value,
		added: now,
	}

	c.mtx.Lock()
	c.items[key] = item
	if ttl > 0 {
		c.trackExpiry(now.Add(ttl))
	}
	runCleanup := c.sweepDue(now)
	if runCleanup {
		c.sweeping = true
		c.cleanedAt = now
	}
	c.mtx.Unlock()

	if runCleanup {
		go c.deleteExpired()
	}
}

func (c *Cache[TKey, TValue]) trackExpiry(exp time.Time) {
	if c.nextExpiry.IsZero() || exp.Before(c.nextExpiry) {
		c.nextExpiry = exp
	}
}

func (c *Cache[TKey, TValue]) sweepDue(now time.Time) bool {
	return !c.sweeping && !c.nextExpiry.IsZero() && !now.Before(c.nextExpiry) && now.Sub(c.cleanedAt) >= c.sweepInterval()
}

func (c *Cache[TKey, TValue]) sweepInterval() time.Duration {
	if c.ttl > 0 && c.ttl < maxSweepInterval {
		return c.ttl
	}
	return maxSweepInterval
}

// deleteExpired removes all expired entries from the cache.
// It is called in a background goroutine triggered by set.
func (c *Cache[TKey, TValue]) deleteExpired() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.nextExpiry = time.Time{}
	for k, v := range c.items {
		if v.expired() {
			delete(c.items, k)
			continue
		}
		if v.ttl > 0 {
			c.trackExpiry(v.added.Add(v.ttl))
		}
	}
	c.sweeping = false
}

func (c *Cache[TKey, TValue]) initItems() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.items = make(map[TKey]cacheItem[TValue])
	c.nextExpiry = time.Time{}
}

// load a new value using by specified key.
func (c *Cache[TKey, TValue]) load(key TKey, cb func(TValue, *time.Duration, error) (TValue, error)) (val TValue, isLoaded bool, err error) {
	v, called, err := c.loadGroup.Do(key, func() (v TValue, e error) {
		return cb(c.loaderExpireFunc(key))
	})
	if err != nil {
		var def TValue
		return def, called, err
	}
	return v, called, nil
}

func (c *Cache[TKey, TValue]) get(key TKey) (TValue, error) {
	c.mtx.RLock()
	item, ok := c.items[key]
	c.mtx.RUnlock()
	if ok {
		if !item.expired() {
			return item.val, nil
		}
		// Re-read under write lock before deleting: a concurrent Set may have
		// stored a fresh value between our RUnlock and Lock, so we must not
		// delete unless the entry is still expired.
		c.mtx.Lock()
		if current, exists := c.items[key]; exists && current.expired() {
			delete(c.items, key)
		}
		c.mtx.Unlock()
	}
	var def TValue
	return def, ErrNotFound
}

func (c *Cache[TKey, TValue]) setWithUpdateMutex(key TKey, value TValue, ttl time.Duration) {
	c.umtx.RLock()
	defer c.umtx.RUnlock()
	c.set(key, value, ttl)
}

func (c *Cache[TKey, TValue]) getWithLoader(key TKey) (TValue, error) {
	var def TValue
	if c.loaderExpireFunc == nil {
		return def, ErrNotFound
	}
	value, _, err := c.load(key, func(v TValue, expiration *time.Duration, e error) (TValue, error) {
		if e != nil {
			return def, e
		}
		ttl := 0 * time.Second
		if expiration != nil {
			ttl = *expiration
		}
		c.setWithUpdateMutex(key, v, ttl)
		if c.addedFunc != nil {
			c.addedFunc(key, v)
		}
		return v, nil
	})
	if err != nil {
		return def, err
	}
	return value, nil
}
