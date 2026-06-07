package cache

// Cache is a generic cache interface supporting any comparable key type and any value type.
type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V)
	Delete(key K)
	Clear()
}

// GetOrSet retrieves a value from cache; if absent, calls factory to generate and store it.
func GetOrSet[K comparable, V any](c Cache[K, V], key K, factory func() V) V {
	if v, ok := c.Get(key); ok {
		return v
	}
	v := factory()
	c.Set(key, v)
	return v
}
