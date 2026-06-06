package authz

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/jiangfire/cornerstone/internal/models"
)

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "authz-test.sqlite")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Token{},
		&models.Database{},
		&models.Table{},
		&models.Field{},
		&models.Record{},
		&models.File{},
	))

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func createMasterToken(t *testing.T, d *gorm.DB) *models.Token {
	t.Helper()
	token := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(token).Error)
	return token
}

func createNonMasterToken(t *testing.T, d *gorm.DB, scopes string) *models.Token {
	t.Helper()
	token := &models.Token{Name: "worker", IsMaster: false, Scopes: scopes}
	require.NoError(t, d.Create(token).Error)
	return token
}

func createTestData(t *testing.T, d *gorm.DB) (*models.Database, *models.Table, []*models.Field) {
	t.Helper()
	db1 := &models.Database{Name: "db1"}
	require.NoError(t, d.Create(db1).Error)
	tbl1 := &models.Table{DatabaseID: db1.ID, Name: "tbl1"}
	require.NoError(t, d.Create(tbl1).Error)
	f1 := &models.Field{TableID: tbl1.ID, Name: "f1", Type: "string"}
	f2 := &models.Field{TableID: tbl1.ID, Name: "f2", Type: "number"}
	require.NoError(t, d.Create(f1).Error)
	require.NoError(t, d.Create(f2).Error)
	return db1, tbl1, []*models.Field{f1, f2}
}

func TestNewAuthorizer_NilDB(t *testing.T) {
	_, err := NewAuthorizer(nil, "some-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库未初始化")
}

func TestNewAuthorizer_NonexistentToken(t *testing.T) {
	d := setupDB(t)
	_, err := NewAuthorizer(d, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token 不存在")
}

func TestNewAuthorizer_MasterToken(t *testing.T) {
	d := setupDB(t)
	tok := createMasterToken(t, d)
	ClearTokenCache()

	a, err := NewAuthorizer(d, tok.ID)
	require.NoError(t, err)
	assert.True(t, a.IsMaster())
}

func TestNewAuthorizer_NonMasterToken(t *testing.T) {
	d := setupDB(t)
	tok := createNonMasterToken(t, d, `{"databases":{},"tables":{}}`)
	ClearTokenCache()

	a, err := NewAuthorizer(d, tok.ID)
	require.NoError(t, err)
	assert.False(t, a.IsMaster())
}

func TestNewAuthorizer_CachedToken(t *testing.T) {
	d := setupDB(t)
	tok := createMasterToken(t, d)
	ClearTokenCache()

	a1, err := NewAuthorizer(d, tok.ID)
	require.NoError(t, err)
	require.NotNil(t, a1)

	a2, err := NewAuthorizer(d, tok.ID)
	require.NoError(t, err)
	assert.True(t, a2.IsMaster())
}

func TestParseScopes_Empty(t *testing.T) {
	scopes, err := parseScopes("")
	require.NoError(t, err)
	assert.NotNil(t, scopes.Databases)
	assert.NotNil(t, scopes.Tables)
	assert.Len(t, scopes.Databases, 0)
	assert.Len(t, scopes.Tables, 0)
}

func TestParseScopes_Whitespace(t *testing.T) {
	scopes, err := parseScopes("   ")
	require.NoError(t, err)
	assert.NotNil(t, scopes.Databases)
	assert.NotNil(t, scopes.Tables)
}

func TestParseScopes_ValidJSON(t *testing.T) {
	raw := `{"databases":{"db_1":"admin"},"tables":{"tbl_1":{"role":"viewer"}}}`
	scopes, err := parseScopes(raw)
	require.NoError(t, err)
	assert.Equal(t, "admin", scopes.Databases["db_1"])
	assert.Equal(t, "viewer", scopes.Tables["tbl_1"].Role)
}

func TestParseScopes_ValidJSONWithNilCollections(t *testing.T) {
	raw := `{}`
	scopes, err := parseScopes(raw)
	require.NoError(t, err)
	assert.NotNil(t, scopes.Databases)
	assert.NotNil(t, scopes.Tables)
}

func TestParseScopes_InvalidJSON(t *testing.T) {
	_, err := parseScopes("{invalid}")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析 Token Scopes 失败")
}

func TestCanCreateDatabase(t *testing.T) {
	d := setupDB(t)
	master := createMasterToken(t, d)
	worker := createNonMasterToken(t, d, "{}")
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	wa, _ := NewAuthorizer(d, worker.ID)

	assert.True(t, ma.CanCreateDatabase())
	assert.False(t, wa.CanCreateDatabase())
}

func TestCanAccessDatabase_Master(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	assert.True(t, ma.CanAccessDatabase(db1.ID, ActionRead))
	assert.True(t, ma.CanAccessDatabase(db1.ID, ActionWrite))
	assert.True(t, ma.CanAccessDatabase(db1.ID, ActionDelete))
	assert.True(t, ma.CanAccessDatabase(db1.ID, ActionManage))
}

func TestCanAccessDatabase_NonMaster(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	scopes := `{"databases":{"` + db1.ID + `":"viewer"},"tables":{}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)

	assert.True(t, wa.CanAccessDatabase(db1.ID, ActionRead))
	assert.False(t, wa.CanAccessDatabase(db1.ID, ActionWrite))
	assert.False(t, wa.CanAccessDatabase(db1.ID, ActionDelete))
}

func TestCanAccessDatabase_NonMasterEditor(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	scopes := `{"databases":{"` + db1.ID + `":"editor"},"tables":{}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.True(t, wa.CanAccessDatabase(db1.ID, ActionRead))
	assert.True(t, wa.CanAccessDatabase(db1.ID, ActionWrite))
	assert.False(t, wa.CanAccessDatabase(db1.ID, ActionDelete))
}

func TestCanAccessDatabase_NonMasterAdmin(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	scopes := `{"databases":{"` + db1.ID + `":"admin"},"tables":{}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.True(t, wa.CanAccessDatabase(db1.ID, ActionRead))
	assert.True(t, wa.CanAccessDatabase(db1.ID, ActionWrite))
	assert.True(t, wa.CanAccessDatabase(db1.ID, ActionDelete))
	assert.True(t, wa.CanAccessDatabase(db1.ID, ActionManage))
}

func TestCanAccessDatabase_NoAccess(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	worker := createNonMasterToken(t, d, `{"databases":{},"tables":{}}`)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.False(t, wa.CanAccessDatabase(db1.ID, ActionRead))
}

func TestCanAccessTable_Master(t *testing.T) {
	d := setupDB(t)
	_, tbl1, _ := createTestData(t, d)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	assert.True(t, ma.CanAccessTable(tbl1.ID, ActionRead))
	assert.True(t, ma.CanAccessTable(tbl1.ID, ActionWrite))
}

func TestCanAccessTable_TableScopeOverride(t *testing.T) {
	d := setupDB(t)
	db1, tbl1, _ := createTestData(t, d)
	scopes := `{"databases":{"` + db1.ID + `":"viewer"},"tables":{"` + tbl1.ID + `":{"role":"admin"}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.True(t, wa.CanAccessTable(tbl1.ID, ActionDelete))
}

func TestCanAccessTable_FallbackToDatabaseScope(t *testing.T) {
	d := setupDB(t)
	db1, tbl1, _ := createTestData(t, d)
	scopes := `{"databases":{"` + db1.ID + `":"editor"},"tables":{}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.True(t, wa.CanAccessTable(tbl1.ID, ActionRead))
	assert.True(t, wa.CanAccessTable(tbl1.ID, ActionWrite))
	assert.False(t, wa.CanAccessTable(tbl1.ID, ActionDelete))
}

func TestCanAccessTable_NoScope(t *testing.T) {
	d := setupDB(t)
	_, tbl1, _ := createTestData(t, d)
	worker := createNonMasterToken(t, d, `{"databases":{},"tables":{}}`)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.False(t, wa.CanAccessTable(tbl1.ID, ActionRead))
}

func TestCanAccessField_Master(t *testing.T) {
	d := setupDB(t)
	_, _, fields := createTestData(t, d)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	assert.True(t, ma.CanAccessField(fields[0].ID, ActionRead))
	assert.True(t, ma.CanAccessField(fields[0].ID, ActionWrite))
}

func TestCanAccessField_FieldScope(t *testing.T) {
	d := setupDB(t)
	_, tbl1, fields := createTestData(t, d)
	fID := fields[0].ID
	scopes := `{"databases":{},"tables":{"` + tbl1.ID + `":{"role":"viewer","fields":{"` + fID + `":["write"]}}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.True(t, wa.CanAccessField(fID, ActionWrite))
}

func TestCanAccessField_FieldScopeByName(t *testing.T) {
	d := setupDB(t)
	_, tbl1, fields := createTestData(t, d)
	scopes := `{"databases":{},"tables":{"` + tbl1.ID + `":{"role":"viewer","fields":{"f1":["write"]}}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.True(t, wa.CanAccessField(fields[0].ID, ActionWrite))
}

func TestCanAccessField_FallbackToTable(t *testing.T) {
	d := setupDB(t)
	_, tbl1, fields := createTestData(t, d)
	scopes := `{"databases":{},"tables":{"` + tbl1.ID + `":{"role":"editor"}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.True(t, wa.CanAccessField(fields[0].ID, ActionRead))
	assert.True(t, wa.CanAccessField(fields[0].ID, ActionWrite))
}

func TestCanAccessField_NonexistentField(t *testing.T) {
	d := setupDB(t)
	worker := createNonMasterToken(t, d, `{"databases":{},"tables":{}}`)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	assert.False(t, wa.CanAccessField("nonexistent", ActionRead))
}

func TestCanAccessFields_Master(t *testing.T) {
	d := setupDB(t)
	_, _, fields := createTestData(t, d)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	ids := []string{fields[0].ID, fields[1].ID}
	results := ma.CanAccessFields(ids, ActionRead)
	assert.Len(t, results, 2)
	assert.True(t, results[fields[0].ID])
	assert.True(t, results[fields[1].ID])
}

func TestCanAccessFields_Empty(t *testing.T) {
	d := setupDB(t)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	results := ma.CanAccessFields(nil, ActionRead)
	assert.Len(t, results, 0)
}

func TestCanAccessFields_NonMaster(t *testing.T) {
	d := setupDB(t)
	_, tbl1, fields := createTestData(t, d)
	fID := fields[0].ID
	scopes := `{"databases":{},"tables":{"` + tbl1.ID + `":{"role":"editor","fields":{"` + fID + `":["read"]}}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	results := wa.CanAccessFields([]string{fields[0].ID, fields[1].ID}, ActionRead)
	assert.True(t, results[fields[0].ID])
	assert.True(t, results[fields[1].ID])
}

func TestCanAccessFields_NonMaster_WithFieldOverride(t *testing.T) {
	d := setupDB(t)
	_, tbl1, fields := createTestData(t, d)
	f1ID := fields[0].ID
	f2ID := fields[1].ID
	scopes := `{"databases":{},"tables":{"` + tbl1.ID + `":{"role":"viewer","fields":{"` + f1ID + `":["write"]}}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	results := wa.CanAccessFields([]string{f1ID, f2ID}, ActionWrite)
	assert.True(t, results[f1ID])
	assert.False(t, results[f2ID])
}

func TestCanAccessFields_NonexistentField(t *testing.T) {
	d := setupDB(t)
	worker := createNonMasterToken(t, d, `{"databases":{},"tables":{}}`)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	results := wa.CanAccessFields([]string{"nonexistent"}, ActionRead)
	assert.False(t, results["nonexistent"])
}

func TestRoleHierarchy(t *testing.T) {
	assert.Equal(t, 1, roleLevel("viewer"))
	assert.Equal(t, 2, roleLevel("editor"))
	assert.Equal(t, 3, roleLevel("admin"))
	assert.Equal(t, 0, roleLevel(""))
	assert.Equal(t, 0, roleLevel("unknown"))
	assert.Equal(t, 1, roleLevel("Viewer"))
	assert.Equal(t, 2, roleLevel("Editor"))
	assert.Equal(t, 3, roleLevel("Admin"))
}

func TestRequiredRoleLevel(t *testing.T) {
	assert.Equal(t, 1, requiredRoleLevel(ActionRead))
	assert.Equal(t, 2, requiredRoleLevel(ActionWrite))
	assert.Equal(t, 3, requiredRoleLevel(ActionDelete))
	assert.Equal(t, 3, requiredRoleLevel(ActionManage))
	assert.Equal(t, 0, requiredRoleLevel("unknown"))
}

func TestAccessibleDatabaseIDs_Master(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	db2 := &models.Database{Name: "db2"}
	require.NoError(t, d.Create(db2).Error)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	ids, err := ma.AccessibleDatabaseIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, db1.ID)
	assert.Contains(t, ids, db2.ID)
}

func TestAccessibleDatabaseIDs_NonMaster(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	db2 := &models.Database{Name: "db2"}
	require.NoError(t, d.Create(db2).Error)
	scopes := `{"databases":{"` + db1.ID + `":"admin"},"tables":{}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleDatabaseIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Contains(t, ids, db1.ID)
}

func TestAccessibleDatabaseIDs_NonMasterNoAccess(t *testing.T) {
	d := setupDB(t)
	createTestData(t, d)
	worker := createNonMasterToken(t, d, `{"databases":{},"tables":{}}`)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleDatabaseIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 0)
}

func TestAccessibleDatabaseIDs_ViewerRole(t *testing.T) {
	d := setupDB(t)
	db1, _, _ := createTestData(t, d)
	scopes := `{"databases":{"` + db1.ID + `":"viewer"},"tables":{}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleDatabaseIDs()
	require.NoError(t, err)
	assert.Contains(t, ids, db1.ID)
}

func TestAccessibleTableIDs_Master(t *testing.T) {
	d := setupDB(t)
	_, tbl1, _ := createTestData(t, d)
	tbl2 := &models.Table{DatabaseID: tbl1.DatabaseID, Name: "tbl2"}
	require.NoError(t, d.Create(tbl2).Error)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	ids, err := ma.AccessibleTableIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, tbl1.ID)
	assert.Contains(t, ids, tbl2.ID)
}

func TestAccessibleTableIDs_NonMaster_TableScope(t *testing.T) {
	d := setupDB(t)
	db1, tbl1, _ := createTestData(t, d)
	tbl2 := &models.Table{DatabaseID: db1.ID, Name: "tbl2"}
	require.NoError(t, d.Create(tbl2).Error)
	scopes := `{"databases":{},"tables":{"` + tbl1.ID + `":{"role":"viewer"}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleTableIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Contains(t, ids, tbl1.ID)
}

func TestAccessibleTableIDs_NonMaster_DatabaseDerived(t *testing.T) {
	d := setupDB(t)
	db1, tbl1, _ := createTestData(t, d)
	scopes := `{"databases":{"` + db1.ID + `":"admin"},"tables":{}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleTableIDs()
	require.NoError(t, err)
	assert.Contains(t, ids, tbl1.ID)
}

func TestAccessibleTableIDs_NonMaster_Combined(t *testing.T) {
	d := setupDB(t)
	db1, tbl1, _ := createTestData(t, d)
	tbl2 := &models.Table{DatabaseID: db1.ID, Name: "tbl2"}
	require.NoError(t, d.Create(tbl2).Error)
	tbl3 := &models.Table{DatabaseID: db1.ID, Name: "tbl3"}
	require.NoError(t, d.Create(tbl3).Error)
	scopes := `{"databases":{"` + db1.ID + `":"admin"},"tables":{"` + tbl3.ID + `":{"role":"viewer"}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleTableIDs()
	require.NoError(t, err)
	assert.Contains(t, ids, tbl1.ID)
	assert.Contains(t, ids, tbl2.ID)
	assert.Contains(t, ids, tbl3.ID)
}

func TestAccessibleRecordIDs_Master(t *testing.T) {
	d := setupDB(t)
	_, tbl1, _ := createTestData(t, d)
	rec1 := &models.Record{TableID: tbl1.ID, Data: `{}`, Version: 1}
	rec2 := &models.Record{TableID: tbl1.ID, Data: `{}`, Version: 1}
	require.NoError(t, d.Create(rec1).Error)
	require.NoError(t, d.Create(rec2).Error)
	master := createMasterToken(t, d)
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	ids, err := ma.AccessibleRecordIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 2)
}

func TestAccessibleRecordIDs_NonMaster(t *testing.T) {
	d := setupDB(t)
	_, tbl1, _ := createTestData(t, d)
	rec1 := &models.Record{TableID: tbl1.ID, Data: `{}`, Version: 1}
	require.NoError(t, d.Create(rec1).Error)
	worker := createNonMasterToken(t, d, `{"databases":{},"tables":{}}`)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleRecordIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 0)
}

func TestAccessibleRecordIDs_NonMaster_WithTableAccess(t *testing.T) {
	d := setupDB(t)
	_, tbl1, _ := createTestData(t, d)
	rec1 := &models.Record{TableID: tbl1.ID, Data: `{}`, Version: 1}
	require.NoError(t, d.Create(rec1).Error)
	scopes := `{"databases":{},"tables":{"` + tbl1.ID + `":{"role":"viewer"}}}`
	worker := createNonMasterToken(t, d, scopes)
	ClearTokenCache()

	wa, _ := NewAuthorizer(d, worker.ID)
	ids, err := wa.AccessibleRecordIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Contains(t, ids, rec1.ID)
}

func TestInvalidateTokenCache(t *testing.T) {
	d := setupDB(t)
	tok := createMasterToken(t, d)
	ClearTokenCache()

	_, err := NewAuthorizer(d, tok.ID)
	require.NoError(t, err)

	InvalidateTokenCache(tok.ID)

	tok.IsMaster = false
	require.NoError(t, d.Save(tok).Error)

	a, err := NewAuthorizer(d, tok.ID)
	require.NoError(t, err)
	assert.False(t, a.IsMaster())
}

func TestFindTokenByValue_UsesInvalidateTokenCache(t *testing.T) {
	d := setupDB(t)
	tok := createNonMasterToken(t, d, "{}")
	ClearTokenCache()

	cached, err := FindTokenByValue(d, tok.Token)
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, tok.ID, cached.ID)

	tok.Scopes = `{"databases":{"db_x":"viewer"},"tables":{}}`
	require.NoError(t, d.Save(tok).Error)

	stale, err := FindTokenByValue(d, tok.Token)
	require.NoError(t, err)
	assert.Equal(t, "{}", stale.Scopes)

	InvalidateTokenCache(tok.ID)

	fresh, err := FindTokenByValue(d, tok.Token)
	require.NoError(t, err)
	assert.Equal(t, tok.Scopes, fresh.Scopes)
}

func TestClearTokenCache(t *testing.T) {
	d := setupDB(t)
	tok1 := createMasterToken(t, d)
	tok2 := createNonMasterToken(t, d, "{}")
	ClearTokenCache()

	_, _ = NewAuthorizer(d, tok1.ID)
	_, _ = NewAuthorizer(d, tok2.ID)

	ClearTokenCache()

	tok1.IsMaster = false
	require.NoError(t, d.Save(tok1).Error)

	a, err := NewAuthorizer(d, tok1.ID)
	require.NoError(t, err)
	assert.False(t, a.IsMaster())

	a2, err := NewAuthorizer(d, tok2.ID)
	require.NoError(t, err)
	assert.False(t, a2.IsMaster())
}

func TestRequireMaster(t *testing.T) {
	d := setupDB(t)
	master := createMasterToken(t, d)
	worker := createNonMasterToken(t, d, "{}")
	ClearTokenCache()

	ma, _ := NewAuthorizer(d, master.ID)
	wa, _ := NewAuthorizer(d, worker.ID)

	assert.NoError(t, ma.RequireMaster())
	assert.Error(t, wa.RequireMaster())
}

func TestContainsAction(t *testing.T) {
	assert.True(t, containsAction([]string{"read", "write"}, "read"))
	assert.True(t, containsAction([]string{"read", "write"}, "Write"))
	assert.False(t, containsAction([]string{"read"}, "write"))
	assert.False(t, containsAction(nil, "read"))
	assert.False(t, containsAction([]string{}, "read"))
}
