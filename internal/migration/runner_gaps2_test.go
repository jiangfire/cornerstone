package migration

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/migration/source"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func TestRunTable_SchemaOnlyMode(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled:             false,
			BatchSize:           100,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			CheckpointInterval: 100,
			RollbackOnFailure:  RollbackNone,
		},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	runner.src = &fakeSource{
		tables: []string{"users"},
		schemas: map[string]*source.TableSchema{
			"users": {
				Name:        "users",
				Columns:     []source.ColumnSchema{{Name: "id", Type: "INTEGER", IsPrimaryKey: true}, {Name: "name", Type: "TEXT"}},
				PrimaryKey:  []string{"id"},
				RowEstimate: 10,
			},
		},
		rows: map[string][]map[string]interface{}{
			"users": {{"id": int64(1), "name": "Alice"}, {"id": int64(2), "name": "Bob"}},
		},
	}
	runner.state = MigrationState{MigrationID: "test_mig", Tables: map[string]TableState{}}

	database := &models.Database{Name: "schemaonly_db"}
	require.NoError(t, db.Create(database).Error)

	report, err := runner.runTable(database.ID, PreviewTablePlan{
		SourceTable: "users", TargetTable: "users", Fields: 2, EstimatedRows: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, TableStatusCompleted, report.Status)
	assert.Equal(t, 2, report.FieldsCreated)
	assert.Equal(t, int64(0), report.RecordsInserted)
}

func TestRunTable_WithDataImport(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           100,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	runner.src = &fakeSource{
		tables: []string{"items"},
		schemas: map[string]*source.TableSchema{
			"items": {
				Name:        "items",
				Columns:     []source.ColumnSchema{{Name: "id", Type: "INTEGER", IsPrimaryKey: true}, {Name: "val", Type: "TEXT"}},
				PrimaryKey:  []string{"id"},
				RowEstimate: 2,
			},
		},
		rows: map[string][]map[string]interface{}{
			"items": {{"id": int64(1), "val": "a"}, {"id": int64(2), "val": "b"}},
		},
	}
	runner.state = MigrationState{MigrationID: "test_mig", Tables: map[string]TableState{}}

	database := &models.Database{Name: "data_import_db"}
	require.NoError(t, db.Create(database).Error)

	report, err := runner.runTable(database.ID, PreviewTablePlan{
		SourceTable: "items", TargetTable: "items", Fields: 2, EstimatedRows: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, TableStatusCompleted, report.Status)
	assert.Equal(t, int64(2), report.RecordsInserted)
}

func TestRunTable_GetSchemaError(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	runner.src = &schemaErrorSource{}
	runner.state = MigrationState{MigrationID: "test_mig", Tables: map[string]TableState{}}

	database := &models.Database{Name: "schemafail_db"}
	require.NoError(t, db.Create(database).Error)

	_, err = runner.runTable(database.ID, PreviewTablePlan{
		SourceTable: "missing", TargetTable: "missing", Fields: 1, EstimatedRows: 0,
	})
	require.Error(t, err)
}

func TestRunTable_RollbackOnDataError(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	runner.src = &errorAfterSchemaSource{fakeSource{
		tables: []string{"fail_tbl"},
		schemas: map[string]*source.TableSchema{
			"fail_tbl": {
				Name: "fail_tbl", Columns: []source.ColumnSchema{{Name: "id", Type: "INTEGER", IsPrimaryKey: true}},
				PrimaryKey: []string{"id"}, RowEstimate: 1,
			},
		},
	}}
	runner.state = MigrationState{MigrationID: "test_mig", Tables: map[string]TableState{}}

	database := &models.Database{Name: "rollback_err_db"}
	require.NoError(t, db.Create(database).Error)

	report, err := runner.runTable(database.ID, PreviewTablePlan{
		SourceTable: "fail_tbl", TargetTable: "fail_tbl", Fields: 1, EstimatedRows: 1,
	})
	require.Error(t, err)
	assert.Equal(t, TableStatusFailed, report.Status)

	var activeTables []models.Table
	require.NoError(t, db.Where("name = ? AND deleted_at IS NULL", "fail_tbl").Find(&activeTables).Error)
	assert.Empty(t, activeTables)
}

func TestValidateTable_RowCountMismatch(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "val_mismatch_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "orders"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "id", Type: "number", Required: true}).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "status", Type: "string"}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"id":1,"status":"pending"}`, Version: 1}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"id":2,"status":"shipped"}`, Version: 1}).Error)

	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"orders": {
				Name: "orders",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					{Name: "status", Type: "TEXT"},
				},
				PrimaryKey:  []string{"id"},
				RowEstimate: 5,
			},
		},
		rows: map[string][]map[string]interface{}{
			"orders": {
				{"id": int64(1), "status": "pending"},
				{"id": int64(2), "status": "shipped"},
				{"id": int64(3), "status": "delivered"},
				{"id": int64(4), "status": "cancelled"},
				{"id": int64(5), "status": "returned"},
			},
		},
	}
	runner.src = src

	report, err := runner.validateTable(tbl.ID, PreviewTablePlan{
		SourceTable: "orders", TargetTable: "orders",
	}, src.schemas["orders"], src)
	require.NoError(t, err)
	assert.Equal(t, ValidationFailed, report.Status)
	assert.Equal(t, 1, report.TablesFailed)
	assert.False(t, report.Details[0].RowCountMatch)
}

func TestValidateTable_StructureMismatch_ExtraTargetField(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "struct_mismatch_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "products"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "id", Type: "number", Required: true}).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "extra_col", Type: "string"}).Error)

	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"products": {
				Name: "products",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					{Name: "name", Type: "TEXT"},
				},
				PrimaryKey:  []string{"id"},
				RowEstimate: 0,
			},
		},
		rows: map[string][]map[string]interface{}{},
	}
	runner.src = src

	report, err := runner.validateTable(tbl.ID, PreviewTablePlan{
		SourceTable: "products", TargetTable: "products",
	}, src.schemas["products"], src)
	require.NoError(t, err)
	assert.False(t, report.Details[0].StructureMatch)
}

func TestValidateTable_AllMatch(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "val_ok_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "id", Type: "number", Required: true}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"id":1}`, Version: 1}).Error)

	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"items": {
				Name: "items",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
				},
				PrimaryKey:  []string{"id"},
				RowEstimate: 1,
			},
		},
		rows: map[string][]map[string]interface{}{
			"items": {{"id": int64(1)}},
		},
	}
	runner.src = src

	report, err := runner.validateTable(tbl.ID, PreviewTablePlan{
		SourceTable: "items", TargetTable: "items",
	}, src.schemas["items"], src)
	require.NoError(t, err)
	assert.True(t, report.Details[0].StructureMatch)
	assert.True(t, report.Details[0].RowCountMatch)
	assert.Equal(t, 1, report.TablesChecked)
}

func TestFinalizeValidation_AllPassed(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	report := runner.finalizeValidation([]MigrationTableReport{
		{Source: "t1", Target: "t1", Status: TableStatusCompleted, RecordsInserted: 10},
		{Source: "t2", Target: "t2", Status: TableStatusCompleted, RecordsInserted: 20},
	})
	assert.Equal(t, ValidationPassed, report.Status)
	assert.Equal(t, 2, report.TablesChecked)
	assert.Equal(t, 2, report.TablesPassed)
	assert.Equal(t, 0, report.TablesFailed)
	assert.Equal(t, 0, report.TablesWarnings)
}

func TestFinalizeValidation_WithErrors(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	report := runner.finalizeValidation([]MigrationTableReport{
		{Source: "t1", Target: "t1", Status: TableStatusCompleted, RecordsInserted: 10},
		{Source: "t2", Target: "t2", Status: TableStatusFailed, Error: "import failed"},
	})
	assert.Equal(t, ValidationFailed, report.Status)
	assert.Equal(t, 1, report.TablesPassed)
	assert.Equal(t, 1, report.TablesFailed)
	assert.False(t, report.Details[1].StructureMatch)
	assert.Equal(t, "import failed", report.Details[1].Warnings[0])
}

func TestFinalizeValidation_WithWarnings(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	report := runner.finalizeValidation([]MigrationTableReport{
		{Source: "t1", Target: "t1", Status: TableStatusCompleted, RecordsInserted: 5, Warnings: []string{"type fallback warning"}},
	})
	assert.Equal(t, ValidationPassedWithWarn, report.Status)
	assert.Equal(t, 1, report.TablesPassed)
	assert.Equal(t, 1, report.TablesWarnings)
	assert.Equal(t, "type fallback warning", report.Details[0].Warnings[0])
}

func TestFinalizeValidation_WithErrorsAndWarnings(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	report := runner.finalizeValidation([]MigrationTableReport{
		{Source: "t1", Target: "t1", Status: TableStatusCompleted, RecordsInserted: 5, Warnings: []string{"w1"}},
		{Source: "t2", Target: "t2", Status: TableStatusFailed, Error: "err"},
	})
	assert.Equal(t, ValidationFailed, report.Status)
	assert.Equal(t, 1, report.TablesPassed)
	assert.Equal(t, 1, report.TablesFailed)
}

func TestFinalizeValidation_EmptyTables(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	report := runner.finalizeValidation([]MigrationTableReport{})
	assert.Equal(t, ValidationPassed, report.Status)
	assert.Equal(t, 0, report.TablesChecked)
}

func TestFinalizeValidation_ErrorOverridesWarningsStatus(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	report := runner.finalizeValidation([]MigrationTableReport{
		{Source: "t1", Target: "t1", Status: TableStatusFailed, Error: "fatal"},
		{Source: "t2", Target: "t2", Status: TableStatusCompleted, Warnings: []string{"warn"}},
	})
	assert.Equal(t, ValidationFailed, report.Status)
	assert.Equal(t, 1, report.TablesFailed)
	assert.Equal(t, 1, report.TablesWarnings)
}

func TestFinalizeValidation_ErrorTableNoRowCountMatch(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	report := runner.finalizeValidation([]MigrationTableReport{
		{Source: "t1", Target: "t1", Status: TableStatusFailed, Error: "something broke"},
	})
	assert.False(t, report.Details[0].RowCountMatch)
}

func TestCompareColumnStats_NumericMismatch(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "stats_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "stats_tbl"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"amount":10.5}`, Version: 1}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"amount":20.0}`, Version: 1}).Error)

	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"stats_tbl": {
				Name: "stats_tbl",
				Columns: []source.ColumnSchema{
					{Name: "amount", Type: "REAL"},
				},
				PrimaryKey:  []string{"amount"},
				RowEstimate: 3,
			},
		},
		rows: map[string][]map[string]interface{}{
			"stats_tbl": {
				{"amount": 10.5},
				{"amount": 20.0},
				{"amount": 30.0},
			},
		},
	}
	runner.src = src

	warnings, err := runner.compareColumnStats(tbl.ID, PreviewTablePlan{
		SourceTable: "stats_tbl", TargetTable: "stats_tbl",
	}, src.schemas["stats_tbl"], src)
	require.NoError(t, err)
	assert.NotEmpty(t, warnings)
}

func TestCompareColumnStats_DateMismatch(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source:  SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Mapping: MappingConfig{Overrides: map[string]string{"DATE": "date"}},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "date_stats_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "date_tbl"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"created":"2024-01-15"}`, Version: 1}).Error)

	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"date_tbl": {
				Name: "date_tbl",
				Columns: []source.ColumnSchema{
					{Name: "created", Type: "DATE"},
				},
				PrimaryKey:  []string{"created"},
				RowEstimate: 1,
			},
		},
		rows: map[string][]map[string]interface{}{
			"date_tbl": {{"created": "2024-06-15"}},
		},
	}
	runner.src = src

	warnings, err := runner.compareColumnStats(tbl.ID, PreviewTablePlan{
		SourceTable: "date_tbl", TargetTable: "date_tbl",
	}, src.schemas["date_tbl"], src)
	require.NoError(t, err)
	assert.NotEmpty(t, warnings)
}

func TestCompareColumnStats_MatchingData(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "match_stats_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "match_tbl"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"amount":10.5}`, Version: 1}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"amount":20.0}`, Version: 1}).Error)

	src := &fakeSource{
		schemas: map[string]*source.TableSchema{
			"match_tbl": {
				Name: "match_tbl",
				Columns: []source.ColumnSchema{
					{Name: "amount", Type: "REAL"},
				},
				PrimaryKey:  []string{"amount"},
				RowEstimate: 2,
			},
		},
		rows: map[string][]map[string]interface{}{
			"match_tbl": {{"amount": 10.5}, {"amount": 20.0}},
		},
	}
	runner.src = src

	warnings, err := runner.compareColumnStats(tbl.ID, PreviewTablePlan{
		SourceTable: "match_tbl", TargetTable: "match_tbl",
	}, src.schemas["match_tbl"], src)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestRollbackTable_NonExistentTable(t *testing.T) {
	runner, _ := setupRunnerWithDB(t)
	err := runner.rollbackTable("nonexistent_id")
	require.NoError(t, err)
}

func TestImportTableData_EmptySource(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "empty_src_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "empty_src"}
	require.NoError(t, db.Create(tbl).Error)

	runner.src = &fakeSource{
		schemas: map[string]*source.TableSchema{
			"empty_src": {
				Name: "empty_src",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
				},
				PrimaryKey:  []string{"id"},
				RowEstimate: 0,
			},
		},
		rows: map[string][]map[string]interface{}{"empty_src": {}},
	}

	err = runner.importTableData(tbl.ID, "empty_src", runner.src.(*fakeSource).schemas["empty_src"], &TableState{
		Status: TableStatusInProgress, CursorColumn: "id",
	})
	require.NoError(t, err)
}

func TestImportTableData_BatchOverflow(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 2, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "batch_overflow_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "batch_tbl"}
	require.NoError(t, db.Create(tbl).Error)

	rows := make([]map[string]interface{}, 5)
	for i := 0; i < 5; i++ {
		rows[i] = map[string]interface{}{"id": int64(i + 1), "val": "item"}
	}

	runner.src = &fakeSource{
		schemas: map[string]*source.TableSchema{
			"batch_tbl": {
				Name: "batch_tbl",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					{Name: "val", Type: "TEXT"},
				},
				PrimaryKey:  []string{"id"},
				RowEstimate: 5,
			},
		},
		rows: map[string][]map[string]interface{}{"batch_tbl": rows},
	}

	state := &TableState{Status: TableStatusInProgress, CursorColumn: "id"}
	err = runner.importTableData(tbl.ID, "batch_tbl", runner.src.(*fakeSource).schemas["batch_tbl"], state)
	require.NoError(t, err)
	assert.Equal(t, int64(5), state.ProcessedCount)

	var count int64
	require.NoError(t, db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", tbl.ID).Count(&count).Error)
	assert.Equal(t, int64(5), count)
}

func TestImportTableData_WithOffsetPagination(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 2, PaginationStrategy: PaginationOffset, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	database := &models.Database{Name: "offset_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "offset_tbl"}
	require.NoError(t, db.Create(tbl).Error)

	rows := make([]map[string]interface{}, 4)
	for i := 0; i < 4; i++ {
		rows[i] = map[string]interface{}{"id": int64(i + 1), "name": "name_a"}
	}

	runner.src = &fakeSource{
		schemas: map[string]*source.TableSchema{
			"offset_tbl": {
				Name: "offset_tbl",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					{Name: "name", Type: "TEXT"},
				},
				RowEstimate: 4,
			},
		},
		rows: map[string][]map[string]interface{}{"offset_tbl": rows},
	}

	state := &TableState{Status: TableStatusInProgress}
	err = runner.importTableData(tbl.ID, "offset_tbl", runner.src.(*fakeSource).schemas["offset_tbl"], state)
	require.NoError(t, err)
	assert.Equal(t, int64(4), state.ProcessedCount)

	var count int64
	require.NoError(t, db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", tbl.ID).Count(&count).Error)
	assert.Equal(t, int64(4), count)
}

func TestExecuteTablePlans_ContinueOnError(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{ContinueOnError: true, CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	runner.src = &selectiveErrorSource{
		fakeSource: fakeSource{
			tables: []string{"good_tbl", "bad_tbl"},
			schemas: map[string]*source.TableSchema{
				"good_tbl": {
					Name: "good_tbl",
					Columns: []source.ColumnSchema{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					},
					PrimaryKey:  []string{"id"},
					RowEstimate: 1,
				},
				"bad_tbl": {
					Name: "bad_tbl",
					Columns: []source.ColumnSchema{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					},
					PrimaryKey:  []string{"id"},
					RowEstimate: 1,
				},
			},
			rows: map[string][]map[string]interface{}{
				"good_tbl": {{"id": int64(1)}},
				"bad_tbl":  {{"id": int64(1)}},
			},
		},
		errorTables: map[string]bool{"bad_tbl": true},
	}
	runner.state = MigrationState{MigrationID: "test_mig", Tables: map[string]TableState{}}

	database := &models.Database{Name: "continue_db"}
	require.NoError(t, db.Create(database).Error)

	results := runner.executeTablePlans(database.ID, []PreviewTablePlan{
		{SourceTable: "good_tbl", TargetTable: "good_tbl", Fields: 1, EstimatedRows: 1},
		{SourceTable: "bad_tbl", TargetTable: "bad_tbl", Fields: 1, EstimatedRows: 1},
	})
	assert.Len(t, results, 2)
	assert.NoError(t, results[0].err)
	assert.Error(t, results[1].err)
}

func TestExecuteTablePlans_StopOnError(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{ContinueOnError: false, CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	runner.src = &selectiveErrorSource{
		fakeSource: fakeSource{
			tables: []string{"bad_tbl", "good_tbl"},
			schemas: map[string]*source.TableSchema{
				"bad_tbl": {
					Name: "bad_tbl",
					Columns: []source.ColumnSchema{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					},
					PrimaryKey: []string{"id"},
				},
				"good_tbl": {
					Name: "good_tbl",
					Columns: []source.ColumnSchema{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					},
					PrimaryKey: []string{"id"},
				},
			},
			rows: map[string][]map[string]interface{}{
				"bad_tbl":  {{"id": int64(1)}},
				"good_tbl": {{"id": int64(1)}},
			},
		},
		errorTables: map[string]bool{"bad_tbl": true},
	}
	runner.state = MigrationState{MigrationID: "test_mig", Tables: map[string]TableState{}}

	database := &models.Database{Name: "stop_db"}
	require.NoError(t, db.Create(database).Error)

	results := runner.executeTablePlans(database.ID, []PreviewTablePlan{
		{SourceTable: "bad_tbl", TargetTable: "bad_tbl", Fields: 1, EstimatedRows: 1},
		{SourceTable: "good_tbl", TargetTable: "good_tbl", Fields: 1, EstimatedRows: 1},
	})
	assert.Len(t, results, 1)
	assert.Error(t, results[0].err)
}

func TestExecuteTablePlans_SkipsCompletedTables(t *testing.T) {
	db := testutil.SetupTestDBWithTokens(t, "master")
	runner, err := NewRunner(db, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: ":memory:"},
		Data: DataConfig{
			Enabled: true, BatchSize: 100, PaginationStrategy: PaginationCursor, MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackNone},
	}, RunnerOptions{StateDir: t.TempDir()})
	require.NoError(t, err)

	runner.src = &fakeSource{
		tables: []string{"done_tbl"},
		schemas: map[string]*source.TableSchema{
			"done_tbl": {
				Name: "done_tbl",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
		rows: map[string][]map[string]interface{}{},
	}
	runner.state = MigrationState{
		MigrationID: "test_mig",
		Tables: map[string]TableState{
			"done_tbl": {Status: TableStatusCompleted},
		},
	}

	database := &models.Database{Name: "skip_db"}
	require.NoError(t, db.Create(database).Error)
	tbl := &models.Table{DatabaseID: database.ID, Name: "done_tbl"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Record{TableID: tbl.ID, Data: `{"id":1}`, Version: 1}).Error)

	results := runner.executeTablePlans(database.ID, []PreviewTablePlan{
		{SourceTable: "done_tbl", TargetTable: "done_tbl", Fields: 1, EstimatedRows: 1},
	})
	require.Len(t, results, 1)
	assert.NoError(t, results[0].err)
	assert.Equal(t, int64(1), results[0].report.RecordsInserted)
}

type errorAfterSchemaSource struct {
	fakeSource
}

func (e *errorAfterSchemaSource) QueryRows(_ string, _ string, _ source.QueryOptions) ([]map[string]interface{}, error) {
	return nil, errors.New("simulated data read error")
}

type schemaErrorSource struct {
	fakeSource
}

func (s *schemaErrorSource) GetTableSchema(_ string, _ string) (*source.TableSchema, error) {
	return nil, errors.New("schema not found")
}

type selectiveErrorSource struct {
	fakeSource
	errorTables map[string]bool
}

func (s *selectiveErrorSource) QueryRows(_ string, tableName string, opts source.QueryOptions) ([]map[string]interface{}, error) {
	if s.errorTables[tableName] {
		return nil, errors.New("simulated table error")
	}
	return s.fakeSource.QueryRows("", tableName, opts)
}
