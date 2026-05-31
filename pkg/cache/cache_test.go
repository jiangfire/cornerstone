package cache

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getRedisURL() string {
	return os.Getenv("REDIS_URL")
}

func TestMemoryCache_SatisfiesInterface(t *testing.T) {
	var c Cache[string, int] = NewMemoryCache[string, int](time.Hour)
	require.NotNil(t, c)
}

func TestMemoryCache_SetAndGet(t *testing.T) {
	c := NewMemoryCache[string, int](100 * time.Millisecond)

	c.Set("a", 1)
	val, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, val)
}

func TestMemoryCache_GetMissing(t *testing.T) {
	c := NewMemoryCache[string, int](100 * time.Millisecond)

	_, ok := c.Get("missing")
	assert.False(t, ok)
}

func TestMemoryCache_Expiration(t *testing.T) {
	c := NewMemoryCache[string, int](50 * time.Millisecond)

	c.Set("a", 1)
	_, ok := c.Get("a")
	require.True(t, ok)

	time.Sleep(60 * time.Millisecond)
	_, ok = c.Get("a")
	assert.False(t, ok)
}

func TestMemoryCache_ExpiredEntryCleanedUp(t *testing.T) {
	c := NewMemoryCache[string, int](20 * time.Millisecond)

	c.Set("a", 1)
	time.Sleep(30 * time.Millisecond)

	// Get should return false and clean up the entry
	_, ok := c.Get("a")
	assert.False(t, ok)

	// Internal map should no longer contain the key
	assert.Equal(t, 0, len(c.data))
}

func TestMemoryCache_ExpiredGetDoesNotDeleteRefreshedEntry(t *testing.T) {
	c := NewMemoryCache[string, int](20 * time.Millisecond)

	c.Set("a", 1)
	time.Sleep(30 * time.Millisecond)

	// Simulate race: another goroutine refreshes the key between RUnlock and Lock
	c.Set("a", 2)

	// Get should detect the entry is now fresh and return the refreshed value
	val, ok := c.Get("a")
	require.True(t, ok, "entry was refreshed, should be found")
	assert.Equal(t, 2, val)

	// The refreshed entry should still exist
	assert.Equal(t, 1, len(c.data))
}

func TestMemoryCache_Delete(t *testing.T) {
	c := NewMemoryCache[string, int](time.Hour)

	c.Set("a", 1)
	c.Delete("a")
	_, ok := c.Get("a")
	assert.False(t, ok)
}

func TestMemoryCache_Clear(t *testing.T) {
	c := NewMemoryCache[string, int](time.Hour)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()

	_, ok := c.Get("a")
	assert.False(t, ok)
	_, ok = c.Get("b")
	assert.False(t, ok)
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	c := NewMemoryCache[int, int](time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			c.Set(v, v*2)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		val, ok := c.Get(i)
		require.True(t, ok, "key %d should exist", i)
		assert.Equal(t, i*2, val, "key %d should have value %d", i, i*2)
	}
}

func TestGetOrSet(t *testing.T) {
	c := NewMemoryCache[string, int](time.Hour)

	called := 0
	val := GetOrSet(c, "a", func() int {
		called++
		return 42
	})
	assert.Equal(t, 42, val)
	assert.Equal(t, 1, called)

	val = GetOrSet(c, "a", func() int {
		called++
		return 99
	})
	assert.Equal(t, 42, val)
	assert.Equal(t, 1, called)
}

func TestGetOrSet_ExpiredReturnsNew(t *testing.T) {
	c := NewMemoryCache[string, int](20 * time.Millisecond)

	val := GetOrSet(c, "a", func() int { return 1 })
	assert.Equal(t, 1, val)

	time.Sleep(30 * time.Millisecond)

	val = GetOrSet(c, "a", func() int { return 2 })
	assert.Equal(t, 2, val)
}

func TestMemoryCache_SetOverwrite(t *testing.T) {
	c := NewMemoryCache[string, int](time.Hour)

	c.Set("a", 1)
	c.Set("a", 2)
	val, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 2, val)
}

func TestMemoryCache_DeleteMissing(t *testing.T) {
	c := NewMemoryCache[string, int](time.Hour)

	c.Delete("nonexistent")
	// Should not panic
}

func TestMemoryCache_ClearEmpty(t *testing.T) {
	c := NewMemoryCache[string, int](time.Hour)

	c.Clear()
	// Should not panic
}

func TestRedisCache_Integration(t *testing.T) {
	redisURL := getRedisURL()
	if redisURL == "" {
		t.Skip("REDIS_URL not set, skipping Redis integration test")
	}

	c := NewRedisCache[int](redisURL, "test", time.Hour)
	c.Clear()

	t.Run("SetAndGet", func(t *testing.T) {
		c.Set("key1", 42)
		val, ok := c.Get("key1")
		require.True(t, ok)
		assert.Equal(t, 42, val)
	})

	t.Run("GetMissing", func(t *testing.T) {
		_, ok := c.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("Delete", func(t *testing.T) {
		c.Set("key2", 99)
		c.Delete("key2")
		_, ok := c.Get("key2")
		assert.False(t, ok)
	})

	t.Run("SetOverwrite", func(t *testing.T) {
		c.Set("key3", 1)
		c.Set("key3", 2)
		val, ok := c.Get("key3")
		require.True(t, ok)
		assert.Equal(t, 2, val)
	})

	t.Run("StructValue", func(t *testing.T) {
		type testStruct struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		cs := NewRedisCache[testStruct](redisURL, "test_struct", time.Hour)
		defer cs.Clear()

		cs.Set("obj1", testStruct{Name: "hello", Value: 10})
		val, ok := cs.Get("obj1")
		require.True(t, ok)
		assert.Equal(t, "hello", val.Name)
		assert.Equal(t, 10, val.Value)
	})

	t.Run("Clear", func(t *testing.T) {
		c.Set("key4", 1)
		c.Clear()
		_, ok := c.Get("key4")
		assert.False(t, ok)
	})

	c.Clear()
}

func TestRedisCache_NonIntrusive(t *testing.T) {
	c := NewRedisCache[int]("redis://localhost:9999", "test", time.Hour)
	c.Set("a", 1)
	_, ok := c.Get("a")
	assert.False(t, ok, "Get on unreachable Redis should return miss gracefully")
	c.Delete("a")
	c.Clear()
	// Should not panic
}
