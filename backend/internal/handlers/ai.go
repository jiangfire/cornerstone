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
