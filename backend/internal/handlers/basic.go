package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// Register 用户注册
func Register(c *gin.Context) {
	var req services.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	authService := services.NewAuthService(db.DB())
	response, err := authService.Register(req)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, response)
}

// Login 用户登录
func Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	authService := services.NewAuthService(db.DB())
	response, err := authService.Login(req)
	if err != nil {
		types.Error(c, 401, err.Error())
		return
	}

	types.Success(c, response)
}

// GetUserInfo 获取用户信息
func GetUserInfo(c *gin.Context) {
	userID := middleware.GetUserID(c)

	authService := services.NewAuthService(db.DB())
	user, err := authService.GetUserByID(userID)
	if err != nil {
		types.Error(c, 404, err.Error())
		return
	}

	types.Success(c, user)
}

// Logout 用户登出
func Logout(c *gin.Context) {
	// 从 Authorization header 获取 token
	token := c.GetHeader("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	authService := services.NewAuthService(db.DB())
	if err := authService.Logout(token); err != nil {
		types.Error(c, 500, "登出失败: "+err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "登出成功",
	})
}