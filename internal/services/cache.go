package services

import (
	"time"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/cache"
)

// SharedFieldCache is a cross-request shared table field definition cache.
var SharedFieldCache = cache.NewString[[]models.Field]("field", 5*time.Minute)

func init() {
	cache.Register(SharedFieldCache)
}

// InvalidateFieldCache invalidates the field cache for the specified table.
func InvalidateFieldCache(tableID string) {
	SharedFieldCache.Delete(tableID)
}
