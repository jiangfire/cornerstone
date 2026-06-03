package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewString_RedisBackend_InvalidURL(t *testing.T) {
	t.Setenv("REDIS_URL", "://invalid")
	c := NewString[int]("test", 5*time.Minute)
	require.NotNil(t, c)
	rc, ok := c.(*RedisCache[int])
	assert.True(t, ok)
	assert.Nil(t, rc.client)
}

func TestRedisCache_Get_NilClient(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "test", ttl: time.Minute}
	v, ok := rc.Get("key")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestRedisCache_Set_NilClient_NoOp(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "test", ttl: time.Minute}
	rc.Set("key", 42)
	v, ok := rc.Get("key")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestRedisCache_Delete_NilClient_NoOp(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "test", ttl: time.Minute}
	rc.Delete("key")
	v, ok := rc.Get("key")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestRedisCache_Clear_NilClient_NoOp(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "test", ttl: time.Minute}
	rc.Set("a", 1)
	rc.Clear()
	v, ok := rc.Get("a")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestRedisCache_NewRedisCache_InvalidURL(t *testing.T) {
	rc := NewRedisCache[string]("://invalid", "test", time.Minute)
	require.NotNil(t, rc)
	assert.Nil(t, rc.client)
}

func TestRedisCache_key(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "myprefix", ttl: time.Minute}
	assert.Equal(t, "cache:myprefix:mykey", rc.key("mykey"))
}
