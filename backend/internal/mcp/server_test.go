package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMCPTestServer(t *testing.T) (*Server, *gorm.DB, models.User) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.DatabaseAccess{},
	))

	user := models.User{
		Username: "mcp_user",
		Email:    "mcp@example.com",
		Password: "hashed",
	}
	require.NoError(t, db.Create(&user).Error)

	server := NewServer(NewToolService(db, user.ID), "test")
	return server, db, user
}

func mustRawID(t *testing.T, value string) json.RawMessage {
	t.Helper()
	return json.RawMessage(value)
}

func decodeResult[T any](t *testing.T, response *Response) T {
	t.Helper()
	require.NotNil(t, response)
	require.Nil(t, response.Error)

	data, err := json.Marshal(response.Result)
	require.NoError(t, err)

	var result T
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

func TestServerInitialize(t *testing.T) {
	server, _, _ := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		ID:      mustRawID(t, "1"),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
	})

	result := decodeResult[map[string]interface{}](t, response)
	require.Equal(t, "2024-11-05", result["protocolVersion"])
	require.NotNil(t, result["capabilities"])
}

func TestServerInitializeDefaultsProtocolVersionWhenMissing(t *testing.T) {
	server, _, _ := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		ID:      mustRawID(t, "10"),
		Method:  "initialize",
	})

	result := decodeResult[map[string]interface{}](t, response)
	require.Equal(t, defaultProtocolVersion, result["protocolVersion"])
}

func TestServerToolsList(t *testing.T) {
	server, _, _ := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		ID:      mustRawID(t, "2"),
		Method:  "tools/list",
	})

	var result struct {
		Tools []ToolDefinition `json:"tools"`
	}
	result = decodeResult[struct {
		Tools []ToolDefinition `json:"tools"`
	}](t, response)

	require.NotEmpty(t, result.Tools)
	require.Equal(t, "query_data", result.Tools[0].Name)
}

func TestServerToolsCallCreateDatabase(t *testing.T) {
	server, db, user := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		ID:      mustRawID(t, "3"),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"create_database","arguments":{"name":"MCP DB","description":"created via mcp"}}`),
	})

	var result ToolCallResult
	result = decodeResult[ToolCallResult](t, response)
	require.False(t, result.IsError)

	var count int64
	require.NoError(t, db.Model(&models.Database{}).Where("owner_id = ? AND name = ?", user.ID, "MCP DB").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestServerToolsCallQueryDataExpandsAllowedFields(t *testing.T) {
	server, _, user := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		ID:      mustRawID(t, "4"),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"query_data","arguments":{"query":{"from":"users","select":["*"]}}}`),
	})

	var result ToolCallResult
	result = decodeResult[ToolCallResult](t, response)
	require.False(t, result.IsError)

	payload, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)

	var queryResult struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(payload, &queryResult))
	require.Len(t, queryResult.Data, 1)
	require.Equal(t, user.Username, queryResult.Data[0]["username"])
	_, hasPassword := queryResult.Data[0]["password"]
	require.False(t, hasPassword)
}

func TestToolServiceCreateDatabasePublishesNotification(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.DatabaseAccess{},
	))

	user := models.User{
		Username: "notify_user",
		Email:    "notify@example.com",
		Password: "hashed",
	}
	require.NoError(t, db.Create(&user).Error)

	hub := NewSSEHub()
	_, ch, _, _, cleanup := hub.Register(user.ID, "")
	defer cleanup()

	service := NewToolServiceWithNotifier(db, user.ID, hub)
	result, callErr := service.callCreateDatabase(json.RawMessage(`{"name":"Notify DB"}`))
	require.NoError(t, callErr)
	require.False(t, result.IsError)

	select {
	case message := <-ch:
		notification, ok := message.Data.(Notification)
		require.True(t, ok)
		require.Equal(t, "notifications/databases/changed", notification.Method)

		payload, err := json.Marshal(notification.Params)
		require.NoError(t, err)
		require.Contains(t, string(payload), "\"name\":\"Notify DB\"")
	case <-time.After(time.Second):
		t.Fatal("expected database creation notification")
	}
}

func TestServerReturnsMethodNotFoundForUnknownMethod(t *testing.T) {
	server, _, _ := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		ID:      mustRawID(t, "11"),
		Method:  "unknown/method",
	})

	require.NotNil(t, response)
	require.NotNil(t, response.Error)
	require.Equal(t, -32601, response.Error.Code)
	require.Equal(t, "Method not found", response.Error.Message)
}

func TestServerDoesNotRespondToUnknownNotification(t *testing.T) {
	server, _, _ := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		Method:  "unknown/notification",
	})

	require.Nil(t, response)
}

func TestServerReturnsInvalidParamsWhenToolsCallParamsMalformed(t *testing.T) {
	server, _, _ := setupMCPTestServer(t)

	response := server.HandleRequest(context.Background(), Request{
		JSONRPC: jsonRPCVersion,
		ID:      mustRawID(t, "12"),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":`),
	})

	require.NotNil(t, response)
	require.NotNil(t, response.Error)
	require.Equal(t, -32602, response.Error.Code)
	require.Equal(t, "Invalid tools/call params", response.Error.Message)
}
