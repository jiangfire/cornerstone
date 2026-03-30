package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupToolServiceTest(t *testing.T) (*ToolService, *gorm.DB, models.User) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), newMCPTestGormConfig())
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.DatabaseAccess{},
	))

	user := models.User{
		Username: "tool_user",
		Email:    "tool@example.com",
		Password: "hashed",
	}
	require.NoError(t, db.Create(&user).Error)

	return NewToolService(db, user.ID), db, user
}

func TestToolServiceListDatabasesReturnsEmptyPayload(t *testing.T) {
	service, _, _ := setupToolServiceTest(t)

	result, err := service.callListDatabases()
	require.NoError(t, err)
	require.False(t, result.IsError)

	payload, marshalErr := json.Marshal(result.StructuredContent)
	require.NoError(t, marshalErr)

	var decoded struct {
		Databases []map[string]interface{} `json:"databases"`
		Total     int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Empty(t, decoded.Databases)
	require.Zero(t, decoded.Total)
}

func TestToolServiceGetTableSchemaReturnsErrorForUnknownTable(t *testing.T) {
	service, _, _ := setupToolServiceTest(t)

	result, err := service.callGetTableSchema(context.Background(), json.RawMessage(`{"table":"unknown_table"}`))
	require.NoError(t, err)
	require.True(t, result.IsError)

	payload, marshalErr := json.Marshal(result.StructuredContent)
	require.NoError(t, marshalErr)
	require.Contains(t, string(payload), "unknown_table")
}

func TestToolServiceCallRejectsUnknownTool(t *testing.T) {
	service, _, _ := setupToolServiceTest(t)

	result, err := service.Call(context.Background(), "unknown_tool", json.RawMessage(`{}`))
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown tool")
}

func TestToolServiceCallRejectsInvalidCreateDatabaseArguments(t *testing.T) {
	service, _, _ := setupToolServiceTest(t)

	result, err := service.Call(context.Background(), "create_database", json.RawMessage(`{"name":`))
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid create_database arguments")
}
