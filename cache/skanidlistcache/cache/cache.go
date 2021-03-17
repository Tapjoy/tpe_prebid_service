// Package cache provides an in memory bounded cache with configurable TTL
// It is appropriate to use when you have a well defined key set with read heavy access
package cache

import (
	"sync"
	"time"
)

const (
	DefaultTTL      time.Duration = 5 * time.Minute
	DefaultCapacity int           = 10000
)

type item struct {
	value      interface{}
	expiration int64
}

func newItem(v interface{}, ttl time.Duration) item {
	e := time.Now().Add(ttl).UnixNano()

	return item{
		value:      v,
		expiration: e,
	}
}

func (i item) expired() bool {
	return time.Now().UnixNano() > i.expiration
}

type Cache struct {
	mu          sync.RWMutex
	items       map[string]item
	ttl         time.Duration
	capacity    int
	onCacheHit  func(string, interface{})
	onCacheMiss func(string)
	onSet       func(int, int, string)
	onCleanup   func(int, int, []string)
}

// Create a new cache with a configurable TTL and capacity
// You should have a good understanding of the maximum number of keys
// as well as the size of the values to store in order to calculate
// the maximum cache resource utilization.
//
// Many of the implementation choices assume a fixed smallish maxiumum number of keys.
// If you DO NOT have a solid understanding of the maximum number of keys
// and size of values to be stored, DO NOT USE this cache. You may need a more general
// purpose caching library or an out of process cache like redis or memcached.
func New(ttl time.Duration, capacity int) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}

	if capacity <= 0 {
		capacity = DefaultCapacity
	}

	return &Cache{
		items:    make(map[string]item),
		ttl:      ttl,
		capacity: capacity,
	}
}

// Get retrieves an item from the cache if it exits
//
// DANGER: this method involves manual synchonization using a sync.RWMutex
// Be very careful when modifying this method to avoid synchronization issues
// Better yet, do not modify it (except for bug fixes)
//
// The lock should be as fine grained as possible
// The lock should NEVER be held for user defined callbacks
func (c *Cache) Get(k string) (interface{}, bool) {
	c.mu.RLock()

	item, found := c.items[k]

	if !found || item.expired() {
		// unlock before executing callback
		c.mu.RUnlock()
		if c.onCacheMiss != nil {
			c.onCacheMiss(k)
		}

		return nil, false
	}

	// unlock before executing callback
	c.mu.RUnlock()
	if c.onCacheHit != nil {
		c.onCacheHit(k, item.value)
	}

	return item.value, true
}

// Set stores an item in the cache if there is remaining capacity
//
// If the cache is at capacity when a new item is attempted to be stored,
// expired entries will be removed to make room.
//
// If the cache is at capacity and there are no expired entries, no new
// items will be added to the cache. This choice was made rather than a
// more complicated eviction scheme (like LRU) for simplicity in our specific
// use case. Because the maximum number of keys is well known, the capacity can
// be configured large enough that cleanup should rarely be neccessary.
//
// DANGER: this method involves manual synchonization using a sync.RWMutex
// Be very careful when modifying this method to avoid synchronization issues
// Better yet, do not modify it (except for bug fixes)
//
// The lock should be as fine grained as possible
// The lock should NEVER be held for user defined callbacks
func (c *Cache) Set(k string, v interface{}) {
	c.mu.Lock()

	// the onCleanup callback cannot be executed in the cleanup for loop
	// because the lock will still be held, so we set a flag to check after
	// the lock is released
	onCleanup := false
	expiredKeys := []string{}
	if len(c.items) == c.capacity {
		onCleanup = true
		for k, v := range c.items {
			if v.expired() {
				delete(c.items, k)
				expiredKeys = append(expiredKeys, k)
			}
		}
	}

	// the onSet callback cannot be executed until we releae the lock
	// so we set a flag to check after the lock is released
	currSize := len(c.items)
	onSet := false
	if currSize < c.capacity {
		c.items[k] = newItem(v, c.ttl)
		onSet = true
		currSize = len(c.items)
	}
	c.mu.Unlock()

	// make sure no lock is held for user defined callbacks
	if onSet && c.onSet != nil {
		c.onSet(c.capacity, currSize, k)
	}

	if onCleanup && c.onCleanup != nil {
		c.onCleanup(c.capacity, currSize, expiredKeys)
	}
}

// Delete removes an entry from the cache
//
// DANGER: this method involves manual synchonization using a sync.RWMutex
// Be very careful when modifying this method to avoid synchronization issues
// Better yet, do not modify it (except for bug fixes)
func (c *Cache) Delete(k string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, k)
}

// Size return the number of unexpired entries in the cache
//
// DANGER: this method involves manual synchonization using a sync.RWMutex
// Be very careful when modifying this method to avoid synchronization issues
// Better yet, do not modify it (except for bug fixes)
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, v := range c.items {
		if !v.expired() {
			count++
		}
	}

	return count
}

func (c *Cache) OnCacheMiss(f func(string)) {
	c.onCacheMiss = f
}

func (c *Cache) OnCacheHit(f func(string, interface{})) {
	c.onCacheHit = f
}

func (c *Cache) OnSet(f func(int, int, string)) {
	c.onSet = f
}

func (c *Cache) OnCleanup(f func(int, int, []string)) {
	c.onCleanup = f
}
