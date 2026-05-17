package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrCircuitOpen 表示熔断器处于打开状态, 已短路所有外发调用。
var ErrCircuitOpen = errors.New("LLM Governor 熔断器打开, 上游连续失败已触发短路")

// retryConfig 控制单次调用的重试策略。
// MaxAttempts 是含首次的总尝试次数;BaseDelay 是首次重试间隔, 之后翻倍。
type retryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

// circuitBreaker 是一个最小化的连续失败熔断器:
//   - 连续失败 ≥ Threshold 次后, 把电路打开 Cooldown 时长
//   - 打开期间所有调用直接返回 ErrCircuitOpen, 不发起 HTTP
//   - 任意一次成功调用都把连续失败计数清零
//
// 这不是状态机式的 half-open 半开探测器, 也不是按时间窗口的失败率熔断器;
// 选最简单的实现是因为 LLM Governor 是非关键的辅助路径, 短路逻辑只需要"上游挂了别再敲它"。
type circuitBreaker struct {
	mu                  sync.Mutex
	consecutiveFailures int
	openUntil           time.Time
	threshold           int
	cooldown            time.Duration
	now                 func() time.Time // 注入时钟以便测试
}

// allow 检查当前是否允许放行调用。
// 调用方在拿到 true 后必须配套调用 recordResult。
func (b *circuitBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openUntil.IsZero() {
		return true
	}
	if b.now().Before(b.openUntil) {
		return false
	}
	// 冷却结束: 清零, 重新开放调用
	b.openUntil = time.Time{}
	b.consecutiveFailures = 0
	return true
}

// recordResult 把一次调用的结果计入熔断器。
// success=true 直接重置计数;false 累加, 触发阈值时打开电路。
func (b *circuitBreaker) recordResult(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if success {
		b.consecutiveFailures = 0
		b.openUntil = time.Time{}
		return
	}
	b.consecutiveFailures++
	if b.consecutiveFailures >= b.threshold {
		b.openUntil = b.now().Add(b.cooldown)
	}
}

// LLMGovernorClient LLM Governor 客户端
type LLMGovernorClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	retry      retryConfig
	breaker    *circuitBreaker
}

// NewLLMGovernorClient 创建 LLM Governor 客户端
//
// 默认策略:
//   - HTTP 超时 30s
//   - 最多 3 次尝试 (1 次首次 + 2 次重试), 指数退避起点 200ms (200ms → 400ms)
//   - 连续 5 次失败后熔断 30s, 期间直接返回 ErrCircuitOpen
//
// 这些默认值面向"非关键辅助上游", 不暴露配置项;若以后需要按域名分级, 再加构造器选项。
func NewLLMGovernorClient(baseURL, token string) *LLMGovernorClient {
	return &LLMGovernorClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retry: retryConfig{
			MaxAttempts: 3,
			BaseDelay:   200 * time.Millisecond,
		},
		breaker: &circuitBreaker{
			threshold: 5,
			cooldown:  30 * time.Second,
			now:       time.Now,
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
	Success        bool                   `json:"success"`
	Recommendation map[string]interface{} `json:"recommendation"`
	Confidence     float64                `json:"confidence"`
	Reasoning      string                 `json:"reasoning"`
	Error          string                 `json:"error,omitempty"`
}

// transientHTTPError 表示一次需要重试的 HTTP 失败 (5xx 或 429)。
// 区别于 4xx 客户端错误, 后者重试只是浪费配额。
type transientHTTPError struct {
	status int
	body   string
}

func (e *transientHTTPError) Error() string {
	return fmt.Sprintf("transient upstream error %d: %s", e.status, e.body)
}

// isRetryable 判断错误是否值得重试。
// 重试: 网络层错误 (httpClient.Do 直接返错) + transientHTTPError (5xx/429)。
// 不重试: context 取消/超时(调用方决定的边界)、4xx 业务错误、JSON 解码失败。
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var transient *transientHTTPError
	if errors.As(err, &transient) {
		return true
	}
	// 来自 httpClient.Do 的网络层错误统一作为可重试 (url.Error / net.OpError 等)
	return strings.HasPrefix(err.Error(), "do request:")
}

// GenerateRecommendation 生成 AI 建议请求。
// 走熔断器 → 重试包装 → 实际 HTTP 调用三层。
func (c *LLMGovernorClient) GenerateRecommendation(ctx context.Context, req AIRecommendationRequest) (*AIRecommendationResponse, error) {
	if c.breaker != nil && !c.breaker.allow() {
		return nil, ErrCircuitOpen
	}

	var (
		result *AIRecommendationResponse
		err    error
	)
	attempts := max(c.retry.MaxAttempts, 1)
	for i := range attempts {
		result, err = c.doGenerate(ctx, req)
		if err == nil {
			if c.breaker != nil {
				c.breaker.recordResult(true)
			}
			return result, nil
		}
		if !isRetryable(err) || i == attempts-1 {
			break
		}
		// 指数退避: BaseDelay * 2^i; 同时尊重 ctx 取消
		backoff := c.retry.BaseDelay << i
		select {
		case <-ctx.Done():
			err = ctx.Err()
			// 取消属于调用方意图, 不计入熔断失败
			return nil, err
		case <-time.After(backoff):
		}
	}

	if c.breaker != nil {
		c.breaker.recordResult(false)
	}
	return nil, err
}

// doGenerate 是单次 HTTP 调用 (不含重试/熔断), 抽出便于上层包装。
func (c *LLMGovernorClient) doGenerate(ctx context.Context, req AIRecommendationRequest) (*AIRecommendationResponse, error) {
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
		// 5xx + 429 标记为可重试;其它 (4xx) 直接返回, 让上层走业务错误路径
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			return nil, &transientHTTPError{status: resp.StatusCode, body: string(body)}
		}
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
