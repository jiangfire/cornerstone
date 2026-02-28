package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// GetSettings 获取系统设置
func GetSettings(c *gin.Context) {
	settingsService := services.NewSettingsService(db.DB())
	settings, err := settingsService.GetSettings()
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, settings)
}

// UpdateSettings 更新系统设置
func UpdateSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	settingsService := services.NewSettingsService(db.DB())
	settings, err := settingsService.UpdateSettings(req, userID)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, settings)
}
