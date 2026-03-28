package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jiangfire/cornerstone/backend/internal/mcp"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

const mcpServerVersion = "dev"

const (
	defaultMCPKeepaliveInterval = 25 * time.Second
	defaultMCPRetryInterval     = 3 * time.Second
	defaultMCPReplayBuffer      = 128
)

var (
	mcpHub               = mcp.NewSSEHubWithHistory(defaultMCPReplayBuffer)
	mcpKeepaliveInterval = defaultMCPKeepaliveInterval
	mcpRetryInterval     = defaultMCPRetryInterval
	mcpReplayBuffer      = defaultMCPReplayBuffer
)

// MCPOptions 是 HTTP MCP/SSE 的运行时配置。
type MCPOptions struct {
	SSEKeepaliveInterval time.Duration
	SSERetryInterval     time.Duration
	SSEReplayBuffer      int
}

// ConfigureMCP 应用 HTTP MCP/SSE 的运行时配置。
func ConfigureMCP(options MCPOptions) {
	if options.SSEKeepaliveInterval > 0 {
		mcpKeepaliveInterval = options.SSEKeepaliveInterval
	}
	if options.SSERetryInterval > 0 {
		mcpRetryInterval = options.SSERetryInterval
	}
	if options.SSEReplayBuffer > 0 {
		mcpReplayBuffer = options.SSEReplayBuffer
		mcpHub = mcp.NewSSEHubWithHistory(options.SSEReplayBuffer)
	}
}

// HandleMCP 处理 HTTP 版 MCP 请求
func HandleMCP(c *gin.Context) {
	userID := middleware.GetUserID(c)
	server := mcp.NewServer(mcp.NewToolServiceWithNotifier(db.DB(), userID, mcpHub), mcpServerVersion)

	requests, kind, err := parseMCPPayload(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, mcp.Response{
			JSONRPC: "2.0",
			Error: &mcp.ResponseError{
				Code:    -32700,
				Message: "Parse error",
				Data:    err.Error(),
			},
		})
		return
	}

	if kind == payloadResponsesOnly || kind == payloadNotificationsOnly {
		c.Status(http.StatusAccepted)
		return
	}

	if acceptsSSE(c.GetHeader("Accept")) && kind == payloadIncludesRequests {
		disableWriteTimeout(c)
		streamMCPResponses(c, server, requests)
		return
	}

	responses := collectMCPResponses(c, server, requests)
	if len(responses) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	if len(responses) == 1 {
		c.JSON(http.StatusOK, responses[0])
		return
	}

	c.JSON(http.StatusOK, responses)
}

// HandleMCPGet 返回 GET 形式的 SSE 通道
func HandleMCPGet(c *gin.Context) {
	c.Header("Allow", "POST, GET, OPTIONS")
	if !acceptsSSE(c.GetHeader("Accept")) {
		payload := map[string]interface{}{
			"transport": "http",
			"mode":      "streamable-http",
			"methods":   []string{"POST", "GET"},
			"message":   "GET /mcp requires Accept: text/event-stream.",
		}
		c.JSON(http.StatusNotAcceptable, payload)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	disableWriteTimeout(c)

	userID := middleware.GetUserID(c)
	lastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	streamID, stream, replay, replayStatus, cleanup := mcpHub.Register(userID, lastEventID)
	defer cleanup()

	fmt.Fprintf(c.Writer, "retry: %d\n", mcpRetryInterval.Milliseconds())
	fmt.Fprint(c.Writer, ": stream opened\n\n")
	flusher, _ := c.Writer.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	writeSSEJSONWithID(c.Writer, "", "message", mcp.Notification{
		JSONRPC: "2.0",
		Method:  "notifications/stream/connected",
		Params: map[string]interface{}{
			"stream_id":         streamID,
			"keepalive_sec":     int(mcpKeepaliveInterval / time.Second),
			"retry_ms":          mcpRetryInterval.Milliseconds(),
			"replay_supported":  true,
			"replay_buffer_max": mcpReplayBuffer,
		},
	})
	writeReplayStatusNotification(c.Writer, streamID, lastEventID, replayStatus, len(replay))
	for _, message := range replay {
		writeSSEJSONWithID(c.Writer, message.ID, message.Event, message.Data)
	}
	if flusher != nil {
		flusher.Flush()
	}

	keepAlive := time.NewTicker(mcpKeepaliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case message, ok := <-stream:
			if !ok {
				return
			}
			writeSSEJSONWithID(c.Writer, message.ID, message.Event, message.Data)
			if flusher != nil {
				flusher.Flush()
			}
		case <-keepAlive.C:
			fmt.Fprint(c.Writer, ": keepalive\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

// HandleMCPOptions 处理 MCP 端点预检请求
func HandleMCPOptions(c *gin.Context) {
	c.Header("Allow", "POST, GET, OPTIONS")
	c.Status(http.StatusNoContent)
}

type mcpPayloadKind int

const (
	payloadInvalid mcpPayloadKind = iota
	payloadNotificationsOnly
	payloadResponsesOnly
	payloadIncludesRequests
)

type mcpEnvelope struct {
	Method *string         `json:"method"`
	ID     json.RawMessage `json:"id"`
}

func parseMCPPayload(body io.Reader) ([]mcp.Request, mcpPayloadKind, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, payloadInvalid, fmt.Errorf("failed to read request body")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, payloadInvalid, fmt.Errorf("empty request body")
	}

	if data[0] == '[' {
		var rawItems []json.RawMessage
		if err := json.Unmarshal(data, &rawItems); err != nil {
			return nil, payloadInvalid, err
		}
		if len(rawItems) == 0 {
			return nil, payloadInvalid, fmt.Errorf("empty batch request")
		}
		requests := make([]mcp.Request, 0, len(rawItems))
		envelopes := make([]mcpEnvelope, 0, len(rawItems))
		for _, raw := range rawItems {
			var req mcp.Request
			if err := json.Unmarshal(raw, &req); err != nil {
				return nil, payloadInvalid, err
			}
			requests = append(requests, req)

			var envelope mcpEnvelope
			if err := json.Unmarshal(raw, &envelope); err != nil {
				return nil, payloadInvalid, err
			}
			envelopes = append(envelopes, envelope)
		}
		return requests, classifyMCPPayload(envelopes), nil
	}

	var req mcp.Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, payloadInvalid, err
	}

	var envelope mcpEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, payloadInvalid, err
	}

	return []mcp.Request{req}, classifyMCPPayload([]mcpEnvelope{envelope}), nil
}

func classifyMCPPayload(items []mcpEnvelope) mcpPayloadKind {
	hasRequest := false
	hasMethod := false

	for _, item := range items {
		if item.Method != nil && strings.TrimSpace(*item.Method) != "" {
			hasMethod = true
			if len(item.ID) > 0 {
				hasRequest = true
			}
		}
	}

	switch {
	case hasRequest:
		return payloadIncludesRequests
	case hasMethod:
		return payloadNotificationsOnly
	default:
		return payloadResponsesOnly
	}
}

func collectMCPResponses(c *gin.Context, server *mcp.Server, requests []mcp.Request) []mcp.Response {
	responses := make([]mcp.Response, 0, len(requests))
	for _, req := range requests {
		if resp := server.HandleRequest(c.Request.Context(), req); resp != nil {
			responses = append(responses, *resp)
		}
	}
	return responses
}

func streamMCPResponses(c *gin.Context, server *mcp.Server, requests []mcp.Request) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	fmt.Fprintf(c.Writer, "retry: %d\n\n", mcpRetryInterval.Milliseconds())
	flusher, _ := c.Writer.(http.Flusher)
	for _, req := range requests {
		resp := server.HandleRequest(c.Request.Context(), req)
		if resp == nil {
			continue
		}
		writeSSEJSON(c.Writer, "message", resp)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func writeSSEJSON(w io.Writer, event string, payload interface{}) {
	writeSSEJSONWithID(w, uuid.NewString(), event, payload)
}

func writeSSEJSONWithID(w io.Writer, id, event string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	if event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", event)
	}
	if strings.TrimSpace(id) != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", id)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func acceptsSSE(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err == nil && strings.EqualFold(mediaType, "text/event-stream") {
			return true
		}
	}
	return false
}

func disableWriteTimeout(c *gin.Context) {
	controller := http.NewResponseController(c.Writer)
	if err := controller.SetWriteDeadline(time.Time{}); err != nil && err != http.ErrNotSupported {
		// 某些 ResponseWriter 实现可能不支持 deadline 控制；这种情况下退化为默认行为。
		return
	}
}

func writeReplayStatusNotification(w io.Writer, streamID, lastEventID string, status mcp.ReplayStatus, replayed int) {
	switch status {
	case mcp.ReplayNotRequested:
		return
	case mcp.ReplayReplayed, mcp.ReplayAtHead:
		writeSSEJSONWithID(w, "", "message", mcp.Notification{
			JSONRPC: "2.0",
			Method:  "notifications/stream/resumed",
			Params: map[string]interface{}{
				"stream_id":     streamID,
				"last_event_id": lastEventID,
				"replayed":      replayed,
				"status":        string(status),
			},
		})
	case mcp.ReplayMissed:
		writeSSEJSONWithID(w, "", "message", mcp.Notification{
			JSONRPC: "2.0",
			Method:  "notifications/stream/replay_unavailable",
			Params: map[string]interface{}{
				"stream_id":     streamID,
				"last_event_id": lastEventID,
				"status":        string(status),
			},
		})
	}
}
