package cache

import (
	"os"
	"time"
)

// New 创建内存缓存实例，支持任意 comparable key 类型。
func New[K comparable, V any](ttl time.Duration) Cache[K, V] {
	return NewMemoryCache[K, V](ttl)
}

// NewString 创建以 string 为 key 的缓存实例。
// 根据 REDIS_URL 环境变量选择后端：
//   - 空值：使用 MemoryCache（默认）
//   - 有值：使用 RedisCache
func NewString[V any](prefix string, ttl time.Duration) Cache[string, V] {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return NewMemoryCache[string, V](ttl)
	}
	return NewRedisCache[V](redisURL, prefix, ttl)
}
