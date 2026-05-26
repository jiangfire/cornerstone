package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	applog "github.com/jiangfire/cornerstone/backend/pkg/log"
)

type AIRequest struct {
	Message string `json:"message" binding:"required"`
}

var aiAgent *services.AIAgent

func InitAIAgent(agent *services.AIAgent) {
	aiAgent = agent
}

func ChatWithAI(c *gin.Context) {
	if aiAgent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI agent not configured"})
		return
	}

	var req AIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
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
		return services.ExecuteAITool(name, args)
	}

	reply, err := aiAgent.Chat(messages, toolExecutor)
	if err != nil {
		applog.Errorf("AI chat error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI request failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reply": reply})
}
