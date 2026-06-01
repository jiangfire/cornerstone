package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/mcp"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMCPHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB, *models.Token) {
	t.Helper()
	db := testutil.SetupTestDB(t)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	pkgdb.SetDB(db)

	t.Setenv("MASTER_TOKEN", master.Token)

	router := gin.New()
	router.Use(middleware.Auth())
	router.POST("/mcp", HandleMCP)
	router.GET("/mcp", HandleMCPGet)
	router.OPTIONS("/mcp", HandleMCPOptions)

	return router, db, master
}

func doMCPRequest(t *testing.T, router *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.DoRequest(t, router, method, path, token, body)
}

func doMCPRequestRaw(t *testing.T, router *gin.Engine, method, path, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, path, bodyReader)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func doMCPRequestWithAccept(t *testing.T, router *gin.Engine, method, path, token, accept string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, path, bodyReader)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestParseMCPPayload_SingleRequestWithMethodAndID(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2025-03-26"}}`
	requests, kind, err := parseMCPPayload(strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, payloadIncludesRequests, kind)
	require.Len(t, requests, 1)
	assert.Equal(t, "initialize", requests[0].Method)
	assert.Equal(t, "2.0", requests[0].JSONRPC)
}

func TestParseMCPPayload_NotificationNoID(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	requests, kind, err := parseMCPPayload(strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, payloadNotificationsOnly, kind)
	require.Len(t, requests, 1)
	assert.Equal(t, "notifications/initialized", requests[0].Method)
	assert.Empty(t, requests[0].ID)
}

func TestParseMCPPayload_ResponseNoMethod(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`
	requests, kind, err := parseMCPPayload(strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, payloadResponsesOnly, kind)
	require.Len(t, requests, 1)
	assert.Empty(t, requests[0].Method)
}

func TestParseMCPPayload_BatchRequest(t *testing.T) {
	body := `[{"jsonrpc":"2.0","method":"initialize","id":1},{"jsonrpc":"2.0","method":"ping","id":2}]`
	requests, kind, err := parseMCPPayload(strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, payloadIncludesRequests, kind)
	require.Len(t, requests, 2)
	assert.Equal(t, "initialize", requests[0].Method)
	assert.Equal(t, "ping", requests[1].Method)
}

func TestParseMCPPayload_EmptyBody(t *testing.T) {
	_, _, err := parseMCPPayload(strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty request body")
}

func TestParseMCPPayload_InvalidJSON(t *testing.T) {
	_, _, err := parseMCPPayload(strings.NewReader("{not json"))
	require.Error(t, err)
}

func TestParseMCPPayload_EmptyBatch(t *testing.T) {
	_, _, err := parseMCPPayload(strings.NewReader("[]"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty batch")
}

func TestClassifyMCPPayload_IncludesRequests(t *testing.T) {
	method := "initialize"
	envelopes := []mcpEnvelope{
		{Method: &method, ID: json.RawMessage(`1`)},
	}
	assert.Equal(t, payloadIncludesRequests, classifyMCPPayload(envelopes))
}

func TestClassifyMCPPayload_NotificationsOnly(t *testing.T) {
	method := "notifications/initialized"
	envelopes := []mcpEnvelope{
		{Method: &method},
	}
	assert.Equal(t, payloadNotificationsOnly, classifyMCPPayload(envelopes))
}

func TestClassifyMCPPayload_ResponsesOnly(t *testing.T) {
	envelopes := []mcpEnvelope{
		{ID: json.RawMessage(`1`)},
	}
	assert.Equal(t, payloadResponsesOnly, classifyMCPPayload(envelopes))
}

func TestClassifyMCPPayload_EmptyMethod(t *testing.T) {
	blank := "   "
	envelopes := []mcpEnvelope{
		{Method: &blank, ID: json.RawMessage(`1`)},
	}
	assert.Equal(t, payloadResponsesOnly, classifyMCPPayload(envelopes))
}

func TestHandleMCP_InitializeRequest(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"id":      1,
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
		},
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", master.Token, body)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "2.0", resp["jsonrpc"])

	result, ok := resp["result"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2025-03-26", result["protocolVersion"])

	capabilities, ok := result["capabilities"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, capabilities, "tools")

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "cornerstone-mcp", serverInfo["name"])
}

func TestHandleMCP_InvalidJSON(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	rec := doMCPRequestRaw(t, router, "POST", "/mcp", master.Token, "{bad json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "2.0", resp["jsonrpc"])

	errObj, ok := resp["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(-32700), errObj["code"])
	assert.Equal(t, "Parse error", errObj["message"])
}

func TestHandleMCP_NotificationOnly(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", master.Token, body)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Empty(t, rec.Body.Bytes())
}

func TestHandleMCP_ResponseOnly(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"result":  map[string]interface{}{},
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", master.Token, body)
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestHandleMCP_BatchRequests(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "initialize",
			"id":      1,
			"params":  map[string]interface{}{},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "ping",
			"id":      2,
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responses []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &responses))
	require.Len(t, responses, 2)
	assert.Equal(t, "2.0", responses[0]["jsonrpc"])
	assert.Equal(t, "2.0", responses[1]["jsonrpc"])

	result0, _ := responses[0]["result"].(map[string]interface{})
	assert.Contains(t, result0, "protocolVersion")

	result1, _ := responses[1]["result"].(map[string]interface{})
	assert.Empty(t, result1)
}

func TestHandleMCP_BatchRequests_PingResult(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "ping",
			"id":      1,
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "2.0", resp["jsonrpc"])
	result, ok := resp["result"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, result)
}

func TestHandleMCP_ToolsList(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"id":      3,
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", master.Token, body)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "2.0", resp["jsonrpc"])

	result, ok := resp["result"].(map[string]interface{})
	require.True(t, ok)
	tools, ok := result["tools"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(tools), 10)
}

func TestHandleMCPOptions(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	rec := doMCPRequestRaw(t, router, "OPTIONS", "/mcp", master.Token, "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "POST, GET, OPTIONS", rec.Header().Get("Allow"))
}

func TestHandleMCPGet_WithoutAcceptSSE(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	rec := doMCPRequestRaw(t, router, "GET", "/mcp", master.Token, "")
	assert.Equal(t, http.StatusNotAcceptable, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "http", resp["transport"])
	assert.Equal(t, "streamable-http", resp["mode"])
}

func TestAcceptsSSE_VariousHeaders(t *testing.T) {
	assert.True(t, acceptsSSE("text/event-stream"))
	assert.True(t, acceptsSSE("text/event-stream, application/json"))
	assert.True(t, acceptsSSE("application/json, text/event-stream"))
	assert.True(t, acceptsSSE("Text/Event-Stream"))
	assert.False(t, acceptsSSE("application/json"))
	assert.False(t, acceptsSSE(""))
	assert.False(t, acceptsSSE("text/html"))
}

func TestConfigureMCP_AppliesOptions(t *testing.T) {
	originalKeepalive := mcpKeepaliveInterval
	originalRetry := mcpRetryInterval
	originalBuffer := mcpReplayBuffer
	originalHub := mcpHub

	defer func() {
		mcpKeepaliveInterval = originalKeepalive
		mcpRetryInterval = originalRetry
		mcpReplayBuffer = originalBuffer
		mcpHub = originalHub
	}()

	ConfigureMCP(MCPOptions{
		SSEKeepaliveInterval: 10 * time.Second,
		SSERetryInterval:     1 * time.Second,
		SSEReplayBuffer:      256,
	})

	assert.Equal(t, 10*time.Second, mcpKeepaliveInterval)
	assert.Equal(t, 1*time.Second, mcpRetryInterval)
	assert.Equal(t, 256, mcpReplayBuffer)
	assert.NotNil(t, mcpHub)
}

func TestConfigureMCP_IgnoresZeroValues(t *testing.T) {
	originalKeepalive := mcpKeepaliveInterval
	originalRetry := mcpRetryInterval
	originalBuffer := mcpReplayBuffer
	originalHub := mcpHub

	defer func() {
		mcpKeepaliveInterval = originalKeepalive
		mcpRetryInterval = originalRetry
		mcpReplayBuffer = originalBuffer
		mcpHub = originalHub
	}()

	ConfigureMCP(MCPOptions{})

	assert.Equal(t, originalKeepalive, mcpKeepaliveInterval)
	assert.Equal(t, originalRetry, mcpRetryInterval)
	assert.Equal(t, originalBuffer, mcpReplayBuffer)
	assert.Equal(t, originalHub, mcpHub)
}

func TestWriteSSEJSON_Format(t *testing.T) {
	var buf bytes.Buffer
	writeSSEJSON(&buf, "message", mcp.Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  map[string]interface{}{"key": "value"},
	})

	output := buf.String()
	assert.Contains(t, output, "event: message\n")
	assert.Contains(t, output, "data: ")
	assert.Contains(t, output, `"jsonrpc":"2.0"`)

	lines := strings.Split(output, "\n")
	hasIDLine := false
	for _, line := range lines {
		if strings.HasPrefix(line, "id: ") {
			hasIDLine = true
			assert.NotEmpty(t, strings.TrimPrefix(line, "id: "))
		}
	}
	assert.True(t, hasIDLine)
}

func TestWriteSSEJSONWithID_Format(t *testing.T) {
	var buf bytes.Buffer
	writeSSEJSONWithID(&buf, "evt-42", "message", map[string]string{"k": "v"})

	output := buf.String()
	assert.Contains(t, output, "event: message\n")
	assert.Contains(t, output, "id: evt-42\n")
	assert.Contains(t, output, `data: {"k":"v"}`)
	assert.True(t, strings.HasSuffix(output, "\n\n"))
}

func TestWriteSSEJSONWithID_EmptyID(t *testing.T) {
	var buf bytes.Buffer
	writeSSEJSONWithID(&buf, "", "message", map[string]string{"k": "v"})

	output := buf.String()
	assert.NotContains(t, output, "id:")
	assert.Contains(t, output, "event: message\n")
	assert.Contains(t, output, `data: {"k":"v"}`)
}

func TestWriteSSEJSONWithID_EmptyEvent(t *testing.T) {
	var buf bytes.Buffer
	writeSSEJSONWithID(&buf, "id-1", "", map[string]string{"k": "v"})

	output := buf.String()
	assert.NotContains(t, output, "event:")
	assert.Contains(t, output, "id: id-1\n")
}

func TestHandleMCP_Unauthorized(t *testing.T) {
	router, _, _ := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"id":      1,
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", "", body)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleMCP_BatchWithNotificationsOnly(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHandleMCP_SSEStreamResponse(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"id":      1,
		"params":  map[string]interface{}{},
	}
	rec := doMCPRequestWithAccept(t, router, "POST", "/mcp", master.Token, "text/event-stream", body)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))

	output := rec.Body.String()
	assert.Contains(t, output, "event: message")
	assert.Contains(t, output, "data:")
	assert.Contains(t, output, `"jsonrpc":"2.0"`)
}

func TestHandleMCP_BatchMixedRequestsAndNotifications(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "ping",
			"id":      2,
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "2.0", resp["jsonrpc"])
	_, hasResult := resp["result"]
	assert.True(t, hasResult)
}

func TestHandleMCP_MethodNotFound(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "nonexistent/method",
		"id":      99,
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", master.Token, body)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errObj, ok := resp["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(-32601), errObj["code"])
	assert.Equal(t, "Method not found", errObj["message"])
}

func TestHandleMCPGet_WithSSEAcceptButNoContext(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	req, err := http.NewRequest("GET", "/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Accept", "text/event-stream")

	ctx, cancel := contextWithTimeout(t, 2*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
}

func TestHandleMCPGet_SSEStreamWritesConnected(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	req, err := http.NewRequest("GET", "/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Accept", "text/event-stream")

	ctx, cancel := contextWithTimeout(t, 2*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "retry:")
	assert.Contains(t, body, ": stream opened")
	assert.Contains(t, body, "notifications/stream/connected")
	assert.Contains(t, body, "stream_id")
}

func contextWithTimeout(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), timeout)
}

func TestHandleMCPGet_CancelContext(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequest("GET", "/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Accept", "text/event-stream")
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not finish after context cancellation")
	}
}

func TestHandleMCP_InitializeWithNoParams(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"id":      1,
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", master.Token, body)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	result, ok := resp["result"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2025-03-26", result["protocolVersion"])
}

func TestHandleMCP_Ping(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "ping",
		"id":      42,
	}
	rec := doMCPRequest(t, router, "POST", "/mcp", master.Token, body)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	result, ok := resp["result"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, result)
}

func TestWriteReplayStatusNotification_NotRequested(t *testing.T) {
	var buf bytes.Buffer
	writeReplayStatusNotification(&buf, "stream-1", "", mcp.ReplayNotRequested, 0)
	assert.Empty(t, buf.String())
}

func TestWriteReplayStatusNotification_Replayed(t *testing.T) {
	var buf bytes.Buffer
	writeReplayStatusNotification(&buf, "stream-1", "evt-5", mcp.ReplayReplayed, 3)
	output := buf.String()
	assert.Contains(t, output, "notifications/stream/resumed")
	assert.Contains(t, output, `"replayed":3`)
	assert.Contains(t, output, `"status":"replayed"`)
}

func TestWriteReplayStatusNotification_AtHead(t *testing.T) {
	var buf bytes.Buffer
	writeReplayStatusNotification(&buf, "stream-1", "evt-5", mcp.ReplayAtHead, 0)
	output := buf.String()
	assert.Contains(t, output, "notifications/stream/resumed")
	assert.Contains(t, output, `"status":"at_head"`)
}

func TestWriteReplayStatusNotification_Missed(t *testing.T) {
	var buf bytes.Buffer
	writeReplayStatusNotification(&buf, "stream-1", "evt-missing", mcp.ReplayMissed, 0)
	output := buf.String()
	assert.Contains(t, output, "notifications/stream/replay_unavailable")
	assert.Contains(t, output, `"status":"missed"`)
}

func TestHandleMCPGet_AllowHeader(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	rec := doMCPRequestRaw(t, router, "GET", "/mcp", master.Token, "")
	assert.Equal(t, "POST, GET, OPTIONS", rec.Header().Get("Allow"))
}

func TestHandleMCP_EmptyBodyReturnsParseError(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	req, err := http.NewRequest("POST", "/mcp", strings.NewReader(""))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, float64(-32700), errObj["code"])
}

func TestParseMCPPayload_WhitespaceBody(t *testing.T) {
	_, _, err := parseMCPPayload(strings.NewReader("   \n\t  "))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty request body")
}

func TestParseMCPPayload_BatchInvalidItem(t *testing.T) {
	body := `[{"jsonrpc":"2.0","method":"ping","id":1}, "not an object"]`
	_, _, err := parseMCPPayload(strings.NewReader(body))
	require.Error(t, err)
}

func TestHandleMCP_SSEStreamForBatch(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "initialize",
			"id":      1,
			"params":  map[string]interface{}{},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "ping",
			"id":      2,
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	body := w.Body.String()
	assert.Equal(t, 2, strings.Count(body, "event: message"))
}

func TestHandleMCP_BatchWithOnlyNotifications(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHandleMCP_BatchInvalidJSON(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	req, err := http.NewRequest("POST", "/mcp", strings.NewReader("[{bad}]"))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleMCPOptions_AllowHeader(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	req, err := http.NewRequest("OPTIONS", "/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "POST, GET, OPTIONS", w.Header().Get("Allow"))
}

func TestDisableWriteTimeout_DoesNotPanic(t *testing.T) {
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		disableWriteTimeout(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWriteSSEJSON_MarshalError(t *testing.T) {
	var buf bytes.Buffer
	writeSSEJSON(&buf, "message", make(chan int))
	assert.Empty(t, buf.String())
}

func TestWriteSSEJSONWithID_NilPayload(t *testing.T) {
	var buf bytes.Buffer
	writeSSEJSONWithID(&buf, "id-1", "message", nil)
	output := buf.String()
	assert.Contains(t, output, "data: null")
}

func TestCollectMCPResponses_Mixed(t *testing.T) {
	router, db, master := setupMCPHandlerTest(t)
	_ = router

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Set("token_id", master.ID)
	server := mcp.NewServer(mcp.NewToolServiceWithNotifier(db, master.ID, nil), "test")

	requests := []mcp.Request{
		{JSONRPC: "2.0", Method: "ping", ID: json.RawMessage(`1`)},
		{JSONRPC: "2.0", Method: "notifications/initialized"},
		{JSONRPC: "2.0", Method: "initialize", ID: json.RawMessage(`2`), Params: json.RawMessage(`{}`)},
	}
	responses := collectMCPResponses(c, server, requests)
	require.Len(t, responses, 2)
}

func TestStreamMCPResponses(t *testing.T) {
	router, db, master := setupMCPHandlerTest(t)
	_ = router

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)

	server := mcp.NewServer(mcp.NewToolServiceWithNotifier(db, master.ID, nil), "test")

	requests := []mcp.Request{
		{JSONRPC: "2.0", Method: "initialize", ID: json.RawMessage(`1`), Params: json.RawMessage(`{}`)},
		{JSONRPC: "2.0", Method: "ping", ID: json.RawMessage(`2`)},
	}
	streamMCPResponses(c, server, requests)

	output := w.Body.String()
	assert.Equal(t, 2, strings.Count(output, "event: message"))
	assert.Contains(t, output, `"jsonrpc":"2.0"`)
}

func TestHandleMCPGet_406ResponseBody(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	req, err := http.NewRequest("GET", "/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	methods, ok := resp["methods"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, methods, "POST")
	assert.Contains(t, methods, "GET")
}

func TestParseMCPPayload_BatchWithResponsesOnly(t *testing.T) {
	body := `[{"jsonrpc":"2.0","id":1,"result":{}},{"jsonrpc":"2.0","id":2,"result":{}}]`
	requests, kind, err := parseMCPPayload(strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, payloadResponsesOnly, kind)
	require.Len(t, requests, 2)
}

func TestHandleMCP_BatchWithResponsesOnly(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{},
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHandleMCP_BatchMixedRequestsAndResponses(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "ping",
			"id":      2,
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var responses []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &responses))
	require.Len(t, responses, 2)

	errObj, ok := responses[0]["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(-32601), errObj["code"])
}

func TestHandleMCP_SSEBatchDoesNotIncludeNotifications(t *testing.T) {
	router, _, master := setupMCPHandlerTest(t)

	batch := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "initialize",
			"id":      1,
			"params":  map[string]interface{}{},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "ping",
			"id":      2,
		},
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/mcp", bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Equal(t, 2, strings.Count(body, "event: message"))
}

func TestParseMCPPayload_ReaderError(t *testing.T) {
	_, _, err := parseMCPPayload(errReader(0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body")
}

type errReader int

func (errReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func TestWriteSSEJSONWithID_NilData(t *testing.T) {
	var buf bytes.Buffer
	writeSSEJSONWithID(&buf, "", "message", (*int)(nil))
	assert.Contains(t, buf.String(), "data: null")
}
