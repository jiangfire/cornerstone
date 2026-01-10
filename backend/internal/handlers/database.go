package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// CreateDatabase 创建数据库
func CreateDatabase(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.CreateDatabase(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":          database.ID,
		"name":        database.Name,
		"description": database.Description,
		"owner_id":    database.OwnerID,
		"is_public":   database.IsPublic,
		"is_personal": database.IsPersonal,
		"created_at":  database.CreatedAt,
	})
}

// ListDatabases 获取数据库列表
func ListDatabases(c *gin.Context) {
	userID := middleware.GetUserID(c)

	dbService := services.NewDatabaseService(db.DB())
	databases, err := dbService.ListDatabases(userID)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, gin.H{
		"databases": databases,
		"total":     len(databases),
	})
}

// GetDatabase 获取数据库详情
func GetDatabase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.GetDatabase(dbID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, database)
}

// UpdateDatabase 更新数据库信息
func UpdateDatabase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")

	var req services.UpdateDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.UpdateDatabase(dbID, req, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":          database.ID,
		"name":        database.Name,
		"description": database.Description,
		"is_public":   database.IsPublic,
		"updated_at":  database.UpdatedAt,
	})
}

// DeleteDatabase 删除数据库
func DeleteDatabase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	if err := dbService.DeleteDatabase(dbID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "数据库已删除",
	})
}

// ShareDatabase 分享数据库
func ShareDatabase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")

	var req services.ShareDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	if err := dbService.ShareDatabase(dbID, req, userID); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "数据库分享成功",
	})
}

// ListDatabaseUsers 获取数据库用户列表
func ListDatabaseUsers(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	users, err := dbService.ListDatabaseUsers(dbID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"users": users,
		"total": len(users),
	})
}

// RemoveDatabaseUser 移除数据库用户
func RemoveDatabaseUser(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")
	removeUserID := c.Param("user_id")

	dbService := services.NewDatabaseService(db.DB())
	if err := dbService.RemoveDatabaseUser(dbID, removeUserID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "用户已移除",
	})
}

// UpdateDatabaseUserRole 更新数据库用户角色
func UpdateDatabaseUserRole(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dbID := c.Param("id")
	updateUserID := c.Param("user_id")

	var req services.ShareDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	if err := dbService.UpdateDatabaseUserRole(dbID, updateUserID, req, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "角色已更新",
	})
}
