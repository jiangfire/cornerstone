package services

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
)

func setupTableTestEnv(t *testing.T) (*TableService, *gorm.DB, *models.Database, *models.Token) {
	db := setupTestDB(t)
	svc := NewTableService(db)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)

	master := &models.Token{Name: "master", Token: "cs_master_tbl", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	return svc, db, database, master
}

// ============================================================
// CreateTable
// ============================================================

func TestTableService_CreateTable_Success(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	table, err := svc.CreateTable(CreateTableRequest{
		DatabaseID:  database.ID,
		Name:        "orders",
		Description: "order table",
	}, master.ID)

	require.NoError(t, err)
	assert.NotEmpty(t, table.ID)
	assert.Equal(t, "orders", table.Name)
	assert.Equal(t, database.ID, table.DatabaseID)
	assert.Equal(t, "order table", table.Description)
}

func TestTableService_CreateTable_DuplicateName(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "a table with this name already exists in the database")
}

func TestTableService_CreateTable_NonexistentDatabase(t *testing.T) {
	svc, _, _, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: "db_nonexistent",
		Name:       "orders",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database not found")
}

func TestTableService_CreateTable_NameTooShort(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "a",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name validation failed")
}

func TestTableService_CreateTable_NameTooLong(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       strings.Repeat("a", 256),
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name validation failed")
}

func TestTableService_CreateTable_NameStartsWithDigit(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "1table",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name must not start with a digit")
}

func TestTableService_CreateTable_NameWithInvalidChars(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "my table!",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name can only contain letters, numbers and underscores")
}

// ============================================================
// ListTables
// ============================================================

func TestTableService_ListTables_ReturnsTables(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "customers",
	}, master.ID)
	require.NoError(t, err)

	tables, err := svc.ListTables(database.ID, master.ID)
	require.NoError(t, err)
	assert.Len(t, tables, 2)
	names := []string{tables[0].Name, tables[1].Name}
	assert.Contains(t, names, "orders")
	assert.Contains(t, names, "customers")
}

func TestTableService_ListTables_NoAccess(t *testing.T) {
	svc, db, database, _ := setupTableTestEnv(t)

	viewer := &models.Token{
		Name:     "viewer",
		Token:    "cs_viewer_tbl",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	tables, err := svc.ListTables(database.ID, viewer.ID)
	assert.Error(t, err)
	assert.Nil(t, tables)
	assert.Contains(t, err.Error(), "permission denied: cannot access tables in this database")
}

// ============================================================
// GetTable
// ============================================================

func TestTableService_GetTable_Success(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID:  database.ID,
		Name:        "orders",
		Description: "order table",
	}, master.ID)
	require.NoError(t, err)

	table, err := svc.GetTable(created.ID, master.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, table.ID)
	assert.Equal(t, "orders", table.Name)
	assert.Equal(t, database.ID, table.DatabaseID)
}

func TestTableService_GetTable_Nonexistent(t *testing.T) {
	svc, _, _, master := setupTableTestEnv(t)

	_, err := svc.GetTable("tbl_nonexistent", master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table not found")
}

func TestTableService_GetTable_NoAccess(t *testing.T) {
	svc, db, database, _ := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, "user1")
	require.NoError(t, err)

	viewer := &models.Token{
		Name:     "viewer",
		Token:    "cs_viewer_tbl2",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	_, err = svc.GetTable(created.ID, viewer.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied: cannot access this table")
}

// ============================================================
// UpdateTable
// ============================================================

func TestTableService_UpdateTable_Success(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID:  database.ID,
		Name:        "orders",
		Description: "old desc",
	}, master.ID)
	require.NoError(t, err)

	updated, err := svc.UpdateTable(created.ID, UpdateTableRequest{
		Name:        "orders_v2",
		Description: "new desc",
	}, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "orders_v2", updated.Name)
	assert.Equal(t, "new desc", updated.Description)
}

func TestTableService_UpdateTable_DuplicateName(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)

	created2, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "customers",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.UpdateTable(created2.ID, UpdateTableRequest{
		Name: "orders",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "a table with this name already exists in the database")
}

func TestTableService_UpdateTable_Nonexistent(t *testing.T) {
	svc, _, _, master := setupTableTestEnv(t)

	_, err := svc.UpdateTable("tbl_nonexistent", UpdateTableRequest{
		Name: "new_name",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table not found")
}

// ============================================================
// DeleteTable
// ============================================================

func TestTableService_DeleteTable_Success(t *testing.T) {
	svc, db, database, master := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)

	err = svc.DeleteTable(created.ID, master.ID)
	require.NoError(t, err)

	_, err = svc.GetTable(created.ID, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table not found")

	var deleted models.Table
	require.NoError(t, db.Unscoped().Where("id = ?", created.ID).First(&deleted).Error)
	assert.True(t, deleted.DeletedAt.Valid)
}

func TestTableService_DeleteTable_SoftDeleteNameSuffix(t *testing.T) {
	svc, db, database, master := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)

	err = svc.DeleteTable(created.ID, master.ID)
	require.NoError(t, err)

	var deleted models.Table
	require.NoError(t, db.Unscoped().Where("id = ?", created.ID).First(&deleted).Error)
	assert.Contains(t, deleted.Name, "__deleted__")
	assert.True(t, deleted.DeletedAt.Valid)
}

func TestTableService_DeleteTable_Nonexistent(t *testing.T) {
	svc, _, _, master := setupTableTestEnv(t)

	err := svc.DeleteTable("tbl_nonexistent", master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table not found")
}

func TestTableService_DeleteTable_NoAccess(t *testing.T) {
	svc, db, database, _ := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, "user1")
	require.NoError(t, err)

	viewer := &models.Token{
		Name:     "viewer",
		Token:    "cs_viewer_tbl3",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	err = svc.DeleteTable(created.ID, viewer.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied: cannot delete this table")
}

// ============================================================
// validateTableName
// ============================================================

func TestValidateTableName_Valid(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"simple", "orders"},
		{"underscore", "my_table"},
		{"unicode", "user_table"},
		{"mixed", "table_1"},
		{"two chars", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, validateTableName(tt.in))
		})
	}
}

func TestValidateTableName_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		substr string
	}{
		{"too short", "a", "must be between"},
		{"too long", strings.Repeat("x", 256), "must be between"},
		{"starts with digit", "1table", "table name must not start with a digit"},
		{"spaces", "my table", "table name can only contain letters, numbers and underscores"},
		{"special chars", "table!", "table name can only contain letters, numbers and underscores"},
		{"hyphen", "my-table", "table name can only contain letters, numbers and underscores"},
		{"empty", "", "must be between"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTableName(tt.in)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.substr)
		})
	}
}

// ============================================================
// sanitizeTableInput
// ============================================================

func TestSanitizeTableInput(t *testing.T) {
	tests := []struct {
		name       string
		inputName  string
		inputDesc  string
		expectName string
		expectDesc string
	}{
		{
			"strips angle brackets and quotes from name",
			`my<table>"name'`, "",
			"mytablename", "",
		},
		{
			"strips angle brackets and quotes from description",
			"ok", `desc<ri>"pti'on`,
			"ok", "description",
		},
		{
			"trims whitespace",
			"  table  ", "  desc  ",
			"table", "desc",
		},
		{
			"clean input unchanged",
			"my_table", "normal description",
			"my_table", "normal description",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc := sanitizeTableInput(tt.inputName, tt.inputDesc)
			assert.Equal(t, tt.expectName, name)
			assert.Equal(t, tt.expectDesc, desc)
		})
	}
}

// ============================================================
// buildDeletedTableName
// ============================================================

func TestBuildDeletedTableName(t *testing.T) {
	t.Run("normal name", func(t *testing.T) {
		result := buildDeletedTableName("orders", "tbl_123")
		assert.Equal(t, "orders__deleted__tbl_123", result)
	})

	t.Run("truncates long name", func(t *testing.T) {
		longName := strings.Repeat("x", 300)
		tableID := "tbl_1234567890"
		result := buildDeletedTableName(longName, tableID)
		assert.LessOrEqual(t, len(result), 255)
		assert.Contains(t, result, "__deleted__"+tableID)
	})
}

func TestCreateTable_SameNameDifferentDatabase(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTableService(db)

	db1 := &models.Database{Name: "DB1"}
	require.NoError(t, db.Create(db1).Error)
	db2 := &models.Database{Name: "DB2"}
	require.NoError(t, db.Create(db2).Error)

	master := &models.Token{Name: "master", Token: "cs_master_samenamedb", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	t1, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: db1.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, t1.ID)

	t2, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: db2.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, t2.ID)
	assert.NotEqual(t, t1.ID, t2.ID)
}

func TestListTables_EmptyDatabase(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	tables, err := svc.ListTables(database.ID, master.ID)
	require.NoError(t, err)
	assert.Empty(t, tables)
}

func TestGetTable_ResponseBodyFormat(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID:  database.ID,
		Name:        "orders",
		Description: "test table",
	}, master.ID)
	require.NoError(t, err)

	resp, err := svc.GetTable(created.ID, master.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, resp.ID)
	assert.Equal(t, database.ID, resp.DatabaseID)
}

func TestUpdateTable_SameNameAllowed(t *testing.T) {
	svc, _, database, master := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)

	updated, err := svc.UpdateTable(created.ID, UpdateTableRequest{
		Name:        "orders",
		Description: "updated description",
	}, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "orders", updated.Name)
	assert.Equal(t, "updated description", updated.Description)
}

func TestDeleteTable_BuildsDeletedName(t *testing.T) {
	svc, db, database, master := setupTableTestEnv(t)

	created, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "orders",
	}, master.ID)
	require.NoError(t, err)

	expectedName := fmt.Sprintf("orders__deleted__%s", created.ID)

	err = svc.DeleteTable(created.ID, master.ID)
	require.NoError(t, err)

	var deleted models.Table
	require.NoError(t, db.Unscoped().Where("id = ?", created.ID).First(&deleted).Error)
	assert.Equal(t, expectedName, deleted.Name)
}
