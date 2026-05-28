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
//
// @Summary      List all tokens
// @Description  Returns all tokens visible to the current token. Master tokens see everything; client tokens see only themselves.
// @Tags         tokens
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"tokens":[...],"total":0}}"
// @Failure      500  {object}  map[string]any
// @Router       /tokens [get]
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
//
// @Summary      Create a new token
// @Description  Create a new API token. Requires Master Token. The token value is returned only once and cannot be retrieved again.
// @Tags         tokens
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  object  true  "Token to create"  example({"name":"my-token","scopes":"read,write","expires_at":"2026-12-31T00:00:00Z"})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","name":"...","is_master":false,"scopes":"...","token":"cs_..."}}"
// @Failure      400  {object}  map[string]any
// @Failure      500  {object}  map[string]any
// @Router       /tokens [post]
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
//
// @Summary      Delete a token
// @Description  Delete a token by ID. Requires Master Token to delete tokens other than your own.
// @Tags         tokens
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Token ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"..."}}"
// @Failure      400  {object}  map[string]any
// @Router       /tokens/{id} [delete]
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
//
// @Summary      Update a token
// @Description  Update token scopes and/or expiration. Requires Master Token.
// @Tags         tokens
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string  true  "Token ID"
// @Param        body  body  object  true  "Token update fields"  example({"scopes":"read","expires_at":"2026-12-31T00:00:00Z"})
// @Success      200  {object}  map[string]any
// @Failure      400  {object}  map[string]any
// @Router       /tokens/{id} [put]
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
