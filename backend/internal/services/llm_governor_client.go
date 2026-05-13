package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LLMGovernorClient LLM Governor 客户端
type LLMGovernorClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewLLMGovernorClient 创建 LLM Governor 客户端
func NewLLMGovernorClient(baseURL, token string) *LLMGovernorClient {
	return &LLMGovernorClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AIRecommendationRequest AI 建议请求
type AIRecommendationRequest struct {
	TaskType     string                 `json:"task_type"`     // term_binding, classification, dq_rule, impact_summary
	ResourceType string                 `json:"resource_type"` // column, table, database
	ResourceID   string                 `json:"resource_id"`
	Context      map[string]interface{} `json:"context"` // 额外的上下文信息
}

// AIRecommendationResponse AI 建议响应
type AIRecommendationResponse struct {
	Success      bool                   `json:"success"`
	Recommendation map[string]interface{} `json:"recommendation"`
	Confidence   float64                `json:"confidence"`
	Reasoning    string                 `json:"reasoning"`
	Error        string                 `json:"error,omitempty"`
}

// GenerateRecommendation 生成 AI 建议请求
func (c *LLMGovernorClient) GenerateRecommendation(ctx context.Context, req AIRecommendationRequest) (*AIRecommendationResponse, error) {
	url := fmt.Sprintf("%s/api/v1/recommendations", c.baseURL)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("X-Source-System", "cornerstone")
	httpReq.Header.Set("X-Trace-ID", uuid.NewString())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result AIRecommendationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GenerateTermBindingRecommendation 生成术语绑定建议
func (c *LLMGovernorClient) GenerateTermBindingRecommendation(ctx context.Context, columnID, columnName, tableID string) (*AIRecommendationResponse, error) {
	req := AIRecommendationRequest{
		TaskType:     "term_binding",
		ResourceType: "column",
		ResourceID:   columnID,
		Context: map[string]interface{}{
			"column_name": columnName,
			"table_id":    tableID,
		},
	}
	return c.GenerateRecommendation(ctx, req)
}

// GenerateClassificationRecommendation 生成分类建议
func (c *LLMGovernorClient) GenerateClassificationRecommendation(ctx context.Context, columnID, columnName, dataType string) (*AIRecommendationResponse, error) {
	req := AIRecommendationRequest{
		TaskType:     "classification",
		ResourceType: "column",
		ResourceID:   columnID,
		Context: map[string]interface{}{
			"column_name": columnName,
			"data_type":   dataType,
		},
	}
	return c.GenerateRecommendation(ctx, req)
}

// GenerateDQRuleRecommendation 生成数据质量规则建议
func (c *LLMGovernorClient) GenerateDQRuleRecommendation(ctx context.Context, columnID, columnName, dataType string) (*AIRecommendationResponse, error) {
	req := AIRecommendationRequest{
		TaskType:     "dq_rule",
		ResourceType: "column",
		ResourceID:   columnID,
		Context: map[string]interface{}{
			"column_name": columnName,
			"data_type":   dataType,
		},
	}
	return c.GenerateRecommendation(ctx, req)
}

// GenerateImpactSummary 生成影响摘要
func (c *LLMGovernorClient) GenerateImpactSummary(ctx context.Context, resourceType, resourceID, changeType string) (*AIRecommendationResponse, error) {
	req := AIRecommendationRequest{
		TaskType:     "impact_summary",
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Context: map[string]interface{}{
			"change_type": changeType,
		},
	}
	return c.GenerateRecommendation(ctx, req)
}
