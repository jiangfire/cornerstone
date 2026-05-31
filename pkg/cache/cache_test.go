package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache[string, int](100 * time.Millisecond)

	c.Set("a", 1)
	val, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, val)
}

func TestCache_GetMissing(t *testing.T) {
	c := NewCache[string, int](100 * time.Millisecond)

	_, ok := c.Get("missing")
	assert.False(t, ok)
}

func TestCache_Expiration(t *testing.T) {
	c := NewCache[string, int](50 * time.Millisecond)

	c.Set("a", 1)
	_, ok := c.Get("a")
	require.True(t, ok)

	time.Sleep(60 * time.Millisecond)
	_, ok = c.Get("a")
	assert.False(t, ok)
}

func TestCache_Delete(t *testing.T) {
	c := NewCache[string, int](time.Hour)

	c.Set("a", 1)
	c.Delete("a")
	_, ok := c.Get("a")
	assert.False(t, ok)
}

func TestCache_Clear(t *testing.T) {
	c := NewCache[string, int](time.Hour)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()

	_, ok := c.Get("a")
	assert.False(t, ok)
	_, ok = c.Get("b")
	assert.False(t, ok)
}

func TestCache_GetOrSet(t *testing.T) {
	c := NewCache[string, int](time.Hour)

	called := 0
	val := c.GetOrSet("a", func() int {
		called++
		return 42
	})
	assert.Equal(t, 42, val)
	assert.Equal(t, 1, called)

	// 第二次直接从缓存取
	val = c.GetOrSet("a", func() int {
		called++
		return 99
	})
	assert.Equal(t, 42, val)
	assert.Equal(t, 1, called)
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache[int, int](time.Hour)

	// 并发读写不应 panic
	for i := 0; i < 100; i++ {
		go func(v int) {
			c.Set(v, v*2)
			c.Get(v)
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
}
