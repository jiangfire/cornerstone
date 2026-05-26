package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

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

func GetFieldPermissions(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	fieldService := services.NewFieldService(db.DB())
	permissions, err := fieldService.GetFieldPermissions(tableID, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"permissions": permissions,
		"total":       len(permissions),
	})
}

func SetFieldPermission(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	var req services.FieldPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	fieldService := services.NewFieldService(db.DB())
	if err := fieldService.SetFieldPermission(tableID, req, tokenID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"message": "权限设置成功",
	})
}

func BatchSetFieldPermissions(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	var req services.BatchFieldPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	fieldService := services.NewFieldService(db.DB())
	if err := fieldService.BatchSetFieldPermissions(tableID, req, tokenID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"message": "批量权限设置成功",
		"count":   len(req.Permissions),
	})
}
