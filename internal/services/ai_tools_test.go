package services

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func setupAIToolsTestDB(t *testing.T) *gorm.DB {
	return testutil.SetupTestDB(t)
}

func TestExecuteAITool_ListDatabases(t *testing.T) {
	db := setupAIToolsTestDB(t)

	db.Create(&models.Database{Name: "TestDB1", Description: "Test 1"})
	db.Create(&models.Database{Name: "TestDB2", Description: "Test 2"})

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	result, err := ExecuteAIToolForToken(db, master.ID, "list_databases", map[string]any{})
	require.NoError(t, err)

	databases, ok := result.([]DBResult)
	require.True(t, ok)
	assert.Len(t, databases, 2)
}

func TestExecuteAITool_ListTables(t *testing.T) {
	db := setupAIToolsTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	db.Create(&models.Table{DatabaseID: database.ID, Name: "users"})
	db.Create(&models.Table{DatabaseID: database.ID, Name: "orders"})

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	result, err := ExecuteAIToolForToken(db, master.ID, "list_tables", map[string]any{
		"database_id": database.ID,
	})
	require.NoError(t, err)

	tables, ok := result.([]TableResult)
	require.True(t, ok)
	assert.Len(t, tables, 2)
}

func TestExecuteAITool_CreateDatabase(t *testing.T) {
	db := setupAIToolsTestDB(t)
	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	result, err := ExecuteAIToolForToken(db, master.ID, "create_database", map[string]any{
		"name":        "NewDB",
		"description": "A new database",
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "NewDB", resMap["name"])
}

func TestExecuteAITool_CreateTable(t *testing.T) {
	db := setupAIToolsTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	result, err := ExecuteAIToolForToken(db, master.ID, "create_table", map[string]any{
		"database_id": database.ID,
		"name":        "users",
		"description": "User table",
		"fields": []any{
			map[string]any{
				"name":     "username",
				"type":     "string",
				"required": true,
			},
			map[string]any{
				"name": "email",
				"type": "string",
			},
		},
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "users", resMap["name"])

	var count int64
	db.Table("fields").Where("table_id = ?", resMap["id"]).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestExecuteAITool_InsertRecords(t *testing.T) {
	db := setupAIToolsTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	db.Create(&models.Field{TableID: table.ID, Name: "name", Type: "string"})
	db.Create(&models.Field{TableID: table.ID, Name: "age", Type: "number"})

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	result, err := ExecuteAIToolForToken(db, master.ID, "insert_records", map[string]any{
		"table_id": table.ID,
		"records": []any{
			map[string]any{"name": "Alice", "age": float64(30)},
			map[string]any{"name": "Bob", "age": float64(25)},
		},
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, resMap["inserted"])

	var count int64
	db.Table("records").Where("table_id = ?", table.ID).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestExecuteAITool_UpdateRecord(t *testing.T) {
	db := setupAIToolsTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	db.Create(&models.Field{TableID: table.ID, Name: "name", Type: "string"})
	db.Create(&models.Field{TableID: table.ID, Name: "age", Type: "number"})

	record := &models.Record{
		TableID: table.ID,
		Data:    `{"name": "Alice", "age": 30}`,
		Version: 1,
	}
	db.Create(record)

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	result, err := ExecuteAIToolForToken(db, master.ID, "update_record", map[string]any{
		"record_id": record.ID,
		"data":      map[string]any{"age": float64(31)},
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, resMap["version"])

	var updated models.Record
	db.Where("id = ?", record.ID).First(&updated)
	updatedPayload := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(updated.Data), &updatedPayload))
	assert.Equal(t, float64(31), updatedPayload["age"])
}

func TestExecuteAITool_DeleteRecord(t *testing.T) {
	db := setupAIToolsTestDB(t)

	database := &models.Database{Name: "TestDB"}
	db.Create(database)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	db.Create(table)

	record := &models.Record{
		TableID: table.ID,
		Data:    `{"name": "Alice"}`,
		Version: 1,
	}
	db.Create(record)

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	result, err := ExecuteAIToolForToken(db, master.ID, "delete_record", map[string]any{
		"record_id": record.ID,
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, record.ID, resMap["id"])

	var count int64
	db.Table("records").Where("id = ? AND deleted_at IS NULL", record.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestExecuteAITool_UnknownTool(t *testing.T) {
	db := setupAIToolsTestDB(t)
	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	_, err := ExecuteAIToolForToken(db, master.ID, "unknown_tool", map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestExecuteAIToolForToken_HonorsScopesAndQueryWhitelist(t *testing.T) {
	db := setupAIToolsTestDB(t)

	allowedDB := &models.Database{Name: "allowed"}
	blockedDB := &models.Database{Name: "blocked"}
	require.NoError(t, db.Create(allowedDB).Error)
	require.NoError(t, db.Create(blockedDB).Error)

	allowedTable := &models.Table{DatabaseID: allowedDB.ID, Name: "allowed_records"}
	blockedTable := &models.Table{DatabaseID: blockedDB.ID, Name: "blocked_records"}
	require.NoError(t, db.Create(allowedTable).Error)
	require.NoError(t, db.Create(blockedTable).Error)
	require.NoError(t, db.Create(&models.Record{TableID: allowedTable.ID, Data: `{"name":"allowed"}`}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: blockedTable.ID, Data: `{"name":"blocked"}`}).Error)

	viewer := &models.Token{
		Name:   "viewer",
		Token:  "cs_viewer_scope",
		Scopes: `{"databases":{"` + allowedDB.ID + `":"viewer"}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	result, err := ExecuteAIToolForToken(db, viewer.ID, "list_databases", map[string]any{})
	require.NoError(t, err)
	databases, ok := result.([]DBResult)
	require.True(t, ok)
	require.Len(t, databases, 1)
	assert.Equal(t, allowedDB.ID, databases[0].ID)

	result, err = ExecuteAIToolForToken(db, viewer.ID, "execute_query", map[string]any{
		"from": "records",
	})
	require.NoError(t, err)
	rows, ok := result.([]map[string]any)
	require.True(t, ok)
	require.Len(t, rows, 1)

	_, err = ExecuteAIToolForToken(db, viewer.ID, "create_database", map[string]any{
		"name": "should_fail",
	})
	require.Error(t, err)

	_, err = ExecuteAIToolForToken(db, viewer.ID, "execute_query", map[string]any{
		"from": "sqlite_master",
	})
	require.Error(t, err)
}

func TestGenerateFieldValue(t *testing.T) {
	rng := newRand()

	t.Run("string", func(t *testing.T) {
		val := generateFieldValue(rng, "string")
		assert.IsType(t, "", val)
	})

	t.Run("number", func(t *testing.T) {
		val := generateFieldValue(rng, "number")
		assert.IsType(t, float64(0), val)
	})

	t.Run("boolean", func(t *testing.T) {
		val := generateFieldValue(rng, "boolean")
		assert.IsType(t, false, val)
	})

	t.Run("date", func(t *testing.T) {
		val := generateFieldValue(rng, "date")
		assert.IsType(t, "", val)
	})

	t.Run("list", func(t *testing.T) {
		val := generateFieldValue(rng, "list")
		assert.IsType(t, []string{}, val)
	})
}

func newRand() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}
