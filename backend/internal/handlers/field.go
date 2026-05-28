package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// CreateField
//
// @Summary      Create a field
// @Description  Create a new field in a table. Valid types: string, text, number, boolean, date, datetime, attachment, select, list, multiselect, single_select, multi_select
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  object  true  "Field to create"  example({"table_id":"tbl-1","name":"Title","type":"string","description":"Title field","required":true})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","table_id":"...","name":"...","type":"...","description":"...","required":false,"created_at":"..."}}"
// @Failure      400  {object}  map[string]any
// @Router       /fields [post]
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
//
// @Summary      List fields in a table
// @Description  Returns all fields in the specified table.
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Table ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"items":[...],"total":0}}"
// @Failure      403  {object}  map[string]any
// @Router       /tables/{id}/fields [get]
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
//
// @Summary      Get a field
// @Description  Get field details by ID.
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Field ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /fields/{id} [get]
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
//
// @Summary      Update a field
// @Description  Update field properties. Valid types: string, text, number, boolean, date, datetime, attachment, select, list, multiselect, single_select, multi_select
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string  true  "Field ID"
// @Param        body  body  object  true  "Field update fields"  example({"name":"New Name","type":"text","description":"Updated","required":false})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","name":"...","type":"...","description":"...","required":false,"updated_at":"..."}}"
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /fields/{id} [put]
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
//
// @Summary      Delete a field
// @Description  Delete a field by ID.
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Field ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"message":"字段已删除"}}"
// @Failure      403  {object}  map[string]any
// @Router       /fields/{id} [delete]
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

// GetFieldPermissions
//
// @Summary      Get field permissions
// @Description  Get field-level permission settings for a table.
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Table ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"permissions":[...],"total":0}}"
// @Failure      403  {object}  map[string]any
// @Router       /tables/{id}/fields/permissions [get]
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

// SetFieldPermission
//
// @Summary      Set field permission
// @Description  Set permission for a specific field in a table.
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string  true  "Table ID"
// @Param        body  body  object  true  "Permission to set"  example({"field_id":"fld-1","role":"editor","can_read":true,"can_write":true,"can_delete":false})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"message":"权限设置成功"}}"
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /tables/{id}/fields/permissions [post]
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

// BatchSetFieldPermissions
//
// @Summary      Batch set field permissions
// @Description  Set permissions for multiple fields in a table at once.
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string  true  "Table ID"
// @Param        body  body  object  true  "Permissions to set"  example({"permissions":[{"field_id":"fld-1","role":"editor","can_read":true,"can_write":true}]})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"message":"批量权限设置成功","count":1}}"
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /tables/{id}/fields/permissions/batch [put]
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
