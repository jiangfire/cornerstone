package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value  V
	expiry time.Time
}

// MemoryCache is a thread-safe in-memory cache with TTL implementing the Cache[K, V] interface.
type MemoryCache[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]entry[V]
	ttl  time.Duration
}

// NewMemoryCache creates an in-memory cache instance.
func NewMemoryCache[K comparable, V any](ttl time.Duration) *MemoryCache[K, V] {
	return &MemoryCache[K, V]{
		data: make(map[K]entry[V]),
		ttl:  ttl,
	}
}

// Get retrieves a cached value. Returns (zero, false) if key does not exist or has expired. Expired entries are cleaned up.
func (c *MemoryCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if time.Now().After(e.expiry) {
		c.mu.Lock()
		if current, ok := c.data[key]; ok {
			if time.Now().Before(current.expiry) {
				val := current.value
				c.mu.Unlock()
				return val, true
			}
			delete(c.data, key)
		}
		c.mu.Unlock()
		var zero V
		return zero, false
	}

	return e.value, true
}

// Set writes a cached value.
func (c *MemoryCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = entry[V]{value: value, expiry: time.Now().Add(c.ttl)}
}

// Delete removes the specified key.
func (c *MemoryCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear clears all cache entries.
func (c *MemoryCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[K]entry[V])
}
