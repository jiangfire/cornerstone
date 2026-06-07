package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
)

func setupServerTest(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()
	db := testutil.SetupTestDBWithTokens(t, "test_user")
	pkgdb.SetDB(db)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	hub := NewSSEHub()
	ts := NewToolServiceWithNotifier(db, "test_user", hub)
	srv := NewServer(ts, "v0.1.0")
	return srv, db
}

func TestNewServer_DefaultVersion(t *testing.T) {
	srv := NewServer(nil, "")
	assert.Equal(t, "dev", srv.version)
}

func TestNewServer_CustomVersion(t *testing.T) {
	srv := NewServer(nil, "1.2.3")
	assert.Equal(t, "1.2.3", srv.version)
}

func TestServer_HandleRequest_Initialize(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-03-26"}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	assert.Equal(t, "2.0", resp.JSONRPC)

	result := resp.Result.(map[string]interface{})
	assert.Equal(t, "2025-03-26", result["protocolVersion"])

	capabilities := result["capabilities"].(map[string]interface{})
	assert.Contains(t, capabilities, "tools")

	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "cornerstone-mcp", serverInfo["name"])
	assert.Equal(t, "v0.1.0", serverInfo["version"])
}

func TestServer_HandleRequest_Initialize_DefaultProtocolVersion(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	})

	require.NotNil(t, resp)
	result := resp.Result.(map[string]interface{})
	assert.Equal(t, "2025-03-26", result["protocolVersion"])
}

func TestServer_HandleRequest_Initialize_NoParams(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	})

	require.NotNil(t, resp)
	result := resp.Result.(map[string]interface{})
	assert.Equal(t, "2025-03-26", result["protocolVersion"])
}

func TestServer_HandleRequest_Notification(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		Method: "notifications/initialized",
	})

	assert.Nil(t, resp)
}

func TestServer_HandleRequest_Ping(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`42`),
		Method:  "ping",
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
}

func TestServer_HandleRequest_ToolsList(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]ToolDefinition)
	assert.GreaterOrEqual(t, len(tools), 10)
}

func TestServer_HandleRequest_ToolsCall_QueryData(t *testing.T) {
	srv, db := setupServerTest(t)

	db.Create(&models.Database{Name: "QueryDB"})

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "query_data",
			"arguments": {"query": {"from": "databases"}}
		}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(*ToolCallResult)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "succeeded")
}

func TestServer_HandleRequest_ToolsCall_QueryData_Error(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "query_data",
			"arguments": {"query": {"from": "nonexistent_table_xyz"}}
		}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(*ToolCallResult)
	assert.True(t, result.IsError)
}

func TestServer_HandleRequest_ToolsCall_GetTableSchema(t *testing.T) {
	srv, db := setupServerTest(t)

	require.NoError(t, db.Create(&models.Database{Name: "TestDB"}).Error)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "get_table_schema",
			"arguments": {"query_table_name": "databases"}
		}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(*ToolCallResult)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "databases")
}

func TestServer_HandleRequest_ToolsCall_GetTableSchema_Disallowed(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "get_table_schema",
			"arguments": {"query_table_name": "nonexistent_table_xyz"}
		}`),
	})

	require.NotNil(t, resp)
	result := resp.Result.(*ToolCallResult)
	assert.True(t, result.IsError)
}

func TestServer_HandleRequest_ToolsCall_CreateField(t *testing.T) {
	srv, db := setupServerTest(t)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "items"}
	require.NoError(t, db.Create(table).Error)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "create_field",
			"arguments": {"table_id": "` + table.ID + `", "name": "title", "type": "string"}
		}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(*ToolCallResult)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "title")
}

func TestServer_HandleRequest_ToolsCall_CreateField_Error(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "create_field",
			"arguments": {"table_id": "nonexistent", "name": "title", "type": "string"}
		}`),
	})

	require.NotNil(t, resp)
	result := resp.Result.(*ToolCallResult)
	assert.True(t, result.IsError)
}

func TestServer_HandleRequest_ToolsCall_UpdateRecord(t *testing.T) {
	srv, db := setupServerTest(t)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "items"}
	require.NoError(t, db.Create(table).Error)
	require.NoError(t, db.Create(&models.Field{TableID: table.ID, Name: "status", Type: "string"}).Error)

	record := &models.Record{TableID: table.ID, Data: `{"status":"pending"}`}
	require.NoError(t, db.Create(record).Error)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "update_record",
			"arguments": {"record_id": "` + record.ID + `", "data": {"status": "done"}}
		}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(*ToolCallResult)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "updated")
}

func TestServer_HandleRequest_ToolsCall_UpdateRecord_Error(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "update_record",
			"arguments": {"record_id": "nonexistent", "data": {"status": "done"}}
		}`),
	})

	require.NotNil(t, resp)
	result := resp.Result.(*ToolCallResult)
	assert.True(t, result.IsError)
}

func TestServer_HandleRequest_ToolsCall_InvalidParams(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`invalid json`),
	})

	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Invalid tools/call params")
}

func TestServer_HandleRequest_UnknownMethod_WithID(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "custom/method",
	})

	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "Method not found", resp.Error.Message)
}

func TestServer_HandleRequest_UnknownMethod_WithoutID(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		Method: "custom/method",
	})

	assert.Nil(t, resp)
}

func TestServer_HandleRequest_DefaultJSONRPC(t *testing.T) {
	srv, _ := setupServerTest(t)

	resp := srv.HandleRequest(context.Background(), Request{
		ID:     json.RawMessage(`1`),
		Method: "ping",
	})

	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
}

func TestServer_Success_NilID(t *testing.T) {
	srv := NewServer(nil, "dev")
	resp := srv.success(nil, map[string]interface{}{"key": "val"})
	assert.Nil(t, resp)
}

func TestServer_Success_EmptyID(t *testing.T) {
	srv := NewServer(nil, "dev")
	resp := srv.success(json.RawMessage{}, map[string]interface{}{"key": "val"})
	assert.Nil(t, resp)
}

func TestServer_Failure_NilID(t *testing.T) {
	srv := NewServer(nil, "dev")
	resp := srv.failure(nil, -1, "err", nil)
	assert.Nil(t, resp)
}

func TestServer_Failure_EmptyID(t *testing.T) {
	srv := NewServer(nil, "dev")
	resp := srv.failure(json.RawMessage{}, -1, "err", nil)
	assert.Nil(t, resp)
}

func TestServer_Failure_WithID(t *testing.T) {
	srv := NewServer(nil, "dev")
	resp := srv.failure(json.RawMessage(`1`), -32600, "bad request", "details")
	require.NotNil(t, resp)
	assert.Equal(t, -32600, resp.Error.Code)
	assert.Equal(t, "bad request", resp.Error.Message)
	assert.Equal(t, "details", resp.Error.Data)
}

func TestServer_HandleRequest_ToolsCall_GenerateTestData(t *testing.T) {
	srv, db := setupServerTest(t)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "items"}
	require.NoError(t, db.Create(table).Error)
	require.NoError(t, db.Create(&models.Field{TableID: table.ID, Name: "name", Type: "string", Required: true}).Error)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "generate_test_data",
			"arguments": {"table_id": "` + table.ID + `", "count": 3}
		}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(*ToolCallResult)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "3")
}

func TestServer_HandleRequest_ToolsCall_GetTableSchema_LegacyParam(t *testing.T) {
	srv, db := setupServerTest(t)

	require.NoError(t, db.Create(&models.Database{Name: "TestDB"}).Error)

	resp := srv.HandleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "get_table_schema",
			"arguments": {"table": "databases"}
		}`),
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result := resp.Result.(*ToolCallResult)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "databases")
}
