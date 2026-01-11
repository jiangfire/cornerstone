package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// ListUsers 获取用户列表（用于选择成员/共享用户）
func ListUsers(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Query("org_id")
	dbID := c.Query("db_id")

	userService := services.NewUserService(db.DB())
	users, err := userService.ListAvailableUsers(userID, orgID, dbID)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, gin.H{
		"users": users,
		"total": len(users),
	})
}

// SearchUsers 搜索用户
func SearchUsers(c *gin.Context) {
	userID := middleware.GetUserID(c)
	query := c.Query("q")

	if query == "" {
		types.Success(c, gin.H{
			"users": []interface{}{},
			"total": 0,
		})
		return
	}

	userService := services.NewUserService(db.DB())
	users, err := userService.SearchUsers(userID, query)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, gin.H{
		"users": users,
		"total": len(users),
	})
}