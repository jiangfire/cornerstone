package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func TestRunnerIntegration_MySQL(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("MYSQL_TEST_DSN not set, skipping MySQL migration integration test")
	}

	runRunnerIntegrationTest(t, "mysql", dsn)
}

func TestRunnerIntegration_Postgres(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("POSTGRES_TEST_DSN not set, skipping PostgreSQL migration integration test")
	}

	runRunnerIntegrationTest(t, "postgres", dsn)
}

func runRunnerIntegrationTest(t *testing.T, sourceType, dsn string) {
	targetDB := testutil.SetupTestDBWithTokens(t, "master")
	srcDB := openSourceDB(t, sourceType, dsn)

	tableName := fmt.Sprintf("mig_%s_%d", sourceType, time.Now().UnixNano())
	createSourceFixture(t, sourceType, srcDB, tableName)
	t.Cleanup(func() {
		dropSourceFixture(t, sourceType, srcDB, tableName)
	})

	cfg := Config{
		Source: SourceConfig{
			Type: sourceType,
			DSN:  dsn,
		},
		Target: TargetConfig{
			DatabaseName: fmt.Sprintf("target_%s", sourceType),
		},
		Tables: TablesConfig{
			Include: []string{tableName},
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
	require.Len(t, plan.Tables, 1)
	assert.Equal(t, tableName, plan.Tables[0].SourceTable)

	report, err := runner.Run()
	require.NoError(t, err)
	assert.Contains(t, []string{StatusCompleted, StatusCompletedWithIssues}, report.Status)
	assert.Equal(t, 1, report.Summary.TablesSuccess)
	assert.Equal(t, int64(2), report.Summary.RecordsInserted)
	assert.Contains(t, []string{ValidationPassed, ValidationPassedWithWarn}, report.Validation.Status)

	var dbModel models.Database
	require.NoError(t, targetDB.Where("name = ?", cfg.Target.DatabaseName).First(&dbModel).Error)
	var table models.Table
	require.NoError(t, targetDB.Where("database_id = ? AND name = ?", dbModel.ID, tableName).First(&table).Error)
	var records []models.Record
	require.NoError(t, targetDB.Where("table_id = ?", table.ID).Order("created_at asc").Find(&records).Error)
	require.Len(t, records, 2)

	payload := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(records[0].Data), &payload))
	assert.Contains(t, payload, "name")
	assert.Contains(t, payload, "active")
}

func openSourceDB(t *testing.T, sourceType, dsn string) *sql.DB {
	t.Helper()

	driver := map[string]string{
		"mysql":    "mysql",
		"postgres": "pgx",
	}[sourceType]
	db, err := sql.Open(driver, dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func createSourceFixture(t *testing.T, sourceType string, db *sql.DB, tableName string) {
	t.Helper()

	var statements []string
	switch sourceType {
	case "mysql":
		statements = []string{
			fmt.Sprintf(`CREATE TABLE %s (
  id BIGINT PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  active TINYINT(1) NOT NULL,
  created_at DATETIME NOT NULL
)`, tableName),
			fmt.Sprintf(`INSERT INTO %s (id, name, active, created_at) VALUES
  (1, 'alice', 1, '2026-05-31 10:00:00'),
  (2, 'bob', 0, '2026-05-31 11:00:00')`, tableName),
		}
	case "postgres":
		statements = []string{
			fmt.Sprintf(`CREATE TABLE %s (
  id BIGINT PRIMARY KEY,
  name TEXT NOT NULL,
  active BOOLEAN NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
)`, tableName),
			fmt.Sprintf(`INSERT INTO %s (id, name, active, created_at) VALUES
  (1, 'alice', true, '2026-05-31T10:00:00Z'),
  (2, 'bob', false, '2026-05-31T11:00:00Z')`, tableName),
		}
	default:
		t.Fatalf("unsupported source type: %s", sourceType)
	}

	for _, stmt := range statements {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}
}

func dropSourceFixture(t *testing.T, sourceType string, db *sql.DB, tableName string) {
	t.Helper()

	var stmt string
	switch sourceType {
	case "mysql":
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	case "postgres":
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	default:
		return
	}
	_, _ = db.Exec(stmt)
}
