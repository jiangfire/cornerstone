package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

// ListTokens lists tokens
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
// @Success      200  {object}  dto.APIResponse{data=dto.TokenListData}
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /api/v1/tokens [get]
func ListTokens(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	isMaster := middleware.IsMasterToken(c)

	tokenService := services.NewTokenService(db.DB())
	tokens, err := tokenService.ListTokens(tokenID, isMaster)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, dto.TokenListData{Tokens: tokens, Total: len(tokens)})
}

// CreateToken creates a token (requires master token)
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
// @Param        body  body  dto.TokenCreateRequest  true  "Token to create"
// @Success      200  {object}  dto.APIResponse{data=dto.TokenCreateData}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - requires Master Token"
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /api/v1/tokens [post]
func CreateToken(c *gin.Context) {
	var req dto.TokenCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	tokenService := services.NewTokenService(db.DB())
	token, err := tokenService.CreateToken(req)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, dto.TokenCreateData{
		ID:        token.ID,
		Name:      token.Name,
		Scopes:    token.Scopes,
		ExpiresAt: token.ExpiresAt,
		Token:     token.Token,
	})
}

// DeleteToken deletes a token
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
// @Success      200  {object}  dto.APIResponse{data=object}
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - insufficient permissions"
// @Failure      404  {object}  dto.ErrorResponse  "Token not found"
// @Router       /api/v1/tokens/{id} [delete]
func DeleteToken(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	isMaster := middleware.IsMasterToken(c)
	targetID := c.Param("id")

	tokenService := services.NewTokenService(db.DB())
	if err := tokenService.DeleteToken(tokenID, targetID, isMaster); err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, dto.TokenDeleteData{ID: targetID})
}

// UpdateToken updates token permissions (requires master token)
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
// @Param        body  body  dto.TokenUpdateRequest  true  "Token update fields"
// @Success      200  {object}  dto.APIResponse{data=dto.TokenObject}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - requires Master Token"
// @Failure      404  {object}  dto.ErrorResponse  "Token not found"
// @Router       /api/v1/tokens/{id} [put]
func UpdateToken(c *gin.Context) {
	targetID := c.Param("id")

	var req dto.TokenUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	tokenService := services.NewTokenService(db.DB())
	token, err := tokenService.UpdateToken(targetID, req.Scopes, req.ExpiresAt)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, token)
}
