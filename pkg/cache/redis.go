package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache 是 Redis 后端缓存，实现 Cache[string, V] 接口。
// 值通过 JSON 序列化存储。
type RedisCache[V any] struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisCache 创建 Redis 缓存实例。
// redisURL 格式: redis://user:pass@host:port/db
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

// Get 从 Redis 获取并反序列化值。key 不存在或解码失败返回 (zero, false)。
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

// Set 序列化并写入 Redis。
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

// Delete 从 Redis 删除指定 key。
func (r *RedisCache[V]) Delete(key string) {
	if r.client == nil {
		return
	}
	r.client.Del(context.Background(), r.key(key))
}

// Clear 删除当前前缀下的所有缓存条目。
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
