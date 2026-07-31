// Package lru provides a fast, generic, fixed-capacity LRU cache with optional metrics collection.
//
//	Copyright 2026 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package lru

import "sync"

type shard[K comparable, V any] struct {
	mu    sync.RWMutex
	size  int
	items map[K]*entry[K, V]
	root  entry[K, V]
}

func newShard[K comparable, V any](size int) *shard[K, V] {
	sh := &shard[K, V]{
		size:  size,
		items: make(map[K]*entry[K, V], size),
	}
	sh.root.prev = &sh.root
	sh.root.next = &sh.root
	return sh
}

func (sh *shard[K, V]) add(key K, value V) (res addOutcome[K, V]) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if ent, ok := sh.items[key]; ok {
		ent.value = value
		sh.moveToFront(ent)
		return res
	}

	if len(sh.items) >= sh.size {
		oldest := sh.root.prev
		sh.unlink(oldest)
		delete(sh.items, oldest.key)
		res.evicted = true
		res.evictKey, res.evictVal = oldest.key, oldest.value
		oldest.key, oldest.value = key, value
		sh.pushFront(oldest)
		sh.items[key] = oldest
		return res
	}

	ent := &entry[K, V]{key: key, value: value}
	sh.pushFront(ent)
	sh.items[key] = ent
	res.sizeDelta = 1
	return res
}

func (sh *shard[K, V]) get(key K) (value V, ok bool) {
	sh.mu.Lock()
	ent, ok := sh.items[key]
	if ok {
		sh.moveToFront(ent)
		value = ent.value
	}
	sh.mu.Unlock()
	return value, ok
}

func (sh *shard[K, V]) peek(key K) (value V, ok bool) {
	sh.mu.RLock()
	ent, ok := sh.items[key]
	if ok {
		value = ent.value
	}
	sh.mu.RUnlock()
	return value, ok
}

func (sh *shard[K, V]) contains(key K) bool {
	sh.mu.RLock()
	_, ok := sh.items[key]
	sh.mu.RUnlock()
	return ok
}

func (sh *shard[K, V]) remove(key K) (value V, ok bool) {
	sh.mu.Lock()
	ent, ok := sh.items[key]
	if ok {
		sh.unlink(ent)
		delete(sh.items, key)
		value = ent.value
	}
	sh.mu.Unlock()
	return value, ok
}

func (sh *shard[K, V]) appendKeys(dst *[]K) {
	sh.mu.RLock()
	for e := sh.root.prev; e != &sh.root; e = e.prev {
		*dst = append(*dst, e.key)
	}
	sh.mu.RUnlock()
}

func (sh *shard[K, V]) appendValues(dst *[]V) {
	sh.mu.RLock()
	for e := sh.root.prev; e != &sh.root; e = e.prev {
		*dst = append(*dst, e.value)
	}
	sh.mu.RUnlock()
}

func (sh *shard[K, V]) resize(newSize int) []kv[K, V] {
	sh.mu.Lock()
	sh.size = newSize
	var removed []kv[K, V]
	for len(sh.items) > newSize {
		oldest := sh.root.prev
		sh.unlink(oldest)
		delete(sh.items, oldest.key)
		removed = append(removed, kv[K, V]{key: oldest.key, val: oldest.value})
	}
	sh.mu.Unlock()
	return removed
}

func (sh *shard[K, V]) purge(collect bool) (removed []kv[K, V], n int) {
	sh.mu.Lock()
	n = len(sh.items)
	if collect {
		removed = make([]kv[K, V], 0, n)
		for e := sh.root.prev; e != &sh.root; e = e.prev {
			removed = append(removed, kv[K, V]{key: e.key, val: e.value})
		}
	}
	sh.items = make(map[K]*entry[K, V], sh.size)
	sh.root.prev = &sh.root
	sh.root.next = &sh.root
	sh.mu.Unlock()
	return removed, n
}

func (sh *shard[K, V]) pushFront(e *entry[K, V]) {
	e.prev = &sh.root
	e.next = sh.root.next
	sh.root.next.prev = e
	sh.root.next = e
}

func (sh *shard[K, V]) unlink(e *entry[K, V]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.prev = nil
	e.next = nil
}

func (sh *shard[K, V]) moveToFront(e *entry[K, V]) {
	if sh.root.next == e {
		return
	}
	e.prev.next = e.next
	e.next.prev = e.prev
	e.prev = &sh.root
	e.next = sh.root.next
	sh.root.next.prev = e
	sh.root.next = e
}
