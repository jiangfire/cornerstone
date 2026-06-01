package cache

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	c := New[string, int](5 * time.Minute)
	require.NotNil(t, c)
	_, ok := c.(*MemoryCache[string, int])
	assert.True(t, ok)
}

func TestNewString_MemoryBackend(t *testing.T) {
	os.Unsetenv("REDIS_URL")
	c := NewString[int]("test", 5*time.Minute)
	require.NotNil(t, c)
	_, ok := c.(*MemoryCache[string, int])
	assert.True(t, ok)
}

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

func TestRedisCache_Set_NilClient(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "test", ttl: time.Minute}
	assert.NotPanics(t, func() {
		rc.Set("key", 42)
	})
}

func TestRedisCache_Delete_NilClient(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "test", ttl: time.Minute}
	assert.NotPanics(t, func() {
		rc.Delete("key")
	})
}

func TestRedisCache_Clear_NilClient(t *testing.T) {
	rc := &RedisCache[int]{client: nil, prefix: "test", ttl: time.Minute}
	assert.NotPanics(t, func() {
		rc.Clear()
	})
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
