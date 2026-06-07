package cache

import (
	"os"
	"time"
)

// New creates an in-memory cache instance supporting any comparable key type.
func New[K comparable, V any](ttl time.Duration) Cache[K, V] {
	return NewMemoryCache[K, V](ttl)
}

// NewString creates a cache instance with string keys.
// Selects backend based on the REDIS_URL environment variable:
//   - Empty: use MemoryCache (default)
//   - Set: use RedisCache
func NewString[V any](prefix string, ttl time.Duration) Cache[string, V] {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return NewMemoryCache[string, V](ttl)
	}
	return NewRedisCache[V](redisURL, prefix, ttl)
}
