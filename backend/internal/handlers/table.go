package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// CreateTable 创建表
func CreateTable(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	tableService := services.NewTableService(db.DB())
	table, err := tableService.CreateTable(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":          table.ID,
		"database_id": table.DatabaseID,
		"name":        table.Name,
		"description": table.Description,
		"created_at":  table.CreatedAt,
	})
}

// ListTables 获取表列表
func ListTables(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	tables, err := tableService.ListTables(dbID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"tables": tables,
		"total":  len(tables),
	})
}

// GetTable 获取表详情
func GetTable(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tableID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	table, err := tableService.GetTable(tableID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, table)
}

// UpdateTable 更新表信息
func UpdateTable(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tableID := c.Param("id")

	var req services.UpdateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	tableService := services.NewTableService(db.DB())
	table, err := tableService.UpdateTable(tableID, req, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":          table.ID,
		"name":        table.Name,
		"description": table.Description,
		"updated_at":  table.UpdatedAt,
	})
}

// DeleteTable 删除表
func DeleteTable(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tableID := c.Param("id")

	tableService := services.NewTableService(db.DB())
	if err := tableService.DeleteTable(tableID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "表已删除",
	})
}
