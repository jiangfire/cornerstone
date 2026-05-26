package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

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
