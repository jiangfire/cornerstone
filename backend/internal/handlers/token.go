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
// @Description  Returns all tokens visible to the current token.
//
//	Master tokens see every token in the system.
//	Client tokens can only see their own token entry.
//	Results are sorted by creation time (newest first).
//
// @Tags         tokens
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  swagger.APIResponse{data=swagger.TokenListResponse}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      500  {object}  swagger.ErrorResponse
// @Router       /api/tokens [get]
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
// @Description  Create a new API token. Requires Master Token.
//
//	The token value (starting with "cs_") is returned only once in the response
//	and cannot be retrieved again. Store it securely.
//
//	Validation rules:
//	  - name is required and must be unique
//	  - scopes is a comma-separated string (e.g. "read,write")
//	  - expires_at is optional; if set, must be a valid ISO 8601 timestamp
//
// @Tags         tokens
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.TokenCreateRequest  true  "Token to create"
// @Success      200  {object}  swagger.APIResponse{data=swagger.TokenCreateResponse}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - requires Master Token"
// @Failure      500  {object}  swagger.ErrorResponse
// @Router       /api/tokens [post]
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
		"scopes":     token.Scopes,
		"expires_at": token.ExpiresAt,
		"created_at": token.CreatedAt,
		"token":      token.Token,
	})
}

// DeleteToken 删除 Token
//
// @Summary      Delete a token
// @Description  Delete a token by ID.
//
//	Requires Master Token to delete tokens other than your own.
//	Client tokens can only delete themselves.
//
// @Tags         tokens
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Token ID"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - insufficient permissions"
// @Failure      404  {object}  swagger.ErrorResponse  "Token not found"
// @Router       /api/tokens/{id} [delete]
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
// @Description  Update token scopes and/or expiration date. Requires Master Token.
//
//	Validation rules:
//	  - scopes is a comma-separated string (e.g. "read,write")
//	  - expires_at is optional; if set, must be a valid ISO 8601 timestamp
//
// @Tags         tokens
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string              true  "Token ID"
// @Param        body  body  swagger.TokenUpdateRequest  true  "Token update fields"
// @Success      200  {object}  swagger.APIResponse{data=swagger.TokenObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - requires Master Token"
// @Failure      404  {object}  swagger.ErrorResponse  "Token not found"
// @Router       /api/tokens/{id} [put]
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
