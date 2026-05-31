package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
	applog "github.com/jiangfire/cornerstone/pkg/log"
)

type AIRequest struct {
	Message string         `json:"message" binding:"required"`
	Context map[string]any `json:"context"`
}

var aiAgent *services.AIAgent

func InitAIAgent(agent *services.AIAgent) {
	aiAgent = agent
}

// ChatWithAI
//
// @Summary      Chat with AI assistant
// @Description  Send a message to the AI assistant and get a reply.
//
//	The AI assistant can query and manage data using available tools.
//	Returns 503 if the LLM_API_KEY environment variable is not configured.
//	The context field is optional and can provide additional information
//	to guide the AI's responses.
//
// @Tags         ai
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.AIChatRequest  true  "Chat request"
// @Success      200  {object}  swagger.APIResponse{data=swagger.AIChatResponse}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - message is required"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      503  {object}  swagger.ErrorResponse  "AI service unavailable - LLM_API_KEY not configured"
// @Router       /api/ai/chat [post]
func ChatWithAI(c *gin.Context) {
	if aiAgent == nil {
		dto.InternalServerError(c, "AI agent not configured")
		return
	}

	var req AIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	messages := []services.Message{
		{
			Role:    "system",
			Content: "You are a data assistant for Cornerstone. Help users query and manage their data assets.",
		},
		{
			Role:    "user",
			Content: req.Message,
		},
	}

	toolExecutor := func(name string, args map[string]any) (any, error) {
		return services.ExecuteAIToolForToken(db.DB(), middleware.GetTokenID(c), name, args)
	}

	reply, err := aiAgent.Chat(messages, toolExecutor)
	if err != nil {
		applog.Errorf("AI chat error: %v", err)
		dto.InternalServerError(c, "AI request failed")
		return
	}

	dto.Success(c, gin.H{
		"type":    "result",
		"reply":   reply,
		"context": req.Context,
	})
}
