package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
)

func setupMCPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	err := pkgdb.InitDB(config.DatabaseConfig{
		Type: "sqlite",
		URL:  ":memory:",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
	})

	db := pkgdb.DB()
	err = db.AutoMigrate(
		&models.Token{},
		&models.Database{},
		&models.Table{},
		&models.Field{},
		&models.Record{},
		&models.File{},
	)
	require.NoError(t, err)

	return db
}

func TestToolService_ListTools(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	tools := svc.ListTools()
	assert.GreaterOrEqual(t, len(tools), 10)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"query_data",
		"create_database",
		"list_databases",
		"get_table_schema",
		"create_table",
		"create_field",
		"insert_record",
		"update_record",
		"delete_record",
		"generate_test_data",
	}

	for _, name := range expectedTools {
		assert.True(t, toolNames[name], "Tool %q should exist", name)
	}
}

func TestToolService_Call_UnknownTool(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	_, err := svc.Call(context.Background(), "unknown_tool", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestToolService_Call_ListDatabases(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	db.Create(&models.Database{Name: "DB1"})
	db.Create(&models.Database{Name: "DB2"})

	result, err := svc.Call(context.Background(), "list_databases", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "2")
}

func TestToolService_Call_CreateDatabase(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	args, _ := json.Marshal(map[string]any{
		"name":        "TestDB",
		"description": "Test database",
	})

	result, err := svc.Call(context.Background(), "create_database", args)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "TestDB")
}

func TestToolService_Call_CreateTable(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	args, _ := json.Marshal(map[string]any{
		"database_id": database.ID,
		"name":        "users",
		"description": "User table",
		"fields": []any{
			map[string]any{"name": "username", "type": "string", "required": true},
			map[string]any{"name": "email", "type": "string"},
		},
	})

	result, err := svc.Call(context.Background(), "create_table", args)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "users")

	var fieldCount int64
	db.Table("fields").Count(&fieldCount)
	assert.Equal(t, int64(2), fieldCount)
}

func TestToolService_Call_InsertRecord(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	db.Create(&models.Field{TableID: table.ID, Name: "name", Type: "string", Required: true})
	db.Create(&models.Field{TableID: table.ID, Name: "age", Type: "number"})

	args, _ := json.Marshal(map[string]any{
		"table_id": table.ID,
		"data":     map[string]any{"name": "Alice", "age": float64(30)},
	})

	result, err := svc.Call(context.Background(), "insert_record", args)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "inserted")

	var recordCount int64
	db.Table("records").Count(&recordCount)
	assert.Equal(t, int64(1), recordCount)
}

func TestToolService_Call_DeleteRecord(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	db.Create(&models.Field{TableID: table.ID, Name: "name", Type: "string", Required: true})

	insertArgs, _ := json.Marshal(map[string]any{
		"table_id": table.ID,
		"data":     map[string]any{"name": "Alice"},
	})
	insertResult, err := svc.Call(context.Background(), "insert_record", insertArgs)
	require.NoError(t, err)
	require.False(t, insertResult.IsError)

	var record models.Record
	db.Where("table_id = ? AND deleted_at IS NULL", table.ID).First(&record)

	args, _ := json.Marshal(map[string]any{
		"record_id": record.ID,
	})

	result, err := svc.Call(context.Background(), "delete_record", args)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "deleted")

	var activeCount int64
	db.Table("records").Where("deleted_at IS NULL").Count(&activeCount)
	assert.Equal(t, int64(0), activeCount)
}

func TestToolService_Call_GenerateTestData(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	args, _ := json.Marshal(map[string]any{
		"table_id": table.ID,
		"count":    float64(5),
	})

	result, err := svc.Call(context.Background(), "generate_test_data", args)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "5")

	var recordCount int64
	db.Table("records").Count(&recordCount)
	assert.Equal(t, int64(5), recordCount)
}

func TestToolService_Call_GenerateTestData_InvalidCount(t *testing.T) {
	db := setupMCPTestDB(t)
	svc := NewToolService(db, "test_user")

	args, _ := json.Marshal(map[string]any{
		"table_id": "tbl_xxx",
		"count":    float64(200),
	})

	result, err := svc.Call(context.Background(), "generate_test_data", args)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
