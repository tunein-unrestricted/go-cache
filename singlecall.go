// Package cache provides in-memory lru (discards the least recently used items first) cache functionality
//
//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

// This module provides a duplicate function call suppression
// mechanism.
// original idea taken from here:
// https://github.com/bluele/gcache/blob/master/singleflight.go

import (
	"sync"
)

// call is an in-flight or completed Do call
type call[TValue any] struct {
	wg  sync.WaitGroup
	val TValue
	err error
}

// Group represents a class of work and forms a namespace in which
// units of work can be executed with duplicate suppression.
type Group[TKey comparable, TValue any] struct {
	c   Cacher[TKey, TValue]
	mtx sync.Mutex             // protects m
	m   map[TKey]*call[TValue] // lazily initialized
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
func (g *Group[TKey, TValue]) Do(key TKey, fn func() (TValue, error)) (res TValue, dupl bool, err error) {
	g.mtx.Lock()
	v, err := g.c.get(key)
	if err == nil {
		g.mtx.Unlock()
		return v, false, nil
	}
	if g.m == nil {
		g.m = make(map[TKey]*call[TValue])
	}
	if c, ok := g.m[key]; ok {
		g.mtx.Unlock()
		c.wg.Wait()
		return c.val, false, c.err
	}
	c := new(call[TValue])
	c.wg.Add(1)
	g.m[key] = c
	g.mtx.Unlock()
	v, err = g.call(c, key, fn)
	return v, true, err
}

func (g *Group[TKey, TValue]) call(c *call[TValue], key TKey, fn func() (TValue, error)) (TValue, error) {
	c.val, c.err = fn()
	c.wg.Done()

	g.mtx.Lock()
	delete(g.m, key)
	g.mtx.Unlock()

	return c.val, c.err
}
