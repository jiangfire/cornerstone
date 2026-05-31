package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// CreateField
func CreateField(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	var req services.CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	fieldService := services.NewFieldService(db.DB())
	field, err := fieldService.CreateField(req, tokenID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"id":          field.ID,
		"table_id":    field.TableID,
		"name":        field.Name,
		"type":        field.Type,
		"description": field.Description,
		"required":    field.Required,
		"created_at":  field.CreatedAt,
	})
}

// ListFields
func ListFields(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	fieldService := services.NewFieldService(db.DB())
	fields, err := fieldService.ListFields(tableID, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"items": fields,
		"total": len(fields),
	})
}

// GetField
func GetField(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	fieldID := c.Param("id")

	fieldService := services.NewFieldService(db.DB())
	field, err := fieldService.GetField(fieldID, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, field)
}

// UpdateField
func UpdateField(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	fieldID := c.Param("id")

	var req services.UpdateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	fieldService := services.NewFieldService(db.DB())
	field, err := fieldService.UpdateField(fieldID, req, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"id":          field.ID,
		"name":        field.Name,
		"type":        field.Type,
		"description": field.Description,
		"required":    field.Required,
		"updated_at":  field.UpdatedAt,
	})
}

// DeleteField
func DeleteField(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	fieldID := c.Param("id")

	fieldService := services.NewFieldService(db.DB())
	if err := fieldService.DeleteField(fieldID, tokenID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"message": "字段已删除",
	})
}
