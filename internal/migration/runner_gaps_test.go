package migration

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/migration/source"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
	"gorm.io/gorm"
)

func setupRunnerWithDB(t *testing.T) (*Runner, *gorm.DB) {
	t.Helper()
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: "test.db"},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           100,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			CheckpointInterval: 100,
			RollbackOnFailure:  RollbackTable,
		},
	}, RunnerOptions{
		StateDir: t.TempDir(),
	})
	require.NoError(t, err)
	return runner, db
}

func createTestTableWithRecords(t *testing.T, db *gorm.DB) (*models.Database, *models.Table, []models.Field, []models.Record) {
	t.Helper()
	dbModel := &models.Database{Name: "rollback_test_db"}
	require.NoError(t, db.Create(dbModel).Error)

	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "orders"}
	require.NoError(t, db.Create(tbl).Error)

	fields := []models.Field{
		{TableID: tbl.ID, Name: "id", Type: "number", Required: true},
		{TableID: tbl.ID, Name: "status", Type: "string"},
		{TableID: tbl.ID, Name: "total", Type: "number"},
	}
	for i := range fields {
		require.NoError(t, db.Create(&fields[i]).Error)
	}

	records := []models.Record{
		{TableID: tbl.ID, Data: `{"id":1,"status":"pending","total":100}`, Version: 1},
		{TableID: tbl.ID, Data: `{"id":2,"status":"shipped","total":250}`, Version: 1},
		{TableID: tbl.ID, Data: `{"id":3,"status":"delivered","total":500}`, Version: 1},
	}
	for i := range records {
		require.NoError(t, db.Create(&records[i]).Error)
	}

	return dbModel, tbl, fields, records
}

func TestRollbackTable_SoftDeletesAll(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, _ := createTestTableWithRecords(t, db)

	err := runner.rollbackTable(tbl.ID)
	require.NoError(t, err)

	var deletedRecords []models.Record
	require.NoError(t, db.Unscoped().Where("table_id = ?", tbl.ID).Find(&deletedRecords).Error)
	for _, r := range deletedRecords {
		assert.NotNil(t, r.DeletedAt, "record %s should be soft-deleted", r.ID)
	}

	var deletedFields []models.Field
	require.NoError(t, db.Unscoped().Where("table_id = ?", tbl.ID).Find(&deletedFields).Error)
	for _, f := range deletedFields {
		assert.NotNil(t, f.DeletedAt, "field %s should be soft-deleted", f.ID)
	}

	var deletedTable models.Table
	require.NoError(t, db.Unscoped().Where("id = ?", tbl.ID).First(&deletedTable).Error)
	assert.NotNil(t, deletedTable.DeletedAt)

	var activeRecords []models.Record
	require.NoError(t, db.Where("table_id = ? AND deleted_at IS NULL", tbl.ID).Find(&activeRecords).Error)
	assert.Empty(t, activeRecords)

	var activeFields []models.Field
	require.NoError(t, db.Where("table_id = ? AND deleted_at IS NULL", tbl.ID).Find(&activeFields).Error)
	assert.Empty(t, activeFields)
}

func TestRollbackTable_DoesNotAffectOtherTables(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl1, _, _ := createTestTableWithRecords(t, db)

	dbModel2 := &models.Database{Name: "other_db"}
	require.NoError(t, db.Create(dbModel2).Error)
	tbl2 := &models.Table{DatabaseID: dbModel2.ID, Name: "other_orders"}
	require.NoError(t, db.Create(tbl2).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl2.ID, Name: "id", Type: "number", Required: true}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl2.ID, Data: `{"id":99}`, Version: 1}).Error)

	err := runner.rollbackTable(tbl1.ID)
	require.NoError(t, err)

	var activeRecords []models.Record
	require.NoError(t, db.Where("table_id = ? AND deleted_at IS NULL", tbl2.ID).Find(&activeRecords).Error)
	assert.Len(t, activeRecords, 1)

	var activeFields []models.Field
	require.NoError(t, db.Where("table_id = ? AND deleted_at IS NULL", tbl2.ID).Find(&activeFields).Error)
	assert.Len(t, activeFields, 1)
}

func TestRollbackTable_AlreadyDeletedIsNoop(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, records := createTestTableWithRecords(t, db)

	require.NoError(t, runner.rollbackTable(tbl.ID))
	err := runner.rollbackTable(tbl.ID)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", tbl.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)

	var totalUnscoped int64
	require.NoError(t, db.Unscoped().Model(&models.Record{}).Where("table_id = ?", tbl.ID).Count(&totalUnscoped).Error)
	assert.Equal(t, int64(len(records)), totalUnscoped)
}

func TestPickStrategyWithSource_ForceOffset(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Data.PaginationStrategy = PaginationOffset

	schema := &source.TableSchema{PrimaryKey: []string{"id"}}
	result := runner.pickStrategyWithSource(nil, "users", schema)
	assert.Equal(t, source.StrategyOffset, result)
}

func TestPickStrategyWithSource_CursorColumnOverride(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Data.CursorColumn = "created_at"
	runner.cfg.Data.PaginationStrategy = PaginationCursor

	schema := &source.TableSchema{PrimaryKey: []string{"id"}}
	result := runner.pickStrategyWithSource(nil, "users", schema)
	assert.Equal(t, source.StrategyCursor, result)
}

func TestPickStrategyWithSource_NilSourceWithPrimaryKey(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{PrimaryKey: []string{"id"}}
	result := runner.pickStrategyWithSource(nil, "users", schema)
	assert.Equal(t, source.StrategyCursor, result)
}

func TestPickStrategyWithSource_NilSourceWithUniqueKey(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{
		UniqueKeys: [][]string{{"email"}},
	}
	result := runner.pickStrategyWithSource(nil, "users", schema)
	assert.Equal(t, source.StrategyCursor, result)
}

func TestPickStrategyWithSource_NilSourceNoKeyFallsBackToOffset(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{}
	result := runner.pickStrategyWithSource(nil, "users", schema)
	assert.Equal(t, source.StrategyOffset, result)
}

func TestPickStrategyWithSource_NilSourceCompositePKFallsBack(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{PrimaryKey: []string{"tenant_id", "user_id"}}
	result := runner.pickStrategyWithSource(nil, "users", schema)
	assert.Equal(t, source.StrategyOffset, result)
}

func TestPickStrategyWithSource_NilSourceCompositeUniqueFallsBack(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{
		UniqueKeys: [][]string{{"col_a", "col_b"}},
	}
	result := runner.pickStrategyWithSource(nil, "users", schema)
	assert.Equal(t, source.StrategyOffset, result)
}

func TestPickStrategyWithSource_DelegatesToSource(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"users": {PrimaryKey: []string{"id"}},
		},
	}
	result := runner.pickStrategyWithSource(src, "users", src.schemas["users"])
	assert.Equal(t, source.StrategyCursor, result)
}

func TestPickStrategyWithSource_SourceRecommendsOffset(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"logs": {},
		},
	}
	result := runner.pickStrategyWithSource(src, "logs", src.schemas["logs"])
	assert.Equal(t, source.StrategyOffset, result)
}

func TestResolveCursorColumn_ConfigOverride(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Data.CursorColumn = "custom_cursor"

	schema := &source.TableSchema{PrimaryKey: []string{"id"}}
	assert.Equal(t, "custom_cursor", runner.resolveCursorColumn(schema))
}

func TestResolveCursorColumn_PrimaryKeyFallback(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{PrimaryKey: []string{"id"}}
	assert.Equal(t, "id", runner.resolveCursorColumn(schema))
}

func TestResolveCursorColumn_UniqueKeyFallback(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{
		UniqueKeys: [][]string{{"email"}},
	}
	assert.Equal(t, "email", runner.resolveCursorColumn(schema))
}

func TestResolveCursorColumn_NoCandidate(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{}
	assert.Equal(t, "", runner.resolveCursorColumn(schema))
}

func TestResolveCursorColumn_CompositePKSkipped(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{PrimaryKey: []string{"a", "b"}}
	assert.Equal(t, "", runner.resolveCursorColumn(schema))
}

func TestResolveCursorColumn_CompositeUniqueSkipped(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{
		UniqueKeys: [][]string{{"a", "b"}, {"email"}},
	}
	assert.Equal(t, "email", runner.resolveCursorColumn(schema))
}

func TestResolveCursorColumn_PKPreferredOverUnique(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	schema := &source.TableSchema{
		PrimaryKey: []string{"id"},
		UniqueKeys: [][]string{{"email"}},
	}
	assert.Equal(t, "id", runner.resolveCursorColumn(schema))
}

func TestResolveCursorColumn_WhitespaceOnlyOverrideIgnored(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Data.CursorColumn = "   "
	schema := &source.TableSchema{PrimaryKey: []string{"id"}}
	assert.Equal(t, "id", runner.resolveCursorColumn(schema))
}

func TestTargetTableName_RenameHit(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Tables.Rename = map[string]string{"source_tbl": "target_tbl"}
	assert.Equal(t, "target_tbl", runner.targetTableName("source_tbl"))
}

func TestTargetTableName_RenameWhitespaceFallsBack(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Tables.Rename = map[string]string{"source_tbl": "   "}
	assert.Equal(t, "source_tbl", runner.targetTableName("source_tbl"))
}

func TestTargetTableName_NoRenameReturnsSource(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Tables.Rename = map[string]string{}
	assert.Equal(t, "users", runner.targetTableName("users"))
}

func TestTargetTableName_OtherRenamesUnaffected(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Tables.Rename = map[string]string{"orders": "legacy_orders"}
	assert.Equal(t, "users", runner.targetTableName("users"))
	assert.Equal(t, "legacy_orders", runner.targetTableName("orders"))
}

func TestSourceDatabaseName_ExplicitName(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Source.Database = "my_shop"
	assert.Equal(t, "my_shop", runner.sourceDatabaseName())
}

func TestSourceDatabaseName_SqliteFallback(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Source.Type = "sqlite"
	runner.cfg.Source.Database = ""
	runner.cfg.Target.DatabaseName = "fallback_target"
	assert.Equal(t, "fallback_target", runner.sourceDatabaseName())
}

func TestSourceDatabaseName_EmptyForNonSqlite(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Source.Type = "mysql"
	runner.cfg.Source.Database = ""
	runner.cfg.Target.DatabaseName = "some_target"
	assert.Equal(t, "", runner.sourceDatabaseName())
}

func TestSourceDatabaseName_WhitespaceOnlyIgnored(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.cfg.Source.Database = "   "
	runner.cfg.Source.Type = "mysql"
	assert.Equal(t, "", runner.sourceDatabaseName())
}

func TestFetchTargetPayloadByCursor_FindsMatch(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, _ := createTestTableWithRecords(t, db)

	payload, err := runner.fetchTargetPayloadByCursor(tbl.ID, "id", float64(2))
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Equal(t, float64(2), payload["id"])
	assert.Equal(t, "shipped", payload["status"])
}

func TestFetchTargetPayloadByCursor_NotFound(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, _ := createTestTableWithRecords(t, db)

	payload, err := runner.fetchTargetPayloadByCursor(tbl.ID, "id", float64(999))
	require.NoError(t, err)
	assert.Nil(t, payload)
}

func TestFetchTargetPayloadByCursor_StringField(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, _ := createTestTableWithRecords(t, db)

	payload, err := runner.fetchTargetPayloadByCursor(tbl.ID, "status", "pending")
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Equal(t, float64(1), payload["id"])
	assert.Equal(t, "pending", payload["status"])
}

func TestFetchTargetPayloadByCursor_EmptyTable(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	dbModel := &models.Database{Name: "empty_cursor_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "empty"}
	require.NoError(t, db.Create(tbl).Error)

	payload, err := runner.fetchTargetPayloadByCursor(tbl.ID, "id", float64(1))
	require.NoError(t, err)
	assert.Nil(t, payload)
}

func TestRecordExists_True(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, _ := createTestTableWithRecords(t, db)

	exists, err := runner.recordExists(tbl.ID, "id", float64(1))
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRecordExists_False(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, _ := createTestTableWithRecords(t, db)

	exists, err := runner.recordExists(tbl.ID, "id", float64(999))
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRecordExists_ByStringField(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	_, tbl, _, _ := createTestTableWithRecords(t, db)

	exists, err := runner.recordExists(tbl.ID, "status", "shipped")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = runner.recordExists(tbl.ID, "status", "unknown")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRecordExists_EmptyTable(t *testing.T) {
	runner, db := setupRunnerWithDB(t)
	dbModel := &models.Database{Name: "empty_exists_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "empty"}
	require.NoError(t, db.Create(tbl).Error)

	exists, err := runner.recordExists(tbl.ID, "id", float64(1))
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestEnsureSource_AlreadyInitialized(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	fake := &fakeSource{tables: []string{"users"}}
	runner.src = fake

	err := runner.ensureSource()
	require.NoError(t, err)
	assert.Same(t, fake, runner.src)
}

func TestEnsureSource_FactoryError(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	factoryErr := errors.New("factory unavailable")
	runner.sourceFactory = func() (source.Source, error) {
		return nil, factoryErr
	}
	runner.src = nil

	err := runner.ensureSource()
	assert.Equal(t, factoryErr, err)
}

func TestEnsureSource_ConnectError(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.sourceFactory = func() (source.Source, error) {
		return &failingConnectSource{}, nil
	}
	runner.src = nil

	err := runner.ensureSource()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MIG-001")
}

func TestEnsureSource_Success(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.src = nil

	src, err := NewRunner(runner.db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           100,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			CheckpointInterval: 100,
			RollbackOnFailure:  RollbackTable,
		},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	err = src.ensureSource()
	require.NoError(t, err)
	assert.NotNil(t, src.src)
}

func TestCloseSource_NilNoop(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	runner.src = nil
	runner.closeSource()
	assert.Nil(t, runner.src)
}

func TestCloseSource_SetsNil(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	fake := &fakeSource{tables: []string{"users"}}
	runner.src = fake
	runner.closeSource()
	assert.Nil(t, runner.src)
	assert.True(t, fake.closed)
}

type failingConnectSource struct {
	fakeSource
}

func (f *failingConnectSource) Connect(string) error {
	return errors.New("connection refused")
}
