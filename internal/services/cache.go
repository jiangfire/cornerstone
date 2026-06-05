package services

import (
	"time"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/cache"
)

// SharedFieldCache 跨请求共享的表字段定义缓存。
var SharedFieldCache = cache.NewString[[]models.Field]("field", 5*time.Minute)

func init() {
	cache.Register(SharedFieldCache)
}

// InvalidateFieldCache 失效指定表的字段缓存。
func InvalidateFieldCache(tableID string) {
	SharedFieldCache.Delete(tableID)
}
