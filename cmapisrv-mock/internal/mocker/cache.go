// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/lock"
)

type cache[T store.Key] struct {
	mu     lock.RWMutex
	keys   map[string]int
	values []T
}

func newCache[T store.Key]() cache[T] {
	return cache[T]{keys: make(map[string]int)}
}

func (c *cache[T]) Get(rnd *random) T {
	c.mu.RLock()
	defer c.mu.RUnlock()

	id := rnd.Index(len(c.values))
	return c.values[id]
}

func (c *cache[T]) AlmostEmpty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.values) <= 3
}

func (c *cache[T]) Add(value T) bool {
	return c.upsert(value, false)
}

func (c *cache[T]) Upsert(value T) {
	c.upsert(value, true)
}

func (c *cache[T]) Remove(rnd *random) T {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := rnd.Index(len(c.values))
	value := c.values[id]

	c.values[id] = c.values[len(c.values)-1]
	c.keys[c.values[id].GetKeyName()] = id

	c.values = c.values[:len(c.values)-1]
	delete(c.keys, value.GetKeyName())

	return value
}

func (c *cache[T]) upsert(value T, overwrite bool) (stored bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := value.GetKeyName()
	if idx, ok := c.keys[key]; ok {
		if overwrite {
			c.values[idx] = value
		}

		return overwrite
	}

	c.keys[key] = len(c.values)
	c.values = append(c.values, value)
	return true
}
