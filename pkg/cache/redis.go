package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is a Redis-backed cache implementing the Cache[string, V] interface.
// Values are stored via JSON serialization.
type RedisCache[V any] struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisCache creates a Redis cache instance.
// redisURL format: redis://user:pass@host:port/db
func NewRedisCache[V any](redisURL, prefix string, ttl time.Duration) *RedisCache[V] {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return &RedisCache[V]{
			client: nil,
			prefix: prefix,
			ttl:    ttl,
		}
	}
	return &RedisCache[V]{
		client: redis.NewClient(opts),
		prefix: prefix,
		ttl:    ttl,
	}
}

func (r *RedisCache[V]) key(key string) string {
	return fmt.Sprintf("cache:%s:%s", r.prefix, key)
}

// Get retrieves and deserializes a value from Redis. Returns (zero, false) if key does not exist or decode fails.
func (r *RedisCache[V]) Get(key string) (V, bool) {
	var zero V
	if r.client == nil {
		return zero, false
	}

	data, err := r.client.Get(context.Background(), r.key(key)).Bytes()
	if err != nil {
		return zero, false
	}

	var v V
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, false
	}
	return v, true
}

// Set serializes and writes to Redis.
func (r *RedisCache[V]) Set(key string, value V) {
	if r.client == nil {
		return
	}

	data, err := json.Marshal(value)
	if err != nil {
		return
	}

	r.client.Set(context.Background(), r.key(key), data, r.ttl)
}

// Delete removes the specified key from Redis.
func (r *RedisCache[V]) Delete(key string) {
	if r.client == nil {
		return
	}
	r.client.Del(context.Background(), r.key(key))
}

// Clear removes all cache entries under the current prefix.
func (r *RedisCache[V]) Clear() {
	if r.client == nil {
		return
	}

	pattern := fmt.Sprintf("cache:%s:*", r.prefix)
	iter := r.client.Scan(context.Background(), 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(context.Background()) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return
	}
	if len(keys) > 0 {
		r.client.Del(context.Background(), keys...)
	}
}
