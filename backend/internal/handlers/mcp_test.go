package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

type sseEvent struct {
	ID    string
	Event string
	Data  string
}

type sseConnection struct {
	resp   *http.Response
	reader *bufio.Reader
	cancel context.CancelFunc
}

func setupMCPHandlerTest(t *testing.T) (*gin.Engine, models.User) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	ConfigureMCP(MCPOptions{
		SSEKeepaliveInterval: defaultMCPKeepaliveInterval,
		SSERetryInterval:     defaultMCPRetryInterval,
		SSEReplayBuffer:      defaultMCPReplayBuffer,
	})
	t.Cleanup(func() {
		ConfigureMCP(MCPOptions{
			SSEKeepaliveInterval: defaultMCPKeepaliveInterval,
			SSERetryInterval:     defaultMCPRetryInterval,
			SSEReplayBuffer:      defaultMCPReplayBuffer,
		})
	})

	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-mcp-handler")
	dbFile := t.TempDir() + "\\mcp-handler-test.db"
	cfg := config.DatabaseConfig{
		Type: "sqlite",
		URL:  dbFile,
	}
	require.NoError(t, pkgdb.InitDB(cfg))
	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
	})

	require.NoError(t, pkgdb.DB().AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.DatabaseAccess{},
		&models.Table{},
		&models.Field{},
		&models.FieldPermission{},
		&models.ActivityLog{},
		&models.GovernanceTask{},
		&models.GovernanceReview{},
		&models.GovernanceEvidence{},
		&models.GovernanceExternalLink{},
		&models.GovernanceComment{},
		&models.GovernanceOutboxEvent{},
		&models.TokenBlacklist{},
	))

	user := models.User{
		Username: "mcp_http_user",
		Email:    "mcp_http@example.com",
		Password: "hashed",
	}
	require.NoError(t, pkgdb.DB().Create(&user).Error)

	r := gin.New()
	r.OPTIONS("/mcp", HandleMCPOptions)
	mcpRoute := r.Group("/mcp")
	mcpRoute.Use(middleware.MCPOriginGuard(), middleware.Auth())
	mcpRoute.POST("", HandleMCP)
	mcpRoute.GET("", HandleMCPGet)

	protected := r.Group("/api")
	protected.Use(middleware.Auth())
	protected.POST("/databases", CreateDatabase)
	protected.PUT("/databases/:id", UpdateDatabase)
	protected.POST("/tables", CreateTable)
	protected.PUT("/tables/:id", UpdateTable)
	protected.POST("/fields", CreateField)
	protected.PUT("/fields/:id", UpdateField)
	protected.POST("/governance/tasks", CreateGovernanceTask)
	protected.PUT("/governance/tasks/:id", UpdateGovernanceTask)
	protected.POST("/governance/reviews", CreateGovernanceReview)
	protected.POST("/governance/reviews/:id/approve", ApproveGovernanceReview)
	protected.POST("/governance/reviews/:id/reject", RejectGovernanceReview)
	protected.POST("/governance/reviews/:id/apply", ApplyGovernanceReview)

	return r, user
}

func authHeaderForUser(t *testing.T, user models.User) string {
	t.Helper()
	token, err := utils.GenerateToken(user.ID, user.Username, "user")
	require.NoError(t, err)
	return "Bearer " + token
}

func TestHandleMCPCreateDatabase(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_database","arguments":{"name":"HTTP MCP DB"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "2.0", response["jsonrpc"])
	require.NotNil(t, response["result"])

	var count int64
	require.NoError(t, pkgdb.DB().Model(&models.Database{}).Where("owner_id = ? AND name = ?", user.ID, "HTTP MCP DB").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestHandleMCPGetReturnsMethodNotAllowed(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", authHeaderForUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestHandleMCPRejectsMismatchedOrigin(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", bytes.NewReader(body))
	req.Host = "localhost"
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example.com")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandleMCPPostStreamsSSEWhenRequested(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), "retry: 3000")
	require.Contains(t, w.Body.String(), "event: message")
	require.Contains(t, w.Body.String(), "\"jsonrpc\":\"2.0\"")
	require.Contains(t, w.Body.String(), "\"tools\"")
}

func TestHandleMCPGetEstablishesSSEStream(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Accept", "text/event-stream")

	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected GET /mcp SSE handler to stop after context cancellation")
	}

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), ": stream opened")
	require.Contains(t, w.Body.String(), "\"method\":\"notifications/stream/connected\"")
}

func TestHandleMCPGetReceivesServerInitiatedNotification(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getReq := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	getReq.Header.Set("Authorization", authHeaderForUser(t, user))
	getReq.Header.Set("Accept", "text/event-stream; charset=utf-8")

	getW := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(getW, getReq)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_database","arguments":{"name":"MCP SSE DB"}}}`)
	postReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	postReq.Header.Set("Authorization", authHeaderForUser(t, user))
	postReq.Header.Set("Content-Type", "application/json")

	postW := httptest.NewRecorder()
	router.ServeHTTP(postW, postReq)
	require.Equal(t, http.StatusOK, postW.Code)

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected GET /mcp SSE handler to stop after context cancellation")
	}

	require.Contains(t, getW.Body.String(), ": stream opened")
	require.Contains(t, getW.Body.String(), "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, getW.Body.String(), "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, getW.Body.String(), "\"name\":\"MCP SSE DB\"")
}

func TestHandleMCPPostSSEAcceptsMediaTypeParameters(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream; charset=utf-8")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), "retry: 3000")
	require.Contains(t, w.Body.String(), "event: message")
}

func TestHandleMCPRejectsEmptyBatch(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`[]`)))
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "\"code\":-32700")
	require.Contains(t, w.Body.String(), "empty batch request")
}

func TestHandleMCPReturnsAcceptedForNotificationsOnlyPayload(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)))
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Empty(t, w.Body.String())
}

func TestHandleMCPReturnsAcceptedForResponsesOnlyPayload(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)))
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Empty(t, w.Body.String())
}

func TestHandleMCPBatchRequestIgnoresNotificationsWithoutID(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	body := []byte(`[
		{"jsonrpc":"2.0","method":"notifications/initialized"},
		{"jsonrpc":"2.0","id":1,"method":"tools/list"}
	]`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "\"id\":1")
	require.Contains(t, w.Body.String(), "\"tools\"")
	require.NotContains(t, w.Body.String(), "notifications/initialized")
}

func TestHandleMCPGetReplaysEventsFromLastEventID(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	first, _ := mcpHub.PublishNotificationToUser(user.ID, "notifications/databases/changed", map[string]interface{}{
		"database": map[string]interface{}{"name": "db-1"},
	})
	second, _ := mcpHub.PublishNotificationToUser(user.ID, "notifications/databases/changed", map[string]interface{}{
		"database": map[string]interface{}{"name": "db-2"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", first.ID)

	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected GET /mcp SSE handler to stop after context cancellation")
	}

	require.Contains(t, w.Body.String(), "\"method\":\"notifications/stream/resumed\"")
	require.Contains(t, w.Body.String(), "\"last_event_id\":\""+first.ID+"\"")
	require.Contains(t, w.Body.String(), "\"replayed\":1")
	require.Contains(t, w.Body.String(), second.ID)
	require.Contains(t, w.Body.String(), "\"name\":\"db-2\"")
	require.NotContains(t, w.Body.String(), "\"name\":\"db-1\"")
}

func TestHandleMCPGetReportsReplayUnavailableForUnknownLastEventID(t *testing.T) {
	router, user := setupMCPHandlerTest(t)

	_, _ = mcpHub.PublishNotificationToUser(user.ID, "notifications/databases/changed", map[string]interface{}{
		"database": map[string]interface{}{"name": "db-1"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", "missing-event-id")

	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected GET /mcp SSE handler to stop after context cancellation")
	}

	require.Contains(t, w.Body.String(), "\"method\":\"notifications/stream/replay_unavailable\"")
	require.Contains(t, w.Body.String(), "\"last_event_id\":\"missing-event-id\"")
}

func TestHandleMCPGetSendsKeepaliveUsingConfiguredInterval(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	ConfigureMCP(MCPOptions{
		SSEKeepaliveInterval: 10 * time.Millisecond,
		SSERetryInterval:     defaultMCPRetryInterval,
		SSEReplayBuffer:      defaultMCPReplayBuffer,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Accept", "text/event-stream")

	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected GET /mcp SSE handler to stop after context cancellation")
	}

	require.Contains(t, w.Body.String(), ": keepalive")
}

func TestHandleMCPRealHTTPStreamReceivesNotification(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Accept", "text/event-stream")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	reader := bufio.NewReader(resp.Body)

	connected := readNextSSEEvent(t, reader)
	require.Equal(t, "message", connected.Event)
	require.Contains(t, connected.Data, "\"method\":\"notifications/stream/connected\"")

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_database","arguments":{"name":"HTTP Real SSE DB"}}}`)
	postReq, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	require.NoError(t, err)
	postReq.Header.Set("Authorization", authHeaderForUser(t, user))
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := server.Client().Do(postReq)
	require.NoError(t, err)
	defer postResp.Body.Close()
	require.Equal(t, http.StatusOK, postResp.StatusCode)

	event := readNextSSEEvent(t, reader)
	require.Equal(t, "message", event.Event)
	require.NotEmpty(t, event.ID)
	require.Contains(t, event.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, event.Data, "\"name\":\"HTTP Real SSE DB\"")

	cancel()
}

func TestHandleMCPRealHTTPReconnectReplaysMissedEvents(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	connect := func(lastEventID string) (*http.Response, *bufio.Reader, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/mcp", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", authHeaderForUser(t, user))
		req.Header.Set("Accept", "text/event-stream")
		if lastEventID != "" {
			req.Header.Set("Last-Event-ID", lastEventID)
		}

		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return resp, bufio.NewReader(resp.Body), cancel
	}

	createDB := func(name string) {
		body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_database","arguments":{"name":"` + name + `"}}}`)
		req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Authorization", authHeaderForUser(t, user))
		req.Header.Set("Content-Type", "application/json")

		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	resp1, reader1, cancel1 := connect("")
	defer resp1.Body.Close()

	connected1 := readNextSSEEvent(t, reader1)
	require.Contains(t, connected1.Data, "\"method\":\"notifications/stream/connected\"")

	createDB("Replay DB 1")
	firstChange := readNextSSEEvent(t, reader1)
	require.Contains(t, firstChange.Data, "\"name\":\"Replay DB 1\"")
	require.NotEmpty(t, firstChange.ID)

	cancel1()
	_ = resp1.Body.Close()

	createDB("Replay DB 2")

	resp2, reader2, cancel2 := connect(firstChange.ID)
	defer cancel2()
	defer resp2.Body.Close()

	connected2 := readNextSSEEvent(t, reader2)
	require.Contains(t, connected2.Data, "\"method\":\"notifications/stream/connected\"")

	resumed := readNextSSEEvent(t, reader2)
	require.Contains(t, resumed.Data, "\"method\":\"notifications/stream/resumed\"")
	require.Contains(t, resumed.Data, "\"last_event_id\":\""+firstChange.ID+"\"")

	replayed := readNextSSEEvent(t, reader2)
	require.Contains(t, replayed.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, replayed.Data, "\"name\":\"Replay DB 2\"")
}

func TestHandleMCPRealHTTPPostSSEIncludesRetryAndEventID(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeaderForUser(t, user))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")

	reader := bufio.NewReader(resp.Body)
	event := readNextSSEEvent(t, reader)
	require.Equal(t, "message", event.Event)
	require.NotEmpty(t, event.ID)
	require.Contains(t, event.Data, "\"tools\"")
}

func TestHandleMCPRealHTTPMultipleSubscribersReceiveSameNotification(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	conn1 := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	defer closeSSEConnection(conn1)
	conn2 := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	defer closeSSEConnection(conn2)

	connected1 := readNextSSEEvent(t, conn1.reader)
	connected2 := readNextSSEEvent(t, conn2.reader)
	require.Contains(t, connected1.Data, "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, connected2.Data, "\"method\":\"notifications/stream/connected\"")

	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "Concurrent SSE DB")

	event1 := readNextSSEEvent(t, conn1.reader)
	event2 := readNextSSEEvent(t, conn2.reader)
	require.Contains(t, event1.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, event2.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, event1.Data, "\"name\":\"Concurrent SSE DB\"")
	require.Contains(t, event2.Data, "\"name\":\"Concurrent SSE DB\"")
}

func TestHandleMCPRealHTTPReplayUnavailableAfterBufferEviction(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	ConfigureMCP(MCPOptions{
		SSEKeepaliveInterval: defaultMCPKeepaliveInterval,
		SSERetryInterval:     defaultMCPRetryInterval,
		SSEReplayBuffer:      2,
	})
	server := httptest.NewServer(router)
	defer server.Close()

	conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	firstConnected := readNextSSEEvent(t, conn.reader)
	require.Contains(t, firstConnected.Data, "\"method\":\"notifications/stream/connected\"")

	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "Replay Edge DB 1")
	firstEvent := readNextSSEEvent(t, conn.reader)
	require.NotEmpty(t, firstEvent.ID)
	require.Contains(t, firstEvent.Data, "\"name\":\"Replay Edge DB 1\"")
	closeSSEConnection(conn)

	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "Replay Edge DB 2")
	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "Replay Edge DB 3")
	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "Replay Edge DB 4")

	reconnect := openSSEConnection(t, server.URL, authHeaderForUser(t, user), firstEvent.ID)
	defer closeSSEConnection(reconnect)

	connected := readNextSSEEvent(t, reconnect.reader)
	require.Contains(t, connected.Data, "\"method\":\"notifications/stream/connected\"")

	replayUnavailable := readNextSSEEvent(t, reconnect.reader)
	require.Contains(t, replayUnavailable.Data, "\"method\":\"notifications/stream/replay_unavailable\"")
	require.Contains(t, replayUnavailable.Data, "\"last_event_id\":\""+firstEvent.ID+"\"")
}

func TestHandleMCPRealHTTPReplayAtHeadReturnsResumedWithoutReplay(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	connected := readNextSSEEvent(t, conn.reader)
	require.Contains(t, connected.Data, "\"method\":\"notifications/stream/connected\"")

	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "Replay Head DB")
	event := readNextSSEEvent(t, conn.reader)
	require.NotEmpty(t, event.ID)
	require.Contains(t, event.Data, "\"name\":\"Replay Head DB\"")
	closeSSEConnection(conn)

	reconnect := openSSEConnection(t, server.URL, authHeaderForUser(t, user), event.ID)
	defer closeSSEConnection(reconnect)

	connectedAgain := readNextSSEEvent(t, reconnect.reader)
	require.Contains(t, connectedAgain.Data, "\"method\":\"notifications/stream/connected\"")

	resumed := readNextSSEEvent(t, reconnect.reader)
	require.Contains(t, resumed.Data, "\"method\":\"notifications/stream/resumed\"")
	require.Contains(t, resumed.Data, "\"status\":\"at_head\"")
	require.Contains(t, resumed.Data, "\"replayed\":0")
}

func TestHandleMCPRealHTTPStreamEmitsRepeatedKeepalives(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	ConfigureMCP(MCPOptions{
		SSEKeepaliveInterval: 15 * time.Millisecond,
		SSERetryInterval:     defaultMCPRetryInterval,
		SSEReplayBuffer:      defaultMCPReplayBuffer,
	})
	server := httptest.NewServer(router)
	defer server.Close()

	conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	defer closeSSEConnection(conn)

	connected := readNextSSEEvent(t, conn.reader)
	require.Contains(t, connected.Data, "\"method\":\"notifications/stream/connected\"")

	keepalives := readKeepaliveComments(t, conn.reader, 3)
	require.GreaterOrEqual(t, keepalives, 3)
}

func TestHandleMCPRealHTTPUserIsolationKeepsNotificationsScoped(t *testing.T) {
	router, userA := setupMCPHandlerTest(t)
	userB := createTestUser(t, "mcp_user_b")
	server := httptest.NewServer(router)
	defer server.Close()

	connA := openSSEConnection(t, server.URL, authHeaderForUser(t, userA), "")
	defer closeSSEConnection(connA)
	connB := openSSEConnection(t, server.URL, authHeaderForUser(t, userB), "")
	defer closeSSEConnection(connB)

	require.Contains(t, readNextSSEEvent(t, connA.reader).Data, "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, readNextSSEEvent(t, connB.reader).Data, "\"method\":\"notifications/stream/connected\"")

	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, userA), "isolated-db-a-1")
	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, userB), "isolated-db-b-1")

	eventA1 := readNextSSEEvent(t, connA.reader)
	eventB1 := readNextSSEEvent(t, connB.reader)
	require.Contains(t, eventA1.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, eventA1.Data, "\"name\":\"isolated-db-a-1\"")
	require.Contains(t, eventB1.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, eventB1.Data, "\"name\":\"isolated-db-b-1\"")

	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, userA), "isolated-db-a-2")
	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, userB), "isolated-db-b-2")

	eventA2 := readNextSSEEvent(t, connA.reader)
	eventB2 := readNextSSEEvent(t, connB.reader)
	require.Contains(t, eventA2.Data, "\"name\":\"isolated-db-a-2\"")
	require.NotContains(t, eventA2.Data, "\"isolated-db-b-")
	require.Contains(t, eventB2.Data, "\"name\":\"isolated-db-b-2\"")
	require.NotContains(t, eventB2.Data, "\"isolated-db-a-")
}

func TestHandleMCPRealHTTPHighConcurrencySubscribersReceiveSequentialNotifications(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	const subscriberCount = 10
	connections := make([]*sseConnection, 0, subscriberCount)
	for i := 0; i < subscriberCount; i++ {
		conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
		connections = append(connections, conn)
	}
	defer func() {
		for _, conn := range connections {
			closeSSEConnection(conn)
		}
	}()

	for _, conn := range connections {
		require.Contains(t, readNextSSEEvent(t, conn.reader).Data, "\"method\":\"notifications/stream/connected\"")
	}

	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "burst-db-1")
	createDatabaseThroughMCP(t, server.URL, authHeaderForUser(t, user), "burst-db-2")

	var wg sync.WaitGroup
	errCh := make(chan error, subscriberCount)
	for idx, conn := range connections {
		wg.Add(1)
		go func(i int, c *sseConnection) {
			defer wg.Done()

			first := readNextSSEEvent(t, c.reader)
			second := readNextSSEEvent(t, c.reader)

			if !strings.Contains(first.Data, "\"name\":\"burst-db-1\"") {
				errCh <- fmt.Errorf("subscriber %d first event mismatch: %s", i, first.Data)
				return
			}
			if !strings.Contains(second.Data, "\"name\":\"burst-db-2\"") {
				errCh <- fmt.Errorf("subscriber %d second event mismatch: %s", i, second.Data)
				return
			}
		}(idx, conn)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}

func TestHandleMCPRealHTTPRESTTableAndFieldChangesEmitNotifications(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	database := createOwnedDatabaseForUser(t, user.ID, "rest-notify-db")
	conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	defer closeSSEConnection(conn)

	require.Contains(t, readNextSSEEvent(t, conn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	tableResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/tables", authHeaderForUser(t, user), map[string]interface{}{
		"database_id": database.ID,
		"name":        "orders",
		"description": "order table",
	})
	require.Equal(t, http.StatusOK, tableResp.status)
	tableID := responseDataString(t, tableResp.body, "id")

	tableEvent := readNextSSEEvent(t, conn.reader)
	require.Contains(t, tableEvent.Data, "\"method\":\"notifications/tables/changed\"")
	require.Contains(t, tableEvent.Data, "\"action\":\"created\"")
	require.Contains(t, tableEvent.Data, "\"name\":\"orders\"")

	fieldResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/fields", authHeaderForUser(t, user), map[string]interface{}{
		"table_id": tableID,
		"name":     "status",
		"type":     "select",
		"required": false,
		"config": map[string]interface{}{
			"options": []string{"draft", "published"},
		},
	})
	require.Equal(t, http.StatusOK, fieldResp.status)
	fieldID := responseDataString(t, fieldResp.body, "id")

	fieldCreatedEvent := readNextSSEEvent(t, conn.reader)
	require.Contains(t, fieldCreatedEvent.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, fieldCreatedEvent.Data, "\"action\":\"created\"")
	require.Contains(t, fieldCreatedEvent.Data, "\"name\":\"status\"")

	updateFieldResp := performJSONRequest(t, http.MethodPut, server.URL+"/api/fields/"+fieldID, authHeaderForUser(t, user), map[string]interface{}{
		"name":     "status_v2",
		"type":     "multiselect",
		"required": true,
		"config": map[string]interface{}{
			"options": []string{"draft", "published", "archived"},
		},
	})
	require.Equal(t, http.StatusOK, updateFieldResp.status)

	fieldUpdatedEvent := readNextSSEEvent(t, conn.reader)
	require.Contains(t, fieldUpdatedEvent.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, fieldUpdatedEvent.Data, "\"action\":\"updated\"")
	require.Contains(t, fieldUpdatedEvent.Data, "\"name\":\"status_v2\"")
	require.Contains(t, fieldUpdatedEvent.Data, "\"required\":true")
	require.Contains(t, fieldUpdatedEvent.Data, "\"type\":\"multiselect\"")
}

func TestHandleMCPRealHTTPGovernanceWorkflowBroadcastsToParticipants(t *testing.T) {
	router, creator := setupMCPHandlerTest(t)
	reviewer := createTestUser(t, "mcp_reviewer")
	outsider := createTestUser(t, "mcp_outsider")

	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/integration/v1/recommendations/classifications", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer applyServer.Close()

	t.Setenv("INTEGRATION_BASE_URLS", "fakecmdb="+applyServer.URL)
	t.Setenv("OUTBOUND_INTEGRATION_TIMEOUT_SEC", "1")

	server := httptest.NewServer(router)
	defer server.Close()

	creatorConn := openSSEConnection(t, server.URL, authHeaderForUser(t, creator), "")
	defer closeSSEConnection(creatorConn)
	reviewerConn := openSSEConnection(t, server.URL, authHeaderForUser(t, reviewer), "")
	defer closeSSEConnection(reviewerConn)
	outsiderConn := openSSEConnection(t, server.URL, authHeaderForUser(t, outsider), "")
	defer closeSSEConnection(outsiderConn)

	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, readNextSSEEvent(t, reviewerConn.reader).Data, "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, readNextSSEEvent(t, outsiderConn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	taskResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/tasks", authHeaderForUser(t, creator), map[string]interface{}{
		"title":         "Review external schema",
		"description":   "check classification sync",
		"task_type":     "classification_review",
		"priority":      "high",
		"source_system": "fakecmdb",
		"resource_type": "table",
		"resource_id":   "ext_table_001",
		"assignee_id":   reviewer.ID,
	})
	require.Equal(t, http.StatusOK, taskResp.status)
	taskID := responseDataString(t, taskResp.body, "id")

	creatorTaskCreated := readNextSSEEvent(t, creatorConn.reader)
	reviewerTaskCreated := readNextSSEEvent(t, reviewerConn.reader)
	require.Contains(t, creatorTaskCreated.Data, "\"method\":\"notifications/governance/tasks/changed\"")
	require.Contains(t, creatorTaskCreated.Data, "\"action\":\"created\"")
	require.Contains(t, creatorTaskCreated.Data, "\"id\":\""+taskID+"\"")
	require.Contains(t, reviewerTaskCreated.Data, "\"method\":\"notifications/governance/tasks/changed\"")
	require.Contains(t, reviewerTaskCreated.Data, "\"id\":\""+taskID+"\"")

	reviewResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/reviews", authHeaderForUser(t, creator), map[string]interface{}{
		"task_id":          taskID,
		"review_type":      "classification",
		"reviewer_id":      reviewer.ID,
		"proposal_source":  "assistant",
		"proposal_payload": `{"classification":"pii"}`,
	})
	require.Equal(t, http.StatusOK, reviewResp.status)
	reviewID := responseDataString(t, reviewResp.body, "id")

	creatorReviewCreated := readNextSSEEvent(t, creatorConn.reader)
	creatorTaskInReview := readNextSSEEvent(t, creatorConn.reader)
	reviewerReviewCreated := readNextSSEEvent(t, reviewerConn.reader)
	reviewerTaskInReview := readNextSSEEvent(t, reviewerConn.reader)
	require.Contains(t, creatorReviewCreated.Data, "\"method\":\"notifications/governance/reviews/changed\"")
	require.Contains(t, creatorReviewCreated.Data, "\"action\":\"created\"")
	require.Contains(t, creatorReviewCreated.Data, "\"id\":\""+reviewID+"\"")
	require.Contains(t, creatorTaskInReview.Data, "\"method\":\"notifications/governance/tasks/changed\"")
	require.Contains(t, creatorTaskInReview.Data, "\"action\":\"entered_review\"")
	require.Contains(t, creatorTaskInReview.Data, "\"status\":\"in_review\"")
	require.Contains(t, reviewerReviewCreated.Data, "\"method\":\"notifications/governance/reviews/changed\"")
	require.Contains(t, reviewerTaskInReview.Data, "\"status\":\"in_review\"")

	approveResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/reviews/"+reviewID+"/approve", authHeaderForUser(t, reviewer), map[string]interface{}{
		"decision_payload": `{"approved":true,"reason":"looks good"}`,
	})
	require.Equal(t, http.StatusOK, approveResp.status)

	creatorReviewApproved := readNextSSEEvent(t, creatorConn.reader)
	creatorTaskDone := readNextSSEEvent(t, creatorConn.reader)
	reviewerReviewApproved := readNextSSEEvent(t, reviewerConn.reader)
	reviewerTaskDone := readNextSSEEvent(t, reviewerConn.reader)
	require.Contains(t, creatorReviewApproved.Data, "\"method\":\"notifications/governance/reviews/changed\"")
	require.Contains(t, creatorReviewApproved.Data, "\"action\":\"approved\"")
	require.Contains(t, creatorReviewApproved.Data, "\"status\":\"approved\"")
	require.Contains(t, creatorReviewApproved.Data, "\"apply_status\":\"succeeded\"")
	require.Contains(t, creatorTaskDone.Data, "\"method\":\"notifications/governance/tasks/changed\"")
	require.Contains(t, creatorTaskDone.Data, "\"status\":\"done\"")
	require.Contains(t, reviewerReviewApproved.Data, "\"apply_status\":\"succeeded\"")
	require.Contains(t, reviewerTaskDone.Data, "\"status\":\"done\"")

	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, outsider), "outsider-only-db")
	outsiderEvent := readNextSSEEvent(t, outsiderConn.reader)
	require.Contains(t, outsiderEvent.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, outsiderEvent.Data, "\"name\":\"outsider-only-db\"")
	require.NotContains(t, outsiderEvent.Data, "\"notifications/governance/")
}

func TestHandleMCPRealHTTPLastEventIDDoesNotCrossUserReplayBoundary(t *testing.T) {
	router, userA := setupMCPHandlerTest(t)
	userB := createTestUser(t, "mcp_replay_user_b")
	server := httptest.NewServer(router)
	defer server.Close()

	connA := openSSEConnection(t, server.URL, authHeaderForUser(t, userA), "")
	defer closeSSEConnection(connA)
	require.Contains(t, readNextSSEEvent(t, connA.reader).Data, "\"method\":\"notifications/stream/connected\"")

	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, userA), "replay-boundary-db")
	userAEvent := readNextSSEEvent(t, connA.reader)
	require.NotEmpty(t, userAEvent.ID)
	require.Contains(t, userAEvent.Data, "\"name\":\"replay-boundary-db\"")

	connB := openSSEConnection(t, server.URL, authHeaderForUser(t, userB), userAEvent.ID)
	defer closeSSEConnection(connB)

	connected := readNextSSEEvent(t, connB.reader)
	replayUnavailable := readNextSSEEvent(t, connB.reader)
	require.Contains(t, connected.Data, "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, replayUnavailable.Data, "\"method\":\"notifications/stream/replay_unavailable\"")
	require.Contains(t, replayUnavailable.Data, "\"last_event_id\":\""+userAEvent.ID+"\"")
	require.NotContains(t, replayUnavailable.Data, "replay-boundary-db")
}

func TestHandleMCPRealHTTPSharedDatabaseAdminUpdateNotifiesOwnerAndOperator(t *testing.T) {
	router, owner := setupMCPHandlerTest(t)
	admin := createTestUser(t, "mcp_db_admin")
	server := httptest.NewServer(router)
	defer server.Close()

	database := createOwnedDatabaseForUser(t, owner.ID, "shared-db-before")
	grantDatabaseAccess(t, database.ID, admin.ID, "admin")

	ownerConn := openSSEConnection(t, server.URL, authHeaderForUser(t, owner), "")
	defer closeSSEConnection(ownerConn)
	adminConn := openSSEConnection(t, server.URL, authHeaderForUser(t, admin), "")
	defer closeSSEConnection(adminConn)

	require.Contains(t, readNextSSEEvent(t, ownerConn.reader).Data, "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, readNextSSEEvent(t, adminConn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	updateResp := performJSONRequest(t, http.MethodPut, server.URL+"/api/databases/"+database.ID, authHeaderForUser(t, admin), map[string]interface{}{
		"name":        "shared-db-after",
		"description": "updated by admin",
		"is_public":   true,
	})
	require.Equal(t, http.StatusOK, updateResp.status)

	ownerEvent := readNextSSEEvent(t, ownerConn.reader)
	adminEvent := readNextSSEEvent(t, adminConn.reader)
	require.Contains(t, ownerEvent.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, ownerEvent.Data, "\"action\":\"updated\"")
	require.Contains(t, ownerEvent.Data, "\"name\":\"shared-db-after\"")
	require.Contains(t, adminEvent.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, adminEvent.Data, "\"action\":\"updated\"")
	require.Contains(t, adminEvent.Data, "\"name\":\"shared-db-after\"")
}

func TestHandleMCPRealHTTPGovernanceRejectBroadcastsBlockedState(t *testing.T) {
	router, creator := setupMCPHandlerTest(t)
	reviewer := createTestUser(t, "mcp_reject_reviewer")
	server := httptest.NewServer(router)
	defer server.Close()

	creatorConn := openSSEConnection(t, server.URL, authHeaderForUser(t, creator), "")
	defer closeSSEConnection(creatorConn)
	reviewerConn := openSSEConnection(t, server.URL, authHeaderForUser(t, reviewer), "")
	defer closeSSEConnection(reviewerConn)

	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"method\":\"notifications/stream/connected\"")
	require.Contains(t, readNextSSEEvent(t, reviewerConn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	taskResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/tasks", authHeaderForUser(t, creator), map[string]interface{}{
		"title":         "Reject path task",
		"description":   "exercise reject branch",
		"task_type":     "classification_review",
		"priority":      "medium",
		"source_system": "fakecmdb",
		"resource_type": "table",
		"resource_id":   "reject_table_001",
		"assignee_id":   reviewer.ID,
	})
	require.Equal(t, http.StatusOK, taskResp.status)
	taskID := responseDataString(t, taskResp.body, "id")

	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"id\":\""+taskID+"\"")
	require.Contains(t, readNextSSEEvent(t, reviewerConn.reader).Data, "\"id\":\""+taskID+"\"")

	reviewResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/reviews", authHeaderForUser(t, creator), map[string]interface{}{
		"task_id":          taskID,
		"review_type":      "classification",
		"reviewer_id":      reviewer.ID,
		"proposal_source":  "assistant",
		"proposal_payload": `{"classification":"restricted"}`,
	})
	require.Equal(t, http.StatusOK, reviewResp.status)
	reviewID := responseDataString(t, reviewResp.body, "id")

	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"id\":\""+reviewID+"\"")
	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"status\":\"in_review\"")
	require.Contains(t, readNextSSEEvent(t, reviewerConn.reader).Data, "\"id\":\""+reviewID+"\"")
	require.Contains(t, readNextSSEEvent(t, reviewerConn.reader).Data, "\"status\":\"in_review\"")

	rejectResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/reviews/"+reviewID+"/reject", authHeaderForUser(t, reviewer), map[string]interface{}{
		"decision_payload": `{"approved":false,"reason":"insufficient evidence"}`,
	})
	require.Equal(t, http.StatusOK, rejectResp.status)

	creatorReviewRejected := readNextSSEEvent(t, creatorConn.reader)
	creatorTaskBlocked := readNextSSEEvent(t, creatorConn.reader)
	reviewerReviewRejected := readNextSSEEvent(t, reviewerConn.reader)
	reviewerTaskBlocked := readNextSSEEvent(t, reviewerConn.reader)
	require.Contains(t, creatorReviewRejected.Data, "\"method\":\"notifications/governance/reviews/changed\"")
	require.Contains(t, creatorReviewRejected.Data, "\"action\":\"rejected\"")
	require.Contains(t, creatorReviewRejected.Data, "\"status\":\"rejected\"")
	require.Contains(t, creatorReviewRejected.Data, "\"apply_status\":\"not_requested\"")
	require.Contains(t, creatorTaskBlocked.Data, "\"method\":\"notifications/governance/tasks/changed\"")
	require.Contains(t, creatorTaskBlocked.Data, "\"status\":\"blocked\"")
	require.Contains(t, reviewerReviewRejected.Data, "\"action\":\"rejected\"")
	require.Contains(t, reviewerReviewRejected.Data, "\"status\":\"rejected\"")
	require.Contains(t, reviewerTaskBlocked.Data, "\"status\":\"blocked\"")
}

func TestHandleMCPRealHTTPReplayPreservesMixedBusinessEventOrder(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	database := createOwnedDatabaseForUser(t, user.ID, "mixed-replay-db")
	conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	require.Contains(t, readNextSSEEvent(t, conn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	tableResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/tables", authHeaderForUser(t, user), map[string]interface{}{
		"database_id": database.ID,
		"name":        "mixed_orders",
		"description": "mixed replay table",
	})
	require.Equal(t, http.StatusOK, tableResp.status)
	tableID := responseDataString(t, tableResp.body, "id")
	tableEvent := readNextSSEEvent(t, conn.reader)
	require.Contains(t, tableEvent.Data, "\"method\":\"notifications/tables/changed\"")
	require.NotEmpty(t, tableEvent.ID)

	fieldResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/fields", authHeaderForUser(t, user), map[string]interface{}{
		"table_id": tableID,
		"name":     "stage",
		"type":     "select",
		"config": map[string]interface{}{
			"options": []string{"a", "b"},
		},
	})
	require.Equal(t, http.StatusOK, fieldResp.status)
	fieldID := responseDataString(t, fieldResp.body, "id")
	fieldCreatedEvent := readNextSSEEvent(t, conn.reader)
	require.Contains(t, fieldCreatedEvent.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, fieldCreatedEvent.Data, "\"action\":\"created\"")

	updateResp := performJSONRequest(t, http.MethodPut, server.URL+"/api/fields/"+fieldID, authHeaderForUser(t, user), map[string]interface{}{
		"name":     "stage_v2",
		"type":     "multiselect",
		"required": true,
		"config": map[string]interface{}{
			"options": []string{"a", "b", "c"},
		},
	})
	require.Equal(t, http.StatusOK, updateResp.status)
	fieldUpdatedEvent := readNextSSEEvent(t, conn.reader)
	require.Contains(t, fieldUpdatedEvent.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, fieldUpdatedEvent.Data, "\"action\":\"updated\"")

	closeSSEConnection(conn)

	reconnect := openSSEConnection(t, server.URL, authHeaderForUser(t, user), tableEvent.ID)
	defer closeSSEConnection(reconnect)

	require.Contains(t, readNextSSEEvent(t, reconnect.reader).Data, "\"method\":\"notifications/stream/connected\"")
	resumed := readNextSSEEvent(t, reconnect.reader)
	require.Contains(t, resumed.Data, "\"method\":\"notifications/stream/resumed\"")
	require.Contains(t, resumed.Data, "\"replayed\":2")

	replayedCreated := readNextSSEEvent(t, reconnect.reader)
	replayedUpdated := readNextSSEEvent(t, reconnect.reader)
	require.Contains(t, replayedCreated.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, replayedCreated.Data, "\"action\":\"created\"")
	require.Contains(t, replayedCreated.Data, "\"name\":\"stage\"")
	require.Contains(t, replayedUpdated.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, replayedUpdated.Data, "\"action\":\"updated\"")
	require.Contains(t, replayedUpdated.Data, "\"name\":\"stage_v2\"")
}

func TestHandleMCPRealHTTPGovernanceAudienceDeduplicatesOverlappingRoles(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	server := httptest.NewServer(router)
	defer server.Close()

	conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	defer closeSSEConnection(conn)
	require.Contains(t, readNextSSEEvent(t, conn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	taskResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/tasks", authHeaderForUser(t, user), map[string]interface{}{
		"title":         "Self handled task",
		"description":   "same user owns all roles",
		"task_type":     "classification_review",
		"priority":      "medium",
		"source_system": "manual",
		"resource_type": "table",
		"resource_id":   "self_task_001",
		"assignee_id":   user.ID,
	})
	require.Equal(t, http.StatusOK, taskResp.status)
	taskID := responseDataString(t, taskResp.body, "id")

	taskCreated := readNextSSEEvent(t, conn.reader)
	require.Contains(t, taskCreated.Data, "\"method\":\"notifications/governance/tasks/changed\"")
	require.Contains(t, taskCreated.Data, "\"id\":\""+taskID+"\"")

	reviewResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/reviews", authHeaderForUser(t, user), map[string]interface{}{
		"task_id":          taskID,
		"review_type":      "generic",
		"reviewer_id":      user.ID,
		"proposal_source":  "manual",
		"proposal_payload": `{"note":"self review"}`,
	})
	require.Equal(t, http.StatusOK, reviewResp.status)

	reviewCreated := readNextSSEEvent(t, conn.reader)
	taskInReview := readNextSSEEvent(t, conn.reader)
	require.Contains(t, reviewCreated.Data, "\"method\":\"notifications/governance/reviews/changed\"")
	require.Contains(t, reviewCreated.Data, "\"action\":\"created\"")
	require.Contains(t, taskInReview.Data, "\"method\":\"notifications/governance/tasks/changed\"")
	require.Contains(t, taskInReview.Data, "\"action\":\"entered_review\"")

	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, user), "after-dedup-db")
	nextEvent := readNextSSEEvent(t, conn.reader)
	require.Contains(t, nextEvent.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, nextEvent.Data, "\"name\":\"after-dedup-db\"")
}

func TestHandleMCPRealHTTPApproveFailureDoesNotEmitFalseSuccessNotification(t *testing.T) {
	router, creator := setupMCPHandlerTest(t)
	reviewer := createTestUser(t, "mcp_failure_reviewer")
	server := httptest.NewServer(router)
	defer server.Close()

	creatorConn := openSSEConnection(t, server.URL, authHeaderForUser(t, creator), "")
	defer closeSSEConnection(creatorConn)
	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	taskResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/tasks", authHeaderForUser(t, creator), map[string]interface{}{
		"title":         "Apply failure task",
		"description":   "approve should fail without outbound base url",
		"task_type":     "classification_review",
		"priority":      "high",
		"source_system": "fakecmdb",
		"resource_type": "table",
		"resource_id":   "apply_failure_001",
		"assignee_id":   reviewer.ID,
	})
	require.Equal(t, http.StatusOK, taskResp.status)
	taskID := responseDataString(t, taskResp.body, "id")
	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"id\":\""+taskID+"\"")

	reviewResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/reviews", authHeaderForUser(t, creator), map[string]interface{}{
		"task_id":          taskID,
		"review_type":      "classification",
		"reviewer_id":      reviewer.ID,
		"proposal_source":  "assistant",
		"proposal_payload": `{"classification":"pii"}`,
	})
	require.Equal(t, http.StatusOK, reviewResp.status)
	reviewID := responseDataString(t, reviewResp.body, "id")
	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"id\":\""+reviewID+"\"")
	require.Contains(t, readNextSSEEvent(t, creatorConn.reader).Data, "\"status\":\"in_review\"")

	approveResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/governance/reviews/"+reviewID+"/approve", authHeaderForUser(t, reviewer), map[string]interface{}{
		"decision_payload": `{"approved":true,"reason":"should fail outbound"}`,
	})
	require.Equal(t, http.StatusBadRequest, approveResp.status)
	require.Contains(t, string(approveResp.raw), "未配置 fakecmdb 的回写地址")

	var review models.GovernanceReview
	require.NoError(t, pkgdb.DB().Where("id = ?", reviewID).First(&review).Error)
	require.Equal(t, "approved", review.Status)
	require.Equal(t, "failed", review.ApplyStatus)
	require.Contains(t, review.ApplyError, "未配置 fakecmdb 的回写地址")

	var task models.GovernanceTask
	require.NoError(t, pkgdb.DB().Where("id = ?", taskID).First(&task).Error)
	require.Equal(t, "open", task.Status)

	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, creator), "after-approve-failure-db")
	nextEvent := readNextSSEEvent(t, creatorConn.reader)
	require.Contains(t, nextEvent.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, nextEvent.Data, "\"name\":\"after-approve-failure-db\"")
	require.NotContains(t, nextEvent.Data, "\"notifications/governance/reviews/changed\"")
}

func TestHandleMCPRealHTTPMixedReplayBufferEvictsOldestBoundary(t *testing.T) {
	router, user := setupMCPHandlerTest(t)
	ConfigureMCP(MCPOptions{
		SSEKeepaliveInterval: defaultMCPKeepaliveInterval,
		SSERetryInterval:     defaultMCPRetryInterval,
		SSEReplayBuffer:      3,
	})
	server := httptest.NewServer(router)
	defer server.Close()

	database := createOwnedDatabaseForUser(t, user.ID, "mixed-evict-db")
	conn := openSSEConnection(t, server.URL, authHeaderForUser(t, user), "")
	require.Contains(t, readNextSSEEvent(t, conn.reader).Data, "\"method\":\"notifications/stream/connected\"")

	tableResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/tables", authHeaderForUser(t, user), map[string]interface{}{
		"database_id": database.ID,
		"name":        "evict_orders",
		"description": "mixed eviction table",
	})
	require.Equal(t, http.StatusOK, tableResp.status)
	tableID := responseDataString(t, tableResp.body, "id")
	event1 := readNextSSEEvent(t, conn.reader)
	require.Contains(t, event1.Data, "\"method\":\"notifications/tables/changed\"")
	require.NotEmpty(t, event1.ID)

	fieldResp := performJSONRequest(t, http.MethodPost, server.URL+"/api/fields", authHeaderForUser(t, user), map[string]interface{}{
		"table_id": tableID,
		"name":     "evict_stage",
		"type":     "select",
		"config": map[string]interface{}{
			"options": []string{"x", "y"},
		},
	})
	require.Equal(t, http.StatusOK, fieldResp.status)
	fieldID := responseDataString(t, fieldResp.body, "id")
	event2 := readNextSSEEvent(t, conn.reader)
	require.Contains(t, event2.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, event2.Data, "\"action\":\"created\"")

	updateResp := performJSONRequest(t, http.MethodPut, server.URL+"/api/fields/"+fieldID, authHeaderForUser(t, user), map[string]interface{}{
		"name":     "evict_stage_v2",
		"type":     "multiselect",
		"required": true,
		"config": map[string]interface{}{
			"options": []string{"x", "y", "z"},
		},
	})
	require.Equal(t, http.StatusOK, updateResp.status)
	event3 := readNextSSEEvent(t, conn.reader)
	require.Contains(t, event3.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, event3.Data, "\"action\":\"updated\"")

	createDatabaseThroughAPI(t, server.URL, authHeaderForUser(t, user), "mixed-evict-db-2")
	event4 := readNextSSEEvent(t, conn.reader)
	require.Contains(t, event4.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, event4.Data, "\"name\":\"mixed-evict-db-2\"")

	closeSSEConnection(conn)

	oldestReconnect := openSSEConnection(t, server.URL, authHeaderForUser(t, user), event1.ID)
	defer closeSSEConnection(oldestReconnect)
	require.Contains(t, readNextSSEEvent(t, oldestReconnect.reader).Data, "\"method\":\"notifications/stream/connected\"")
	replayUnavailable := readNextSSEEvent(t, oldestReconnect.reader)
	require.Contains(t, replayUnavailable.Data, "\"method\":\"notifications/stream/replay_unavailable\"")
	require.Contains(t, replayUnavailable.Data, "\"last_event_id\":\""+event1.ID+"\"")

	boundaryReconnect := openSSEConnection(t, server.URL, authHeaderForUser(t, user), event2.ID)
	defer closeSSEConnection(boundaryReconnect)
	require.Contains(t, readNextSSEEvent(t, boundaryReconnect.reader).Data, "\"method\":\"notifications/stream/connected\"")
	resumed := readNextSSEEvent(t, boundaryReconnect.reader)
	require.Contains(t, resumed.Data, "\"method\":\"notifications/stream/resumed\"")
	require.Contains(t, resumed.Data, "\"replayed\":2")

	replayed3 := readNextSSEEvent(t, boundaryReconnect.reader)
	replayed4 := readNextSSEEvent(t, boundaryReconnect.reader)
	require.Contains(t, replayed3.Data, "\"method\":\"notifications/fields/changed\"")
	require.Contains(t, replayed3.Data, "\"action\":\"updated\"")
	require.Contains(t, replayed3.Data, "\"name\":\"evict_stage_v2\"")
	require.Contains(t, replayed4.Data, "\"method\":\"notifications/databases/changed\"")
	require.Contains(t, replayed4.Data, "\"name\":\"mixed-evict-db-2\"")
}

func openSSEConnection(t *testing.T, baseURL, authHeader, lastEventID string) *sseConnection {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "text/event-stream")
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return &sseConnection{
		resp:   resp,
		reader: bufio.NewReader(resp.Body),
		cancel: cancel,
	}
}

func closeSSEConnection(conn *sseConnection) {
	if conn == nil {
		return
	}
	if conn.cancel != nil {
		conn.cancel()
	}
	if conn.resp != nil && conn.resp.Body != nil {
		_ = conn.resp.Body.Close()
	}
}

func createDatabaseThroughMCP(t *testing.T, baseURL, authHeader, name string) {
	t.Helper()

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_database","arguments":{"name":"` + name + `"}}}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/mcp", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func createDatabaseThroughAPI(t *testing.T, baseURL, authHeader, name string) {
	t.Helper()

	resp := performJSONRequest(t, http.MethodPost, baseURL+"/api/databases", authHeader, map[string]interface{}{
		"name": name,
	})
	require.Equal(t, http.StatusOK, resp.status)
}

type jsonResponse struct {
	status int
	body   map[string]interface{}
	raw    []byte
}

func performJSONRequest(t *testing.T, method, url, authHeader string, body interface{}) jsonResponse {
	t.Helper()

	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		payload = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, payload)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	decoded := map[string]interface{}{}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &decoded))
	}

	return jsonResponse{
		status: resp.StatusCode,
		body:   decoded,
		raw:    raw,
	}
}

func responseDataString(t *testing.T, body map[string]interface{}, key string) string {
	t.Helper()

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "response data missing: %v", body)
	value, ok := data[key].(string)
	require.True(t, ok, "response data key %s missing: %v", key, body)
	return value
}

func createTestUser(t *testing.T, prefix string) models.User {
	t.Helper()

	user := models.User{
		Username: prefix + "_" + fmt.Sprint(time.Now().UnixNano()),
		Email:    prefix + "_" + fmt.Sprint(time.Now().UnixNano()) + "@example.com",
		Password: "hashed",
	}
	require.NoError(t, pkgdb.DB().Create(&user).Error)
	return user
}

func createOwnedDatabaseForUser(t *testing.T, userID, name string) models.Database {
	t.Helper()

	database := models.Database{
		Name:       name,
		OwnerID:    userID,
		IsPersonal: true,
	}
	require.NoError(t, pkgdb.DB().Create(&database).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     userID,
		DatabaseID: database.ID,
		Role:       "owner",
	}).Error)
	return database
}

func grantDatabaseAccess(t *testing.T, databaseID, userID, role string) {
	t.Helper()

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     userID,
		DatabaseID: databaseID,
		Role:       role,
	}).Error)
}

func readNextSSEEvent(t *testing.T, reader *bufio.Reader) sseEvent {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	event := sseEvent{}

	for {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for SSE event")
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}
			t.Fatalf("failed to read SSE stream: %v", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if event.Event != "" || event.Data != "" || event.ID != "" {
				return event
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "retry:") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLine := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if event.Data == "" {
				event.Data = dataLine
			} else {
				event.Data += "\n" + dataLine
			}
		}
	}
}

func readKeepaliveComments(t *testing.T, reader *bufio.Reader, target int) int {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	count := 0

	for count < target {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d keepalive comments, got %d", target, count)
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}
			t.Fatalf("failed to read SSE stream: %v", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == ": keepalive" {
			count++
		}
	}

	return count
}
