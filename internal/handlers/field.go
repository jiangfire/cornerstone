package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

// CreateField
//
// @Summary      Create a field
// @Description  Create a new field in a table.
//
//	Valid field types: string, text, number, boolean, date, datetime, attachment,
//	select, list, multiselect, single_select, multi_select.
//
//	For select/multi_select types, use the options field (comma-separated values)
//	or the config.options array. For number fields, config.min and config.max
//	define the allowed range. For attachment fields, config defines allowed file
//	types and size limits.
//
//	The authenticated token must own the parent database.
//
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.FieldCreateRequest  true  "Field to create"
// @Success      200  {object}  swagger.APIResponse{data=swagger.FieldObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body or field type"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to parent table"
// @Router       /api/fields [post]
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
//
//	The authenticated token must own the parent database or be a Master token.
//	Each field includes its type, configuration, and whether it is required.
//
// @Tags         fields
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Table ID"
// @Success      200  {object}  swagger.APIResponse{data=swagger.FieldListResponse}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this table"
// @Router       /api/tables/{id}/fields [get]
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
// @Summary      Get a field by ID
// @Description  Retrieve full details of a single field by its ID.
//
//	Returns the field type, configuration, required flag, and other metadata.
//	The authenticated token must own the parent database or be a Master token.
//
// @Tags         fields
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Field ID"
// @Success      200  {object}  swagger.APIResponse{data=swagger.FieldObject}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this field"
// @Failure      404  {object}  swagger.ErrorResponse  "Field not found"
// @Router       /api/fields/{id} [get]
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
// @Description  Update field properties including name, type, description, required flag, and config.
//
//	Valid field types: string, text, number, boolean, date, datetime, attachment,
//	select, list, multiselect, single_select, multi_select.
//
//	Changing a field type may affect existing record data. Use with caution.
//	The authenticated token must own the parent database or be a Master token.
//
// @Tags         fields
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string                true  "Field ID"
// @Param        body  body  swagger.FieldUpdateRequest  true  "Field update fields"
// @Success      200  {object}  swagger.APIResponse{data=swagger.FieldObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body or field type"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this field"
// @Failure      404  {object}  swagger.ErrorResponse  "Field not found"
// @Router       /api/fields/{id} [put]
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
//
//	This action is irreversible and will remove the field from all records.
//	The authenticated token must own the parent database or be a Master token.
//
// @Tags         fields
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Field ID"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this field"
// @Failure      404  {object}  swagger.ErrorResponse  "Field not found"
// @Router       /api/fields/{id} [delete]
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
