package cache

// Clearable 是可以被清空的缓存接口。
type Clearable interface {
	Clear()
}

var globalCaches []Clearable

// Register 将缓存实例注册到全局清理列表。
// 应在包级 init() 中调用，确保测试 cleanup 时能一并清空。
func Register(c Clearable) {
	globalCaches = append(globalCaches, c)
}

// ClearAll 清空所有已注册的全局缓存实例。
func ClearAll() {
	for _, c := range globalCaches {
		c.Clear()
	}
}
