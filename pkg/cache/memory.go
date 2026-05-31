package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value  V
	expiry time.Time
}

// MemoryCache 是带 TTL 的线程安全内存缓存，实现 Cache[K, V] 接口。
type MemoryCache[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]entry[V]
	ttl  time.Duration
}

// NewMemoryCache 创建内存缓存实例。
func NewMemoryCache[K comparable, V any](ttl time.Duration) *MemoryCache[K, V] {
	return &MemoryCache[K, V]{
		data: make(map[K]entry[V]),
		ttl:  ttl,
	}
}

// Get 获取缓存值。key 不存在或已过期返回 (zero, false)。过期条目会被清理。
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

// Set 写入缓存值。
func (c *MemoryCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = entry[V]{value: value, expiry: time.Now().Add(c.ttl)}
}

// Delete 删除指定 key。
func (c *MemoryCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear 清空全部缓存条目。
func (c *MemoryCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[K]entry[V])
}
