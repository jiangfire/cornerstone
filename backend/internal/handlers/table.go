package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// CreateTable 创建表
//
// @Summary      Create a table
// @Description  Create a new table inside a database.
// @Tags         tables
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  object  true  "Table to create"  example({"database_id":"db-1","name":"Users","description":"User table"})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","database_id":"...","name":"...","description":"...","created_at":"..."}}"
// @Failure      400  {object}  map[string]any
// @Router       /tables [post]
func CreateTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req services.CreateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	tableService := services.NewTableService(db.DB())
	table, err := tableService.CreateTable(req, userID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"id":          table.ID,
		"database_id": table.DatabaseID,
		"name":        table.Name,
		"description": table.Description,
		"created_at":  table.CreatedAt,
	})
}

// ListTables 获取表列表
//
// @Summary      List tables in a database
// @Description  Returns all tables in the specified database.
// @Tags         tables
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Database ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"tables":[...],"total":0}}"
// @Failure      403  {object}  map[string]any
// @Router       /databases/{id}/tables [get]
func ListTables(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	tables, err := tableService.ListTables(dbID, userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"tables": tables,
		"total":  len(tables),
	})
}

// GetTable 获取表详情
//
// @Summary      Get a table
// @Description  Get table details by ID.
// @Tags         tables
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Table ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /tables/{id} [get]
func GetTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	table, err := tableService.GetTable(tableID, userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, table)
}

// UpdateTable 更新表信息
//
// @Summary      Update a table
// @Description  Update table name and/or description.
// @Tags         tables
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string  true  "Table ID"
// @Param        body  body  object  true  "Table update fields"  example({"name":"New Name","description":"Updated"})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","name":"...","description":"...","updated_at":"..."}}"
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /tables/{id} [put]
func UpdateTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	var req services.UpdateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	tableService := services.NewTableService(db.DB())
	table, err := tableService.UpdateTable(tableID, req, userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"id":          table.ID,
		"name":        table.Name,
		"description": table.Description,
		"updated_at":  table.UpdatedAt,
	})
}

// DeleteTable 删除表
//
// @Summary      Delete a table
// @Description  Delete a table by ID.
// @Tags         tables
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Table ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"message":"表已删除"}}"
// @Failure      403  {object}  map[string]any
// @Router       /tables/{id} [delete]
func DeleteTable(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	if err := tableService.DeleteTable(tableID, userID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"message": "表已删除",
	})
}
