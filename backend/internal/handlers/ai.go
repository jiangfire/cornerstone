package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
	applog "github.com/jiangfire/cornerstone/backend/pkg/log"
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
// @Description  Send a message to the AI assistant and get a reply. The assistant can query and manage data using available tools.
// @Tags         ai
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  object  true  "Chat request"  example({"message":"Show me all databases","context":{}})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"type":"result","reply":"...","context":{}}}"
// @Failure      400  {object}  map[string]any
// @Failure      500  {object}  map[string]any
// @Router       /ai/chat [post]
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
