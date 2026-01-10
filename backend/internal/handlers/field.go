package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// CreateField 创建字段
func CreateField(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	fieldService := services.NewFieldService(db.DB())
	field, err := fieldService.CreateField(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":         field.ID,
		"table_id":   field.TableID,
		"name":       field.Name,
		"type":       field.Type,
		"required":   field.Required,
		"created_at": field.CreatedAt,
	})
}

// ListFields 获取字段列表
func ListFields(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tableID := c.Param("id")

	fieldService := services.NewFieldService(db.DB())
	fields, err := fieldService.ListFields(tableID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"fields": fields,
		"total":  len(fields),
	})
}

// GetField 获取字段详情
func GetField(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fieldID := c.Param("id")

	fieldService := services.NewFieldService(db.DB())
	field, err := fieldService.GetField(fieldID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, field)
}

// UpdateField 更新字段信息
func UpdateField(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fieldID := c.Param("id")

	var req services.UpdateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	fieldService := services.NewFieldService(db.DB())
	field, err := fieldService.UpdateField(fieldID, req, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":         field.ID,
		"name":       field.Name,
		"type":       field.Type,
		"required":   field.Required,
		"updated_at": field.UpdatedAt,
	})
}

// DeleteField 删除字段
func DeleteField(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fieldID := c.Param("id")

	fieldService := services.NewFieldService(db.DB())
	if err := fieldService.DeleteField(fieldID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "字段已删除",
	})
}
