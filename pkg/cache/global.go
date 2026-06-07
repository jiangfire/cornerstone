package cache

// Clearable is a cache interface that can be cleared.
type Clearable interface {
	Clear()
}

var globalCaches []Clearable

// Register registers a cache instance to the global cleanup list.
// Should be called in package-level init() to ensure cleanup during tests.
func Register(c Clearable) {
	globalCaches = append(globalCaches, c)
}

// ClearAll clears all registered global cache instances.
func ClearAll() {
	for _, c := range globalCaches {
		c.Clear()
	}
}
