// Package cache provides in-memory lru (discards the least recently used items first) cache functionality
//
//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

// ErrNotFound returned is nothing has been found by the provided key
var ErrNotFound = errors.New("item has not been found in the cache")

// Cache provides in-memory lru (discards the least recently used items first) cache functionality
type Cache[TKey comparable, TValue any] struct {
	mtx              sync.RWMutex
	umtx             sync.RWMutex
	items            map[TKey]cacheItem[TValue]
	ttl              time.Duration
	maxSize          int
	lruList          *list.List
	lruIndex         map[TKey]*list.Element
	addedFunc        AddedFunc[TKey, TValue]
	evictedFunc      EvictedFunc[TKey, TValue]
	loaderExpireFunc LoaderExpireFunc[TKey, TValue]
	loadGroup        Group[TKey, TValue]
}

// New returns reference to typed  in-memory cache instance
func New[TKey comparable, TValue any](exp time.Duration) *Cache[TKey, TValue] {
	cc := &Cache[TKey, TValue]{}
	cc.init(exp)
	return cc
}

// NewWithOptions returns a new Cache configured by the provided functional options.
//
//	cc := cache.NewWithOptions(
//	    cache.WithExpiration[string, int](5 * time.Minute),
//	    cache.WithMaxSize[string, int](1000),
//	)
func NewWithOptions[TKey comparable, TValue any](opts ...Option[TKey, TValue]) *Cache[TKey, TValue] {
	cc := &Cache[TKey, TValue]{}
	cc.initItems()
	for _, opt := range opts {
		opt(cc)
	}
	return cc
}

// Set a new key-value pair
func (c *Cache[TKey, TValue]) Set(key TKey, value TValue) {
	c.umtx.RLock()
	defer c.umtx.RUnlock()
	c.set(key, value, 0)
}

// SetWithExpire a new key-value pair with an expiration time. 0 = never expire
func (c *Cache[TKey, TValue]) SetWithExpire(key TKey, value TValue, expiration time.Duration) {
	if expiration < 0 {
		expiration = c.ttl
	}
	c.umtx.RLock()
	defer c.umtx.RUnlock()
	c.set(key, value, expiration)
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
	c.umtx.Lock()
	defer c.umtx.Unlock()
	v, _ := c.get(key)
	c.set(key, calc(v), expiration)
}

// Get a value from cache pool using key if it exists.
// If it does not exists key and has LoaderFunc,
// generate a value using `LoaderFunc` method returns value.
func (c *Cache[TKey, TValue]) Get(key TKey) (TValue, error) {
	v, err := c.get(key)
	if err == ErrNotFound {
		return c.getWithLoader(key, true)
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
	if elem, exists := c.lruIndex[key]; exists {
		c.lruList.Remove(elem)
		delete(c.lruIndex, key)
	}
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

// StartCleaner starts a background goroutine that removes expired items from
// the cache at the given interval. Call the returned stop function to terminate
// the goroutine when the cache is no longer needed.
//
// Without a cleaner, expired items are only removed lazily on Get. In
// write-heavy workloads where many unique keys are written with a TTL but never
// read, expired entries accumulate in memory indefinitely.
func (c *Cache[TKey, TValue]) StartCleaner(interval time.Duration) (stop func()) {
	quit := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				c.deleteExpired()
			}
		}
	}()
	return sync.OnceFunc(func() { close(quit) })
}

func (c *Cache[TKey, TValue]) deleteExpired() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for key, item := range c.items {
		if item.expired() {
			delete(c.items, key)
			if elem, exists := c.lruIndex[key]; exists {
				c.lruList.Remove(elem)
				delete(c.lruIndex, key)
			}
		}
	}
}

func (c *Cache[TKey, TValue]) init(exp time.Duration) {
	c.initItems()
	c.ttl = exp
}

func (c *Cache[TKey, TValue]) set(key TKey, value TValue, ttl time.Duration) {
	if ttl < 1 {
		ttl = c.ttl
	}
	item := cacheItem[TValue]{
		ttl:   ttl,
		val:   value,
		added: time.Now(),
	}

	var (
		evictedKey TKey
		evictedVal TValue
		didEvict   bool
	)

	c.mtx.Lock()
	_, exists := c.items[key]
	c.items[key] = item

	if c.maxSize > 0 {
		if exists {
			if elem, ok := c.lruIndex[key]; ok {
				c.lruList.MoveToFront(elem)
			} else {
				c.lruIndex[key] = c.lruList.PushFront(key)
			}
		} else {
			c.lruIndex[key] = c.lruList.PushFront(key)
			if c.lruList.Len() > c.maxSize {
				if lruElem := c.lruList.Back(); lruElem != nil {
					evictedKey = lruElem.Value.(TKey)
					evictedVal = c.items[evictedKey].val
					delete(c.items, evictedKey)
					delete(c.lruIndex, evictedKey)
					c.lruList.Remove(lruElem)
					didEvict = true
				}
			}
		}
	}
	c.mtx.Unlock()

	if c.addedFunc != nil {
		c.addedFunc(key, value)
	}
	if didEvict && c.evictedFunc != nil {
		c.evictedFunc(evictedKey, evictedVal)
	}
}

func (c *Cache[TKey, TValue]) initItems() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.items = make(map[TKey]cacheItem[TValue])
	c.lruList = list.New()
	c.lruIndex = make(map[TKey]*list.Element)
	c.loadGroup = Group[TKey, TValue]{
		c: c,
	}
}

// load a new value using by specified key.
func (c *Cache[TKey, TValue]) load(key TKey, cb func(TValue,
	*time.Duration, error) (TValue, error), isWait bool,
) (val TValue, isLoaded bool, err error) {
	v, called, err := c.loadGroup.Do(key, func() (v TValue, e error) {
		return cb(c.loaderExpireFunc(key))
	}, isWait)
	if err != nil {
		var def TValue
		return def, called, err
	}
	return v, called, nil
}

func (c *Cache[TKey, TValue]) get(key TKey) (TValue, error) {
	if c.maxSize > 0 {
		return c.getWithLRU(key)
	}
	c.mtx.RLock()
	item, ok := c.items[key]
	c.mtx.RUnlock()
	if ok {
		if !item.expired() {
			return item.val, nil
		}
		// Re-read under a write lock before deleting: another goroutine may have
		// Set a fresh value for this key between the RUnlock above and here.
		c.mtx.Lock()
		if current, exists := c.items[key]; exists && current.expired() {
			delete(c.items, key)
		}
		c.mtx.Unlock()
	}
	var def TValue
	return def, ErrNotFound
}

func (c *Cache[TKey, TValue]) getWithLRU(key TKey) (TValue, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	item, ok := c.items[key]
	if ok {
		if !item.expired() {
			if elem, exists := c.lruIndex[key]; exists {
				c.lruList.MoveToFront(elem)
			}
			return item.val, nil
		}
		delete(c.items, key)
		if elem, exists := c.lruIndex[key]; exists {
			c.lruList.Remove(elem)
			delete(c.lruIndex, key)
		}
	}
	var def TValue
	return def, ErrNotFound
}

func (c *Cache[TKey, TValue]) getWithLoader(key TKey, isWait bool) (TValue, error) {
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
		c.set(key, v, ttl)
		return v, nil
	}, isWait)
	if err != nil {
		return def, err
	}
	return value, nil
}
