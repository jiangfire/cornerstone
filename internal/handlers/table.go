package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

// CreateTable creates a table
//
// @Summary      Create a table
// @Description  Create a new table inside a database.
//
//	The database_id field is required and must reference an existing database
//	owned by the authenticated token. The table name must be non-empty.
//
// @Tags         tables
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.TableCreateRequest  true  "Table to create"
// @Success      200  {object}  swagger.APIResponse{data=swagger.TableObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to parent database"
// @Router       /api/v1/tables [post]
func CreateTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req services.CreateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	tableService := services.NewTableService(db.DB())
	table, err := tableService.CreateTable(req, userID)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	dto.Success(c, tableObjectFromModel(table))
}

// ListTables lists tables
//
// @Summary      List tables in a database
// @Description  Returns all tables in the specified database.
//
//	The authenticated token must have access to the parent database.
//	Results include table ID, name, description, and timestamps.
//
// @Tags         tables
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Database ID"
// @Success      200  {object}  swagger.APIResponse{data=swagger.TableListResponse}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this database"
// @Router       /api/v1/databases/{id}/tables [get]
func ListTables(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	tables, err := tableService.ListTables(dbID, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	items := make([]dto.TableObject, len(tables))
	for i := range tables {
		items[i] = tableObjectFromResponse(&tables[i])
	}
	dto.Success(c, dto.TableListData{Tables: items, Total: len(items)})
}

// GetTable gets table details
//
// @Summary      Get a table by ID
// @Description  Retrieve full details of a single table by its ID.
//
//	The authenticated token must own the parent database or be a Master token.
//
// @Tags         tables
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Table ID"
// @Success      200  {object}  swagger.APIResponse{data=swagger.TableObject}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this table"
// @Failure      404  {object}  swagger.ErrorResponse  "Table not found"
// @Router       /api/v1/tables/{id} [get]
func GetTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	table, err := tableService.GetTable(tableID, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, tableObjectFromResponse(table))
}

// UpdateTable updates a table
//
// @Summary      Update a table
// @Description  Update table name and/or description.
//
//	The authenticated token must own the parent database or be a Master token.
//	The name field is required in the request body.
//
// @Tags         tables
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string                true  "Table ID"
// @Param        body  body  swagger.TableUpdateRequest  true  "Table update fields"
// @Success      200  {object}  swagger.APIResponse{data=swagger.TableObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this table"
// @Failure      404  {object}  swagger.ErrorResponse  "Table not found"
// @Router       /api/v1/tables/{id} [put]
func UpdateTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	var req services.UpdateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	tableService := services.NewTableService(db.DB())
	table, err := tableService.UpdateTable(tableID, req, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, tableObjectFromModel(table))
}

// DeleteTable deletes a table
//
// @Summary      Delete a table
// @Description  Delete a table and all of its associated fields and records.
//
//	This action is irreversible. The authenticated token must own the parent
//	database or be a Master token.
//
// @Tags         tables
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Table ID"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this table"
// @Failure      404  {object}  swagger.ErrorResponse  "Table not found"
// @Router       /api/v1/tables/{id} [delete]
func DeleteTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	if err := tableService.DeleteTable(tableID, userID); err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, dto.MessageData{Message: "table deleted"})
}
