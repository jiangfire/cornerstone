package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/backend/internal/models"
)

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	assert.GreaterOrEqual(t, len(tools), 11)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Function.(ToolFnDef).Name] = true
	}

	expectedTools := []string{
		"list_databases",
		"list_tables",
		"get_schema",
		"create_database",
		"create_table",
		"create_field",
		"execute_query",
		"insert_records",
		"update_record",
		"delete_record",
		"generate_test_data",
	}

	for _, name := range expectedTools {
		assert.True(t, toolNames[name], "Tool %q should be defined", name)
	}
}

func TestAIAgent_NewAIAgent(t *testing.T) {
	t.Run("default base URL", func(t *testing.T) {
		agent := NewAIAgent("test-key", "gpt-4o", "")
		assert.Equal(t, "https://api.openai.com/v1", agent.BaseURL)
	})

	t.Run("custom base URL", func(t *testing.T) {
		agent := NewAIAgent("test-key", "gpt-4o", "https://custom.api/v1")
		assert.Equal(t, "https://custom.api/v1", agent.BaseURL)
	})

	t.Run("trim trailing slash", func(t *testing.T) {
		agent := NewAIAgent("test-key", "gpt-4o", "https://custom.api/v1/")
		assert.Equal(t, "https://custom.api/v1", agent.BaseURL)
	})
}

func TestExecuteAITool_GetSchema(t *testing.T) {
	db := setupTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	db.Create(&models.Field{TableID: table.ID, Name: "username", Type: "string"})
	db.Create(&models.Field{TableID: table.ID, Name: "email", Type: "string"})

	t.Run("get table schema", func(t *testing.T) {
		result, err := ExecuteAITool("get_schema", map[string]any{
			"table_id": table.ID,
		})
		require.NoError(t, err)

		resMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, table.ID, resMap["table_id"])
		assert.Equal(t, "users", resMap["table_name"])

		fields, ok := resMap["fields"].([]models.Field)
		require.True(t, ok)
		assert.Len(t, fields, 2)
	})

	t.Run("get database schema", func(t *testing.T) {
		result, err := ExecuteAITool("get_schema", map[string]any{
			"database_id": database.ID,
		})
		require.NoError(t, err)

		resMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, database.ID, resMap["database_id"])
		assert.Equal(t, "TestDB", resMap["database_name"])

		tables, ok := resMap["tables"].([]models.Table)
		require.True(t, ok)
		assert.Len(t, tables, 1)
	})

	t.Run("missing parameters", func(t *testing.T) {
		_, err := ExecuteAITool("get_schema", map[string]any{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})
}

func TestExecuteAITool_CreateField(t *testing.T) {
	db := setupTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	result, err := ExecuteAITool("create_field", map[string]any{
		"table_id":    table.ID,
		"name":        "phone",
		"type":        "string",
		"description": "Phone number",
		"required":    true,
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "phone", resMap["name"])
	assert.Equal(t, "string", resMap["type"])
}

func TestExecuteAITool_ExecuteQuery(t *testing.T) {
	db := setupTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	db.Create(&models.Record{TableID: table.ID, Data: `{"name": "Alice"}`})
	db.Create(&models.Record{TableID: table.ID, Data: `{"name": "Bob"}`})

	result, err := ExecuteAITool("execute_query", map[string]any{
		"from":  "records",
		"limit": float64(10),
	})
	require.NoError(t, err)

	results, ok := result.([]map[string]any)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(results), 2)
}
