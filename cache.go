package main

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// Cache provides a simple interface for key-value caching with TTL
// Values are opaque and should be JSON-serializable if used across boundaries.
// Implementations should be concurrency-safe.

type Cache interface {
	Get(ctx context.Context, key string) (value interface{}, ok bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) bool
	Delete(ctx context.Context, key string)
	PurgeExpired()
	Stats() CacheStats
}

type CacheStats struct {
	Items        int
	Capacity     int
	Hits         uint64
	Misses       uint64
	Evictions    uint64
	ExpiredCount uint64
}

type cacheItem struct {
	key      string
	value    interface{}
	expireAt time.Time
}

// InMemoryCache is a simple LRU cache with TTL
// It is designed for low-latency, in-process caching.

type InMemoryCache struct {
	mu         sync.RWMutex
	items      map[string]*list.Element
	ll         *list.List
	capacity   int
	defaultTTL time.Duration

	stats struct {
		Hits         uint64
		Misses       uint64
		Evictions    uint64
		ExpiredCount uint64
	}
}

func NewInMemoryCache(capacity int, defaultTTL time.Duration) *InMemoryCache {
	if capacity <= 0 {
		capacity = 1024
	}
	return &InMemoryCache{
		items:      make(map[string]*list.Element, capacity),
		ll:         list.New(),
		capacity:   capacity,
		defaultTTL: defaultTTL,
	}
}

func (c *InMemoryCache) Get(_ context.Context, key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		c.stats.Misses++
		return nil, false
	}
	ci := e.Value.(cacheItem)
	if !ci.expireAt.IsZero() && time.Now().After(ci.expireAt) {
		// expired
		c.ll.Remove(e)
		delete(c.items, key)
		c.stats.ExpiredCount++
		c.stats.Misses++
		return nil, false
	}
	// move to front (MRU)
	c.ll.MoveToFront(e)
	c.stats.Hits++
	return ci.value, true
}

func (c *InMemoryCache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// update existing
	if e, ok := c.items[key]; ok {
		ci := e.Value.(cacheItem)
		ci.value = value
		ci.expireAt = c.effectiveExpire(ttl)
		e.Value = ci
		c.ll.MoveToFront(e)
		return true
	}

	// evict if full
	if c.ll.Len() >= c.capacity {
		if tail := c.ll.Back(); tail != nil {
			ci := tail.Value.(cacheItem)
			c.ll.Remove(tail)
			delete(c.items, ci.key)
			c.stats.Evictions++
		}
	}

	e := c.ll.PushFront(cacheItem{key: key, value: value, expireAt: c.effectiveExpire(ttl)})
	c.items[key] = e
	return true
}

func (c *InMemoryCache) Delete(_ context.Context, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		c.ll.Remove(e)
		delete(c.items, key)
	}
}

func (c *InMemoryCache) PurgeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ll.Len() == 0 {
		return
	}
	now := time.Now()
	for e := c.ll.Back(); e != nil; {
		prev := e.Prev()
		ci := e.Value.(cacheItem)
		if !ci.expireAt.IsZero() && now.After(ci.expireAt) {
			c.ll.Remove(e)
			delete(c.items, ci.key)
			c.stats.ExpiredCount++
		}
		e = prev
	}
}

func (c *InMemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Items:        c.ll.Len(),
		Capacity:     c.capacity,
		Hits:         c.stats.Hits,
		Misses:       c.stats.Misses,
		Evictions:    c.stats.Evictions,
		ExpiredCount: c.stats.ExpiredCount,
	}
}

func (c *InMemoryCache) effectiveExpire(ttl time.Duration) time.Time {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}
