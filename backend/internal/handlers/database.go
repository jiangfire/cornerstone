package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// CreateDatabase
//
// @Summary      Create a database
// @Description  Create a new database owned by the authenticated token.
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  object  true  "Database to create"  example({"name":"My DB","description":"A database"})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","name":"...","description":"...","created_at":"..."}}"
// @Failure      400  {object}  map[string]any
// @Router       /databases [post]
func CreateDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	var req services.CreateDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.CreateDatabase(req, tokenID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"id":          database.ID,
		"name":        database.Name,
		"description": database.Description,
		"created_at":  database.CreatedAt,
	})
}

// ListDatabases
//
// @Summary      List databases
// @Description  Returns all databases accessible to the authenticated token.
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"databases":[...],"total":0}}"
// @Failure      500  {object}  map[string]any
// @Router       /databases [get]
func ListDatabases(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	dbService := services.NewDatabaseService(db.DB())
	databases, err := dbService.ListDatabases(tokenID)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"databases": databases,
		"total":     len(databases),
	})
}

// GetDatabase
//
// @Summary      Get a database
// @Description  Get database details by ID.
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Database ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /databases/{id} [get]
func GetDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.GetDatabase(dbID, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, database)
}

// UpdateDatabase
//
// @Summary      Update a database
// @Description  Update database name and/or description.
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string  true  "Database ID"
// @Param        body  body  object  true  "Database update fields"  example({"name":"New Name","description":"Updated"})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","name":"...","description":"...","updated_at":"..."}}"
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /databases/{id} [put]
func UpdateDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	var req services.UpdateDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.UpdateDatabase(dbID, req, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"id":          database.ID,
		"name":        database.Name,
		"description": database.Description,
		"updated_at":  database.UpdatedAt,
	})
}

// DeleteDatabase
//
// @Summary      Delete a database
// @Description  Delete a database by ID.
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Database ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"message":"数据库已删除"}}"
// @Failure      403  {object}  map[string]any
// @Router       /databases/{id} [delete]
func DeleteDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	if err := dbService.DeleteDatabase(dbID, tokenID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"message": "数据库已删除",
	})
}

// CreateDatabaseWithTables
//
// @Summary      Create database with tables and fields
// @Description  Atomically create a database together with nested tables and fields in a single request.
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  object  true  "Database with nested tables/fields"  example({"name":"DB","description":"","tables":[{"name":"T1","fields":[{"name":"title","type":"string"}]}]})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"database":{...},"tables":[...],"fields":[...],"summary":{"table_count":1,"field_count":1}}}"
// @Failure      400  {object}  map[string]any
// @Router       /databases/with-tables [post]
func CreateDatabaseWithTables(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	var req services.CreateDBWithTablesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	result, err := dbService.CreateDatabaseWithTables(req, tokenID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"database": result.Database,
		"tables":   result.Tables,
		"fields":   result.Fields,
		"summary": gin.H{
			"table_count": len(result.Tables),
			"field_count": len(result.Fields),
		},
	})
}
