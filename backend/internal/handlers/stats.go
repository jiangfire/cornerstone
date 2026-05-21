package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// GetStatsSummary 获取统计数据
func GetStatsSummary(c *gin.Context) {
	userID := middleware.GetUserID(c)

	statsService := services.NewStatsService(db.DB())
	stats, err := statsService.GetSummary(userID)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, stats)
}

// GetRecentActivities 获取最近活动
func GetRecentActivities(c *gin.Context) {
	userID := middleware.GetUserID(c)
	limit := c.DefaultQuery("limit", "10")

	statsService := services.NewStatsService(db.DB())
	activities, err := statsService.GetRecentActivities(userID, limit)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, activities)
}
