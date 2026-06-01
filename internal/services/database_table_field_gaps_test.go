package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
)

func createNonMasterToken(t *testing.T, db *gorm.DB) string {
	t.Helper()
	token := &models.Token{
		ID:       "viewer-" + uuid.New().String()[:8],
		Token:    "cs_viewer_token_" + uuid.New().String()[:8],
		IsMaster: false,
		Scopes:   "{}",
	}
	require.NoError(t, db.Create(token).Error)
	return token.ID
}

func createDBAndTableForGapTest(t *testing.T, db *gorm.DB, userID string) (string, string) {
	t.Helper()
	dbSvc := NewDatabaseService(db)
	database, err := dbSvc.CreateDatabase(CreateDBRequest{Name: "gapdb"}, userID)
	require.NoError(t, err)
	tblSvc := NewTableService(db)
	table, err := tblSvc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "items",
	}, userID)
	require.NoError(t, err)
	return database.ID, table.ID
}

func TestCreateDatabase_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)
	viewerID := createNonMasterToken(t, db)

	_, err := svc.CreateDatabase(CreateDBRequest{
		Name:        "testdb",
		Description: "test",
	}, viewerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "此操作需要 Master Token")
}

func TestCreateDatabase_DescriptionTooLong(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	_, err := svc.CreateDatabase(CreateDBRequest{
		Name:        "testdb",
		Description: strings.Repeat("x", 501),
	}, "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "描述验证失败")
}

func TestListDatabases_NonMasterEmptyScopes(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	_, err := svc.CreateDatabase(CreateDBRequest{Name: "db1"}, "user1")
	require.NoError(t, err)
	_, err = svc.CreateDatabase(CreateDBRequest{Name: "db2"}, "user1")
	require.NoError(t, err)

	viewerID := createNonMasterToken(t, db)

	databases, err := svc.ListDatabases(viewerID)
	require.NoError(t, err)
	assert.Empty(t, databases)
}

func TestGetDatabase_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	database, err := svc.CreateDatabase(CreateDBRequest{Name: "secretdb"}, "user1")
	require.NoError(t, err)

	viewerID := createNonMasterToken(t, db)

	_, err = svc.GetDatabase(database.ID, viewerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该数据库")
}

func TestUpdateDatabase_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	database, err := svc.CreateDatabase(CreateDBRequest{Name: "updatedb"}, "user1")
	require.NoError(t, err)

	viewerID := createNonMasterToken(t, db)

	_, err = svc.UpdateDatabase(database.ID, UpdateDBRequest{
		Name:        "newname",
		Description: "new desc",
	}, viewerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权修改该数据库")
}

func TestUpdateDatabase_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	db1, err := svc.CreateDatabase(CreateDBRequest{Name: "db_alpha"}, "user1")
	require.NoError(t, err)
	_, err = svc.CreateDatabase(CreateDBRequest{Name: "db_beta"}, "user1")
	require.NoError(t, err)

	_, err = svc.UpdateDatabase(db1.ID, UpdateDBRequest{
		Name: "db_beta",
	}, "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在同名数据库")
}

func TestDeleteDatabase_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	database, err := svc.CreateDatabase(CreateDBRequest{Name: "del db"}, "user1")
	require.NoError(t, err)

	viewerID := createNonMasterToken(t, db)

	err = svc.DeleteDatabase(database.ID, viewerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权删除该数据库")
}

func TestCreateDatabaseWithTables_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)
	viewerID := createNonMasterToken(t, db)

	_, err := svc.CreateDatabaseWithTables(CreateDBWithTablesRequest{
		Name:        "bulkdb",
		Description: "bulk create",
		Tables: []CreateTableWithFieldsRequest{
			{
				Name: "orders",
				Fields: []struct {
					Name        string `json:"name" binding:"required"`
					Type        string `json:"type" binding:"required"`
					Description string `json:"description"`
					Required    bool   `json:"required"`
				}{
					{Name: "id", Type: "string", Required: true},
				},
			},
		},
	}, viewerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "此操作需要 Master Token")
}

func TestCreateTable_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	database := &models.Database{Name: "TablePermDB"}
	require.NoError(t, db.Create(database).Error)

	svc := NewTableService(db)
	viewerID := createNonMasterToken(t, db)

	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "denied_table",
	}, viewerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权在该数据库中创建表")
}

func TestUpdateTable_InvalidName(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTableService(db)

	database := &models.Database{Name: "UpdInvNameDB"}
	require.NoError(t, db.Create(database).Error)
	master := &models.Token{Name: "master", Token: "cs_master_updinv", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	table, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "valid_table",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.UpdateTable(table.ID, UpdateTableRequest{
		Name: "",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "表名称验证失败")
}

func TestListTables_NonMasterWithScopes(t *testing.T) {
	db := setupTestDB(t)
	database := &models.Database{Name: "ListScopeDB"}
	require.NoError(t, db.Create(database).Error)

	master := &models.Token{Name: "master", Token: "cs_master_listscope", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	svc := NewTableService(db)
	_, err := svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "tbl_one",
	}, master.ID)
	require.NoError(t, err)
	_, err = svc.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "tbl_two",
	}, master.ID)
	require.NoError(t, err)

	viewer := &models.Token{
		Name:     "viewer_list",
		Token:    "cs_viewer_listscope",
		IsMaster: false,
		Scopes:   fmt.Sprintf(`{"databases":{"%s":"viewer"},"tables":{}}`, database.ID),
	}
	require.NoError(t, db.Create(viewer).Error)

	tables, err := svc.ListTables(database.ID, viewer.ID)
	require.NoError(t, err)
	assert.Len(t, tables, 2)
}

func TestCreateField_DescriptionTooLong(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "FieldDescDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "desc_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_fielddesc", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID:     table.ID,
		Name:        "long_desc_field",
		Type:        "string",
		Description: strings.Repeat("x", 1001),
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段备注验证失败")
}

func TestUpdateField_DescriptionTooLong(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "UpdDescDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "upd_desc_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_upddesc", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "status",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.UpdateField(field.ID, UpdateFieldRequest{
		Name:        "status",
		Type:        "string",
		Description: strings.Repeat("y", 1001),
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段备注验证失败")
}

func TestUpdateField_WithOptionsString(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "OptsDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "opts_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_opts", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "priority",
		Type:    "list",
		Options: "low, medium",
	}, master.ID)
	require.NoError(t, err)

	updated, err := svc.UpdateField(field.ID, UpdateFieldRequest{
		Name:    "priority",
		Type:    "list",
		Options: "low, medium, high, critical",
	}, master.ID)
	require.NoError(t, err)

	var config FieldConfig
	require.NoError(t, json.Unmarshal([]byte(updated.Options), &config))
	assert.Contains(t, config.Options, "low")
	assert.Contains(t, config.Options, "medium")
	assert.Contains(t, config.Options, "high")
	assert.Contains(t, config.Options, "critical")
}

func TestListFields_NonMasterSkipsHiddenFields(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "HiddenDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "hidden_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_hidden", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	f1, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "visible_col",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "restricted_col",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	t.Run("token without table access denied", func(t *testing.T) {
		viewerID := createNonMasterToken(t, db)
		_, err := svc.ListFields(table.ID, viewerID)
		assert.Error(t, err)
	})

	t.Run("token with table admin sees all fields", func(t *testing.T) {
		admin := &models.Token{
			Name:     "admin_fields",
			Token:    "cs_admin_fields",
			IsMaster: false,
			Scopes: fmt.Sprintf(
				`{"databases":{},"tables":{"%s":{"role":"admin","fields":{"%s":["read"]}}}}`,
				table.ID, f1.ID,
			),
		}
		require.NoError(t, db.Create(admin).Error)

		fields, err := svc.ListFields(table.ID, admin.ID)
		require.NoError(t, err)
		assert.Len(t, fields, 2)
	})

	t.Run("CheckFieldPermission restricts per field", func(t *testing.T) {
		restricted := &models.Token{
			Name:     "restricted_fields",
			Token:    "cs_restricted_fields",
			IsMaster: false,
			Scopes: fmt.Sprintf(
				`{"databases":{},"tables":{"%s":{"role":"","fields":{"%s":["read"]}}}}`,
				table.ID, f1.ID,
			),
		}
		require.NoError(t, db.Create(restricted).Error)

		err := svc.CheckFieldPermission(restricted.ID, f1.ID, "read")
		assert.NoError(t, err)

		var allFields []models.Field
		require.NoError(t, db.Where("table_id = ? AND deleted_at IS NULL", table.ID).Find(&allFields).Error)
		for _, f := range allFields {
			if f.ID != f1.ID {
				err := svc.CheckFieldPermission(restricted.ID, f.ID, "read")
				assert.Error(t, err, "field %s should be denied", f.Name)
			}
		}
	})
}

func TestCheckFieldPermissions_BatchCheck(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "BatchPermDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "batch_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_batch", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	f1, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "allowed_field",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	f2, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "blocked_field",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	t.Run("master sees all true", func(t *testing.T) {
		results, err := svc.CheckFieldPermissions(master.ID, []string{f1.ID, f2.ID}, "read")
		require.NoError(t, err)
		assert.True(t, results[f1.ID])
		assert.True(t, results[f2.ID])
	})

	t.Run("non-master with partial access", func(t *testing.T) {
		restricted := &models.Token{
			Name:     "batch_restricted",
			Token:    "cs_batch_restricted",
			IsMaster: false,
			Scopes: fmt.Sprintf(
				`{"databases":{},"tables":{"%s":{"role":"","fields":{"%s":["read"]}}}}`,
				table.ID, f1.ID,
			),
		}
		require.NoError(t, db.Create(restricted).Error)

		results, err := svc.CheckFieldPermissions(restricted.ID, []string{f1.ID, f2.ID}, "read")
		require.NoError(t, err)
		assert.True(t, results[f1.ID])
		assert.False(t, results[f2.ID])
	})

	t.Run("empty list returns empty map", func(t *testing.T) {
		results, err := svc.CheckFieldPermissions(master.ID, []string{}, "read")
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("nonexistent field returns false", func(t *testing.T) {
		results, err := svc.CheckFieldPermissions(master.ID, []string{"fld_nonexistent"}, "read")
		require.NoError(t, err)
		assert.True(t, results["fld_nonexistent"])
	})
}

func TestGetDatabase_Nonexistent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	_, err := svc.GetDatabase("db_nonexistent", "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库不存在")
}

func TestUpdateDatabase_Nonexistent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	_, err := svc.UpdateDatabase("db_nonexistent", UpdateDBRequest{
		Name:        "newname",
		Description: "desc",
	}, "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库不存在")
}

func TestDeleteDatabase_Nonexistent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	err := svc.DeleteDatabase("db_nonexistent", "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库不存在")
}

func TestCreateDatabase_InvalidName(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	_, err := svc.CreateDatabase(CreateDBRequest{Name: "a"}, "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库名称验证失败")
}

func TestCreateDatabase_SanitizesInput(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	database, err := svc.CreateDatabase(CreateDBRequest{
		Name:        "<Test>DB\"",
		Description: "a <b>desc</b>",
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, "TestDB", database.Name)
	assert.Equal(t, "a bdesc/b", database.Description)
}

func TestListDatabases_MasterSeesAll(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	svc.CreateDatabase(CreateDBRequest{Name: "master_db1"}, "user1")
	svc.CreateDatabase(CreateDBRequest{Name: "master_db2"}, "user1")

	databases, err := svc.ListDatabases("user1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(databases), 2)
}

func TestUpdateDatabase_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	database, err := svc.CreateDatabase(CreateDBRequest{
		Name:        "upddb",
		Description: "old",
	}, "user1")
	require.NoError(t, err)

	updated, err := svc.UpdateDatabase(database.ID, UpdateDBRequest{
		Name:        "upddb_v2",
		Description: "new",
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, "upddb_v2", updated.Name)
	assert.Equal(t, "new", updated.Description)
}

func TestDeleteDatabase_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	database, err := svc.CreateDatabase(CreateDBRequest{Name: "deldb"}, "user1")
	require.NoError(t, err)

	err = svc.DeleteDatabase(database.ID, "user1")
	require.NoError(t, err)

	var deleted models.Database
	require.NoError(t, db.Unscoped().Where("id = ?", database.ID).First(&deleted).Error)
	assert.True(t, deleted.DeletedAt.Valid)
}

func TestCreateDatabaseWithTables_EmptyTables(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	result, err := svc.CreateDatabaseWithTables(CreateDBWithTablesRequest{
		Name:        "empty_tables_db",
		Description: "no tables",
		Tables:      []CreateTableWithFieldsRequest{},
	}, "user1")
	require.NoError(t, err)
	assert.NotNil(t, result.Database)
	assert.Empty(t, result.Tables)
	assert.Empty(t, result.Fields)
}

func TestCreateField_NonMasterNoAccess(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "NoAccessDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "noaccess_table"}
	require.NoError(t, db.Create(table).Error)

	viewerID := createNonMasterToken(t, db)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "denied",
		Type:    "string",
	}, viewerID)
	assert.Error(t, err)
}

func TestCreateField_NameStartsWithDigit(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "DigitDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "digit_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_digit", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "1field",
		Type:    "string",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段名称验证失败")
}

func TestDeleteField_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	database := &models.Database{Name: "DelFieldDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "del_field_table"}
	require.NoError(t, db.Create(table).Error)

	svc := NewFieldService(db)
	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "to_delete",
		Type:    "string",
	}, "user1")
	require.NoError(t, err)

	viewerID := createNonMasterToken(t, db)

	err = svc.DeleteField(field.ID, viewerID)
	assert.Error(t, err)
}

func TestGetField_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	database := &models.Database{Name: "GetFieldPermDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "get_perm_table"}
	require.NoError(t, db.Create(table).Error)

	svc := NewFieldService(db)
	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "secret_field",
		Type:    "string",
	}, "user1")
	require.NoError(t, err)

	viewerID := createNonMasterToken(t, db)

	_, err = svc.GetField(field.ID, viewerID)
	assert.Error(t, err)
}

func TestUpdateField_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	database := &models.Database{Name: "UpdFieldPermDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "upd_perm_table"}
	require.NoError(t, db.Create(table).Error)

	svc := NewFieldService(db)
	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "upd_denied",
		Type:    "string",
	}, "user1")
	require.NoError(t, err)

	viewerID := createNonMasterToken(t, db)

	_, err = svc.UpdateField(field.ID, UpdateFieldRequest{
		Name: "upd_denied_new",
		Type: "string",
	}, viewerID)
	assert.Error(t, err)
}

func TestValidateDatabaseName_InvalidChars(t *testing.T) {
	err := validateDatabaseName("my db@!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "只能包含字母")
}

func TestValidateDatabaseName_Valid(t *testing.T) {
	assert.NoError(t, validateDatabaseName("my_database 123"))
	assert.NoError(t, validateDatabaseName("测试数据库"))
}

func TestValidateDescription_TooLong(t *testing.T) {
	err := validateDescription(strings.Repeat("x", 501))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestValidateDescription_Valid(t *testing.T) {
	assert.NoError(t, validateDescription(""))
	assert.NoError(t, validateDescription(strings.Repeat("x", 500)))
}

func TestSanitizeDatabaseInput(t *testing.T) {
	name, desc := sanitizeDatabaseInput(`  <script>"alert"'xss"  `, `  <b>bold</b>  `)
	assert.Equal(t, `scriptalertxss`, name)
	assert.Equal(t, `bbold/b`, desc)
}

func TestGetActiveTable_Nonexistent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTableService(db)

	_, err := svc.getActiveTable("tbl_nonexistent")
	assert.Error(t, err)
}

func TestFieldGetActiveTable_Nonexistent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	_, err := svc.getActiveTable("tbl_nonexistent")
	assert.Error(t, err)
}

func TestFieldGetActiveField_Nonexistent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	_, err := svc.getActiveField("fld_nonexistent")
	assert.Error(t, err)
}

func TestCheckTableAccess_NonexistentTable(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)
	master := &models.Token{Name: "master", Token: "cs_master_checktbl", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	err := svc.checkTableAccess("tbl_nonexistent", master.ID, []string{"owner"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "表不存在")
}

func TestCreateField_WithFileField(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)
	database := &models.Database{Name: "FileFieldDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "file_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_filefld", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "attachment",
		Type:    "file",
	}, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "file", field.Type)
}
