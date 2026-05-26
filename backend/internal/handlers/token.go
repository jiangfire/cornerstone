package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// ListTokens 列出 Token
func ListTokens(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	isMaster := middleware.IsMasterToken(c)

	tokenService := services.NewTokenService(db.DB())
	tokens, err := tokenService.ListTokens(tokenID, isMaster)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, gin.H{"tokens": tokens, "total": len(tokens)})
}

// CreateToken 创建 Token（需 Master Token）
func CreateToken(c *gin.Context) {
	var req services.CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	tokenService := services.NewTokenService(db.DB())
	token, err := tokenService.CreateToken(req)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"id":         token.ID,
		"name":       token.Name,
		"is_master":  token.IsMaster,
		"scopes":     token.Scopes,
		"expires_at": token.ExpiresAt,
		"created_at": token.CreatedAt,
		"token":      token.Token,
	})
}

// DeleteToken 删除 Token
func DeleteToken(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	isMaster := middleware.IsMasterToken(c)
	targetID := c.Param("id")

	tokenService := services.NewTokenService(db.DB())
	if err := tokenService.DeleteToken(tokenID, targetID, isMaster); err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, gin.H{"id": targetID})
}

// UpdateToken 更新 Token 权限（需 Master Token）
func UpdateToken(c *gin.Context) {
	targetID := c.Param("id")

	var req struct {
		Scopes    string     `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	tokenService := services.NewTokenService(db.DB())
	token, err := tokenService.UpdateToken(targetID, req.Scopes, req.ExpiresAt)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, token)
}
