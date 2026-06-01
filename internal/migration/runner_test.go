package migration

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/migration/source"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func TestRunnerPreviewAndRun(t *testing.T) {
	targetDB := testutil.SetupTestDBWithTokens(t, "master")
	sourcePath := buildSQLiteSourceFixture(t)

	cfg := Config{
		Source: SourceConfig{
			Type: "sqlite",
			DSN:  sourcePath,
		},
		Target: TargetConfig{
			DatabaseName: "imported_shop",
		},
		Tables: TablesConfig{
			Exclude: []string{"audit_logs"},
		},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           2,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			ValidateAfter:      true,
			CheckpointInterval: 1,
			RollbackOnFailure:  RollbackTable,
		},
	}

	runner, err := NewRunner(targetDB, "master", cfg, RunnerOptions{
		StateDir: t.TempDir(),
	})
	require.NoError(t, err)

	plan, err := runner.Preview()
	require.NoError(t, err)
	assert.Equal(t, "imported_shop", plan.TargetDatabase)
	require.Len(t, plan.Tables, 1)
	assert.Equal(t, "users", plan.Tables[0].SourceTable)
	assert.Equal(t, string(source.StrategyCursor), plan.Tables[0].MigrationStrategy)
	assert.Equal(t, int64(2), plan.TotalEstimatedRows)

	report, err := runner.Run()
	require.NoError(t, err)
	assert.Equal(t, runner.migrationID, report.MigrationID)
	assert.Equal(t, StatusCompleted, report.Status)
	assert.Equal(t, 1, report.Summary.TablesSuccess)
	assert.Equal(t, int64(2), report.Summary.RecordsInserted)
	assert.Equal(t, ValidationPassed, report.Validation.Status)

	var dbModel models.Database
	require.NoError(t, targetDB.Where("name = ?", "imported_shop").First(&dbModel).Error)

	var table models.Table
	require.NoError(t, targetDB.Where("database_id = ? AND name = ?", dbModel.ID, "users").First(&table).Error)

	var fields []models.Field
	require.NoError(t, targetDB.Where("table_id = ?", table.ID).Order("name asc").Find(&fields).Error)
	assert.Len(t, fields, 4)

	var records []models.Record
	require.NoError(t, targetDB.Where("table_id = ?", table.ID).Order("created_at asc").Find(&records).Error)
	require.Len(t, records, 2)

	payload := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(records[0].Data), &payload))
	assert.Equal(t, "alice", payload["name"])
	assert.Equal(t, "2026-05-31T10:00:00Z", payload["created_at"])
}

func TestRunnerResume_FromCheckpoint(t *testing.T) {
	targetDB := testutil.SetupTestDBWithTokens(t, "master")
	sourcePath := buildSQLiteSourceFixture(t)
	stateDir := t.TempDir()

	cfg := Config{
		Source: SourceConfig{
			Type: "sqlite",
			DSN:  sourcePath,
		},
		Target: TargetConfig{
			DatabaseName: "resume_shop",
		},
		Tables: TablesConfig{
			Exclude: []string{"audit_logs"},
		},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           2,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			ValidateAfter:      true,
			CheckpointInterval: 1,
			RollbackOnFailure:  RollbackTable,
		},
	}

	dbModel := &models.Database{Name: "resume_shop"}
	require.NoError(t, targetDB.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "users"}
	require.NoError(t, targetDB.Create(tbl).Error)
	for _, field := range []models.Field{
		{TableID: tbl.ID, Name: "id", Type: "number", Required: true},
		{TableID: tbl.ID, Name: "name", Type: "string", Required: true},
		{TableID: tbl.ID, Name: "active", Type: "number"},
		{TableID: tbl.ID, Name: "created_at", Type: "string"},
	} {
		require.NoError(t, targetDB.Create(&field).Error)
	}
	require.NoError(t, targetDB.Create(&models.Record{
		TableID: tbl.ID,
		Data:    `{"id":1,"name":"alice","active":1,"created_at":"2026-05-31T10:00:00Z"}`,
		Version: 1,
	}).Error)

	state := MigrationState{
		MigrationID: "mig_resume",
		Source:      "sqlite:" + sourcePath,
		TargetDB:    "resume_shop",
		Tables: map[string]TableState{
			"users": {
				Status:         TableStatusInProgress,
				CursorColumn:   "id",
				CursorValue:    float64(1),
				ProcessedCount: 1,
				TotalEstimate:  2,
			},
		},
	}
	store := NewStateStore(stateDir)
	require.NoError(t, store.Save(state))

	runner, err := NewRunner(targetDB, "master", cfg, RunnerOptions{
		StateDir:    stateDir,
		ResumeID:    "mig_resume",
		MigrationID: "mig_resume",
	})
	require.NoError(t, err)

	report, err := runner.Run()
	require.NoError(t, err)
	assert.Equal(t, int64(2), report.Summary.RecordsInserted)

	var records []models.Record
	require.NoError(t, targetDB.Where("table_id = ?", tbl.ID).Find(&records).Error)
	assert.Len(t, records, 2)
}

func TestRunnerLoadOrInitState_SanitizesSourceDescriptor(t *testing.T) {
	runner, err := NewRunner(nil, "", Config{
		Source: SourceConfig{
			Type: "mysql",
			DSN:  "user:super-secret@tcp(localhost:3306)/shop?parseTime=true",
		},
		Target: TargetConfig{
			DatabaseName: "shop_target",
		},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           100,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			CheckpointInterval: 1,
			RollbackOnFailure:  RollbackTable,
		},
	}, RunnerOptions{
		StateDir: t.TempDir(),
	})
	require.NoError(t, err)

	state, err := runner.loadOrInitState(&compiledPlan{
		PreviewPlan: PreviewPlan{
			TargetDatabase: "shop_target",
			Tables: []PreviewTablePlan{
				{SourceTable: "users", EstimatedRows: 10},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "mysql:shop", state.Source)
	assert.NotContains(t, state.Source, "super-secret")
	assert.NotContains(t, state.Source, "localhost")
}

func buildSQLiteSourceFixture(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
CREATE TABLE users (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	active INTEGER,
	created_at TEXT
);
INSERT INTO users (id, name, active, created_at) VALUES
	(1, 'alice', 1, '2026-05-31T10:00:00Z'),
	(2, 'bob', 0, '2026-05-31T11:00:00Z');

CREATE TABLE audit_logs (
	id INTEGER PRIMARY KEY,
	message TEXT
);
INSERT INTO audit_logs (id, message) VALUES (1, 'skip me');
`)
	require.NoError(t, err)

	return path
}

func TestStateStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStateStore(dir)

	state := MigrationState{
		MigrationID: "mig_1",
		Source:      "sqlite:test.db",
		TargetDB:    "target",
		Tables: map[string]TableState{
			"users": {
				Status:         TableStatusInProgress,
				CursorColumn:   "id",
				CursorValue:    float64(10),
				ProcessedCount: 10,
			},
		},
	}

	require.NoError(t, store.Save(state))
	loaded, err := store.Load("mig_1")
	require.NoError(t, err)
	assert.Equal(t, state.MigrationID, loaded.MigrationID)
	assert.Equal(t, state.Tables["users"].ProcessedCount, loaded.Tables["users"].ProcessedCount)

	path := filepath.Join(dir, "mig_1.state.json")
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestStateStore_LoadCorruptFileReturnsCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mig_bad.state.json")
	require.NoError(t, os.WriteFile(path, []byte("{not-json"), 0o600))

	_, err := NewStateStore(dir).Load("mig_bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MIG-007")
}

func TestRunnerPreviewClosesSource(t *testing.T) {
	src := &fakeSource{
		tables: []string{"users"},
		schemas: map[string]*source.TableSchema{
			"users": {
				Name: "users",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
				},
				PrimaryKey:  []string{"id"},
				RowEstimate: 1,
			},
		},
	}

	runner, err := NewRunner(nil, "", Config{
		Source: SourceConfig{Type: "sqlite", DSN: "fake.db"},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           1,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			CheckpointInterval: 1,
			RollbackOnFailure:  RollbackTable,
		},
	}, RunnerOptions{
		SourceFactory: func() (source.Source, error) { return src, nil },
	})
	require.NoError(t, err)

	_, err = runner.Preview()
	require.NoError(t, err)
	assert.True(t, src.closed)
}

func TestRetryWithBackoffEventuallySucceeds(t *testing.T) {
	var attempts int32
	err := retryWithBackoff(3, time.Millisecond, func() error {
		if atomic.AddInt32(&attempts, 1) < 3 {
			return assert.AnError
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts)
}

func TestRunnerValidationDetectsContentMismatch(t *testing.T) {
	targetDB := testutil.SetupTestDBWithTokens(t, "master")
	src := &fakeSource{
		tables: []string{"users"},
		schemas: map[string]*source.TableSchema{
			"users": {
				Name: "users",
				Columns: []source.ColumnSchema{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					{Name: "name", Type: "TEXT"},
				},
				PrimaryKey:  []string{"id"},
				RowEstimate: 2,
			},
		},
		rows: map[string][]map[string]interface{}{
			"users": {
				{"id": int64(1), "name": "alice"},
				{"id": int64(2), "name": "bob"},
			},
		},
	}

	dbModel := &models.Database{Name: "validate_db"}
	require.NoError(t, targetDB.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "users"}
	require.NoError(t, targetDB.Create(tbl).Error)
	require.NoError(t, targetDB.Create(&models.Field{TableID: tbl.ID, Name: "id", Type: "number", Required: true}).Error)
	require.NoError(t, targetDB.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)
	require.NoError(t, targetDB.Create(&models.Record{TableID: tbl.ID, Data: `{"id":1,"name":"alice"}`, Version: 1}).Error)
	require.NoError(t, targetDB.Create(&models.Record{TableID: tbl.ID, Data: `{"id":2,"name":"wrong"}`, Version: 1}).Error)

	runner, err := NewRunner(targetDB, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: "fake.db"},
		Target: TargetConfig{DatabaseName: "validate_db"},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           10,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			ValidateAfter:      true,
			CheckpointInterval: 1,
			RollbackOnFailure:  RollbackTable,
		},
	}, RunnerOptions{
		SourceFactory: func() (source.Source, error) { return src, nil },
	})
	require.NoError(t, err)

	validation, err := runner.validateTable(tbl.ID, PreviewTablePlan{
		SourceTable:   "users",
		TargetTable:   "users",
		EstimatedRows: 2,
	}, src.schemas["users"], src)
	require.NoError(t, err)
	assert.Equal(t, ValidationFailed, validation.Status)
	require.NotEmpty(t, validation.Details)
	assert.Contains(t, strings.Join(validation.Details[0].Warnings, " "), "name")
}

func TestRunnerRunRespectsMaxConcurrentTables(t *testing.T) {
	targetDB := testutil.SetupTestDBWithTokens(t, "master")
	blocker := newBlockingSource(3)

	runner, err := NewRunner(targetDB, "master", Config{
		Source: SourceConfig{Type: "sqlite", DSN: "fake.db"},
		Target: TargetConfig{DatabaseName: "parallel_db"},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           1,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 2,
		},
		Options: OptionsConfig{
			ContinueOnError:    true,
			ValidateAfter:      false,
			CheckpointInterval: 1,
			RollbackOnFailure:  RollbackTable,
		},
	}, RunnerOptions{
		SourceFactory: func() (source.Source, error) { return blocker, nil },
	})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run()
		done <- runErr
	}()

	blocker.waitForStarts(t, 2)
	assert.Equal(t, int32(2), atomic.LoadInt32(&blocker.maxConcurrent))
	select {
	case <-blocker.startedThird:
		t.Fatal("third table started before a slot was released")
	case <-time.After(50 * time.Millisecond):
	}

	blocker.releaseOne()
	blocker.waitForStarts(t, 3)
	blocker.releaseAll()

	require.NoError(t, <-done)
}

type fakeSource struct {
	tables  []string
	schemas map[string]*source.TableSchema
	rows    map[string][]map[string]interface{}
	closed  bool
}

func (f *fakeSource) Connect(string) error { return nil }
func (f *fakeSource) Close() error         { f.closed = true; return nil }
func (f *fakeSource) ListDatabases() ([]string, error) {
	return []string{"fake"}, nil
}
func (f *fakeSource) ListTables(string) ([]string, error) {
	return append([]string{}, f.tables...), nil
}
func (f *fakeSource) GetTableSchema(_ string, tableName string) (*source.TableSchema, error) {
	return f.schemas[tableName], nil
}
func (f *fakeSource) EstimateRowCount(_ string, tableName string) (int64, error) {
	return f.schemas[tableName].RowEstimate, nil
}
func (f *fakeSource) QueryRows(_ string, tableName string, opts source.QueryOptions) ([]map[string]interface{}, error) {
	rows := f.rows[tableName]
	start := 0
	if opts.Strategy == source.StrategyCursor && opts.CursorValue != nil {
		cursor := opts.CursorValue.(int64)
		for idx, row := range rows {
			if row[opts.CursorColumn].(int64) > cursor {
				start = idx
				break
			}
			start = len(rows)
		}
	}
	if opts.Strategy == source.StrategyOffset {
		start = int(opts.Offset)
	}
	if start >= len(rows) {
		return []map[string]interface{}{}, nil
	}
	end := start + int(opts.Limit)
	if end > len(rows) {
		end = len(rows)
	}
	result := make([]map[string]interface{}, 0, end-start)
	for _, row := range rows[start:end] {
		cloned := make(map[string]interface{}, len(row))
		for k, v := range row {
			cloned[k] = v
		}
		result = append(result, cloned)
	}
	return result, nil
}
func (f *fakeSource) RecommendPaginationStrategy(_ string, tableName string) source.PaginationStrategy {
	if schema := f.schemas[tableName]; schema != nil && len(schema.PrimaryKey) == 1 {
		return source.StrategyCursor
	}
	return source.StrategyOffset
}

type blockingSource struct {
	fakeSource
	mu            sync.Mutex
	enterCount    int
	current       int32
	maxConcurrent int32
	releaseCh     chan struct{}
	startedThird  chan struct{}
}

func newBlockingSource(tableCount int) *blockingSource {
	tables := make([]string, 0, tableCount)
	schemas := map[string]*source.TableSchema{}
	for idx := 1; idx <= tableCount; idx++ {
		name := "t" + strconv.Itoa(idx)
		tables = append(tables, name)
		schemas[name] = &source.TableSchema{
			Name:       name,
			Columns:    []source.ColumnSchema{{Name: "id", Type: "INTEGER", IsPrimaryKey: true}},
			PrimaryKey: []string{"id"},
		}
	}
	return &blockingSource{
		fakeSource: fakeSource{
			tables:  tables,
			schemas: schemas,
			rows:    map[string][]map[string]interface{}{},
		},
		releaseCh:    make(chan struct{}, tableCount),
		startedThird: make(chan struct{}, 1),
	}
}

func (b *blockingSource) QueryRows(_ string, tableName string, opts source.QueryOptions) ([]map[string]interface{}, error) {
	cur := atomic.AddInt32(&b.current, 1)
	for {
		prev := atomic.LoadInt32(&b.maxConcurrent)
		if cur <= prev || atomic.CompareAndSwapInt32(&b.maxConcurrent, prev, cur) {
			break
		}
	}
	b.mu.Lock()
	b.enterCount++
	if b.enterCount == 3 {
		b.startedThird <- struct{}{}
	}
	b.mu.Unlock()

	<-b.releaseCh
	atomic.AddInt32(&b.current, -1)
	return []map[string]interface{}{}, nil
}

func (b *blockingSource) waitForStarts(t *testing.T, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.enterCount >= want
	}, time.Second, 10*time.Millisecond)
}

func (b *blockingSource) releaseOne() {
	b.releaseCh <- struct{}{}
}

func (b *blockingSource) releaseAll() {
	for i := 0; i < cap(b.releaseCh); i++ {
		select {
		case b.releaseCh <- struct{}{}:
		default:
		}
	}
}
