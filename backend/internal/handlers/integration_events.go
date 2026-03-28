package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// ReceiveIntegrationEvent 接收入站集成事件
func ReceiveIntegrationEvent(c *gin.Context) {
	sourceSystem := middleware.GetIntegrationSource(c)

	var req services.ReceiveIntegrationEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	eventService := services.NewIntegrationEventService(db.DB())
	result, err := eventService.ReceiveEvent(sourceSystem, req)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, result)
}
