package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// GetSettings 获取系统设置
func GetSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	authService := services.NewAuthService(db.DB())
	isAdmin, err := authService.IsSystemAdmin(userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}
	if !isAdmin {
		dto.Error(c, 403, "只有系统管理员可以查看系统设置")
		return
	}

	settingsService := services.NewSettingsService(db.DB())
	settings, err := settingsService.GetSettings()
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, settings)
}

// UpdateSettings 更新系统设置
func UpdateSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	authService := services.NewAuthService(db.DB())
	isAdmin, err := authService.IsSystemAdmin(userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}
	if !isAdmin {
		dto.Error(c, 403, "只有系统管理员可以修改系统设置")
		return
	}

	var req services.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	settingsService := services.NewSettingsService(db.DB())
	settings, err := settingsService.UpdateSettings(req, userID)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, settings)
}
