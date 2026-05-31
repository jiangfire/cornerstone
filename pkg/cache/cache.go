package cache

// Cache 是泛型缓存接口，支持任意可比较的 key 类型和任意 value 类型。
type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V)
	Delete(key K)
	Clear()
}

// GetOrSet 从缓存获取值，不存在时调用 factory 生成并写入。
func GetOrSet[K comparable, V any](c Cache[K, V], key K, factory func() V) V {
	if v, ok := c.Get(key); ok {
		return v
	}
	v := factory()
	c.Set(key, v)
	return v
}
