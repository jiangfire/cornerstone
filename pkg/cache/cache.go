package cache

import (
	"sync"
	"time"
)

// Cache 是一个带 TTL 的线程安全内存缓存。
type Cache[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]entry[V]
	ttl  time.Duration
}

type entry[V any] struct {
	value  V
	expiry time.Time
}

// NewCache 创建一个新的 TTL 缓存。
func NewCache[K comparable, V any](ttl time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		data: make(map[K]entry[V]),
		ttl:  ttl,
	}
}

// Get 从缓存中获取值。如果 key 不存在或已过期，返回 (zero, false)。
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(e.expiry) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set 写入缓存，使用默认 TTL。
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = entry[V]{value: value, expiry: time.Now().Add(c.ttl)}
}

// Delete 删除指定 key。
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear 清空所有缓存。
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[K]entry[V])
}

// GetOrSet 先尝试 Get，不存在则调用 factory 生成值并 Set。
func (c *Cache[K, V]) GetOrSet(key K, factory func() V) V {
	if v, ok := c.Get(key); ok {
		return v
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// double-check
	if e, ok := c.data[key]; ok && time.Now().Before(e.expiry) {
		return e.value
	}

	v := factory()
	c.data[key] = entry[V]{value: v, expiry: time.Now().Add(c.ttl)}
	return v
}
