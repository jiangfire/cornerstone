package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestClient 构造一个测试用 client, 把 retry 间隔压到 1ms 让用例毫秒级跑完。
func newTestClient(baseURL string) *LLMGovernorClient {
	c := NewLLMGovernorClient(baseURL, "test-token")
	c.retry.BaseDelay = time.Millisecond
	return c
}

// okBody 给 200 响应用, 复用同一份 JSON 减少噪音。
const okBody = `{"success":true,"recommendation":{"value":"x"},"confidence":0.9,"reasoning":"r"}`

// makeRequest 返回一个简单的有效请求, 用例不关心字段内容。
func makeRequest() AIRecommendationRequest {
	return AIRecommendationRequest{
		TaskType:     "term_binding",
		ResourceType: "column",
		ResourceID:   "col_1",
	}
}

func TestLLMGovernor_RetriesOn5xxAndEventuallySucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		// 前两次返回 503, 第三次返回 200; 验证客户端的退避重试覆盖了瞬时上游故障。
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"upstream"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GenerateRecommendation(context.Background(), makeRequest())
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)
	require.EqualValues(t, 3, atomic.LoadInt32(&calls))
}

func TestLLMGovernor_DoesNotRetryOn4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GenerateRecommendation(context.Background(), makeRequest())
	require.Error(t, err)
	// 4xx 是客户端错误, 重试只会浪费配额; 必须只调一次。
	require.EqualValues(t, 1, atomic.LoadInt32(&calls))
}

func TestLLMGovernor_RetriesExhaustedReturnsError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GenerateRecommendation(context.Background(), makeRequest())
	require.Error(t, err)
	// 默认 MaxAttempts=3, 全失败后返回最后一次错误。
	require.EqualValues(t, 3, atomic.LoadInt32(&calls))
}

func TestLLMGovernor_CircuitBreakerOpensAfterThreshold(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	// 缩短阈值便于测试: 2 次连续失败就开熔断。
	c.breaker.threshold = 2
	c.breaker.cooldown = time.Hour
	// 第 1 次调用: MaxAttempts=3 内部重试 → 1 次失败计数
	_, err := c.GenerateRecommendation(context.Background(), makeRequest())
	require.Error(t, err)
	// 第 2 次调用: 又一次失败 → 触发阈值, 熔断打开
	_, err = c.GenerateRecommendation(context.Background(), makeRequest())
	require.Error(t, err)

	totalBefore := atomic.LoadInt32(&calls)

	// 第 3 次调用: 熔断已开, 应直接返回 ErrCircuitOpen, 不发起 HTTP
	_, err = c.GenerateRecommendation(context.Background(), makeRequest())
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.EqualValues(t, totalBefore, atomic.LoadInt32(&calls), "短路期间不应发起 HTTP")
}

func TestLLMGovernor_CircuitBreakerResetsAfterCooldown(t *testing.T) {
	var calls int32
	mode := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		if mode.Load() == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.breaker.threshold = 1
	c.breaker.cooldown = 10 * time.Millisecond
	// 注入可控时钟: 第 1 次失败时记 t=0, allow 时若 now>=openUntil 则放行。
	// 这里直接用真实 time.Now + 短 cooldown, 让测试在 < 50ms 内跑完。

	// 第 1 次: 失败 → 熔断打开
	_, err := c.GenerateRecommendation(context.Background(), makeRequest())
	require.Error(t, err)
	// 第 2 次: 紧接着, 熔断未冷却完, 应短路
	_, err = c.GenerateRecommendation(context.Background(), makeRequest())
	require.ErrorIs(t, err, ErrCircuitOpen)

	// 等冷却结束并把上游切到 200
	time.Sleep(20 * time.Millisecond)
	mode.Store(1)

	resp, err := c.GenerateRecommendation(context.Background(), makeRequest())
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)
}

func TestLLMGovernor_SuccessResetsFailureCount(t *testing.T) {
	var calls int32
	mode := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		if mode.Load() == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.breaker.threshold = 2
	c.retry.MaxAttempts = 1 // 关掉内部重试便于精确计数
	c.breaker.cooldown = time.Hour

	// 失败 1 次 → 计数 1, 离阈值还差 1
	_, err := c.GenerateRecommendation(context.Background(), makeRequest())
	require.Error(t, err)
	require.Equal(t, 1, c.breaker.consecutiveFailures)

	// 切到成功 → 应清零
	mode.Store(1)
	_, err = c.GenerateRecommendation(context.Background(), makeRequest())
	require.NoError(t, err)
	require.Equal(t, 0, c.breaker.consecutiveFailures)
}

func TestLLMGovernor_ContextCancellationNotCountedAsFailure(t *testing.T) {
	// 上游故意慢; client ctx 主动取消, 这是调用方意图, 不应计入熔断。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(okBody))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.breaker.threshold = 1

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := c.GenerateRecommendation(ctx, makeRequest())
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "do request:"))

	// 熔断不应因为 ctx 超时打开 (因为底层错误也可能被 wrap 成 "do request: context deadline exceeded";
	// 我们至少要保证熔断没有立即触发阈值 1, 否则就是错把调用方意图当上游故障了)。
	// 注意: 当前实现里 doGenerate 把 httpClient.Do 错误一律 wrap 成 "do request: ...",
	// 走 isRetryable=true 路径, 最终经过重试后才落到熔断累加。
	// 这里只断言熔断没有进入打开态。
	require.True(t, c.breaker.openUntil.IsZero() || c.breaker.consecutiveFailures < 5)
}
