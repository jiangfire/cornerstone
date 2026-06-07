package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeValueForField_Boolean(t *testing.T) {
	assert.Equal(t, true, normalizeValueForField("boolean", true))
	assert.Equal(t, false, normalizeValueForField("boolean", false))
	assert.Equal(t, false, normalizeValueForField("boolean", int64(0)))
	assert.Equal(t, true, normalizeValueForField("boolean", int64(1)))
	assert.Equal(t, true, normalizeValueForField("boolean", int64(42)))
	assert.Equal(t, false, normalizeValueForField("boolean", float64(0)))
	assert.Equal(t, true, normalizeValueForField("boolean", float64(3.14)))
	assert.Equal(t, true, normalizeValueForField("boolean", "1"))
	assert.Equal(t, true, normalizeValueForField("boolean", "true"))
	assert.Equal(t, true, normalizeValueForField("boolean", "TRUE"))
	assert.Equal(t, true, normalizeValueForField("boolean", "True"))
	assert.Equal(t, false, normalizeValueForField("boolean", "0"))
	assert.Equal(t, false, normalizeValueForField("boolean", "false"))
	assert.Equal(t, nil, normalizeValueForField("boolean", nil))
}

func TestNormalizeValueForField_Date(t *testing.T) {
	assert.Equal(t, "hello", normalizeValueForField("date", "hello"))
	assert.Equal(t, "bytes_val", normalizeValueForField("date", []byte("bytes_val")))
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2024-06-15", normalizeValueForField("date", ts))
	assert.Equal(t, nil, normalizeValueForField("date", nil))
}

func TestNormalizeValueForField_Datetime(t *testing.T) {
	assert.Equal(t, "hello", normalizeValueForField("datetime", "hello"))
	assert.Equal(t, "bytes_val", normalizeValueForField("datetime", []byte("bytes_val")))
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2024-06-15T10:30:00Z", normalizeValueForField("datetime", ts))
	assert.Equal(t, nil, normalizeValueForField("datetime", nil))
}

func TestNormalizeValueForField_String(t *testing.T) {
	assert.Equal(t, "hello", normalizeValueForField("string", "hello"))
	assert.Equal(t, "bytes_val", normalizeValueForField("string", []byte("bytes_val")))
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2024-06-15T10:30:00Z", normalizeValueForField("string", ts))
	assert.Equal(t, nil, normalizeValueForField("string", nil))
}

func TestNormalizeValueForField_Text(t *testing.T) {
	assert.Equal(t, "hello", normalizeValueForField("text", "hello"))
	assert.Equal(t, "bytes_val", normalizeValueForField("text", []byte("bytes_val")))
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2024-06-15T10:30:00Z", normalizeValueForField("text", ts))
}

func TestNormalizeValueForField_JSON(t *testing.T) {
	parsed := normalizeValueForField("json", `{"key":"val"}`)
	m, ok := parsed.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "val", m["key"])

	assert.Equal(t, "not json", normalizeValueForField("json", "not json"))
	assert.Equal(t, 42, normalizeValueForField("json", 42))
	assert.Equal(t, nil, normalizeValueForField("json", nil))
}

func TestNormalizeValueForField_Default(t *testing.T) {
	assert.Equal(t, 42, normalizeValueForField("number", 42))
	assert.Equal(t, "x", normalizeValueForField("unknown_type", "x"))
	assert.Equal(t, nil, normalizeValueForField("whatever", nil))
}

func TestToFloat64(t *testing.T) {
	v, ok := toFloat64(float64(3.14))
	assert.True(t, ok)
	assert.Equal(t, 3.14, v)

	v, ok = toFloat64(float32(2.5))
	assert.True(t, ok)
	assert.Equal(t, float64(float32(2.5)), v)

	v, ok = toFloat64(int(7))
	assert.True(t, ok)
	assert.Equal(t, 7.0, v)

	v, ok = toFloat64(int64(100))
	assert.True(t, ok)
	assert.Equal(t, 100.0, v)

	v, ok = toFloat64(int32(50))
	assert.True(t, ok)
	assert.Equal(t, 50.0, v)

	v, ok = toFloat64(json.Number("3.14"))
	assert.True(t, ok)
	assert.InDelta(t, 3.14, v, 0.001)

	v, ok = toFloat64("3.14")
	assert.True(t, ok)
	assert.InDelta(t, 3.14, v, 0.001)

	_, ok = toFloat64("not a number")
	assert.False(t, ok)

	_, ok = toFloat64(true)
	assert.False(t, ok)

	_, ok = toFloat64(nil)
	assert.False(t, ok)
}

func TestComputeSampleSize(t *testing.T) {
	assert.Equal(t, 0, computeSampleSize(0))
	assert.Equal(t, 0, computeSampleSize(-5))
	assert.Equal(t, 1, computeSampleSize(1))
	assert.Equal(t, 10, computeSampleSize(10))
	assert.Equal(t, 20, computeSampleSize(20))
	assert.Equal(t, 5, computeSampleSize(100))
	assert.Equal(t, 10, computeSampleSize(400))
	assert.Equal(t, 10, computeSampleSize(10000))
	assert.Equal(t, 10, computeSampleSize(1000000))
	assert.Equal(t, 1, computeSampleSize(21))
}

func TestFirstDifferentField(t *testing.T) {
	assert.Equal(t, "", firstDifferentField(
		map[string]interface{}{"a": 1, "b": "x"},
		map[string]interface{}{"a": 1, "b": "x"},
	))

	assert.Equal(t, "c", firstDifferentField(
		map[string]interface{}{"a": 1, "c": 3},
		map[string]interface{}{"a": 1},
	))

	assert.Equal(t, "a", firstDifferentField(
		map[string]interface{}{"a": 1},
		map[string]interface{}{"a": 2},
	))
}

func TestJsonLikeEqual(t *testing.T) {
	assert.True(t, jsonLikeEqual("hello", "hello"))
	assert.True(t, jsonLikeEqual(42, 42))
	assert.True(t, jsonLikeEqual(map[string]interface{}{"a": 1}, map[string]interface{}{"a": float64(1)}))

	assert.False(t, jsonLikeEqual("a", "b"))
	assert.False(t, jsonLikeEqual(1, 2))

	ch := make(chan int)
	assert.False(t, jsonLikeEqual(ch, "x"))
}

func TestSumNumericFieldFromMaps(t *testing.T) {
	rows := []map[string]interface{}{
		{"amount": 10.5},
		{"amount": 20.0},
		{"amount": 30.5},
	}
	sum, count := sumNumericFieldFromMaps(rows, "amount")
	assert.InDelta(t, 61.0, sum, 0.001)
	assert.Equal(t, 3, count)

	sum, count = sumNumericFieldFromMaps(rows, "missing")
	assert.Equal(t, 0.0, sum)
	assert.Equal(t, 0, count)

	rowsWithBad := []map[string]interface{}{
		{"amount": "not a number"},
		{"amount": 5.0},
	}
	sum, count = sumNumericFieldFromMaps(rowsWithBad, "amount")
	assert.Equal(t, 5.0, sum)
	assert.Equal(t, 1, count)
}

func TestMinMaxStringFieldFromMaps(t *testing.T) {
	rows := []map[string]interface{}{
		{"d": "2024-06-15"},
		{"d": "2024-01-01"},
		{"d": "2024-12-31"},
	}
	gotMin, gotMax := minMaxStringFieldFromMaps(rows, "d")
	assert.Equal(t, "2024-01-01", gotMin)
	assert.Equal(t, "2024-12-31", gotMax)

	gotMin, gotMax = minMaxStringFieldFromMaps(rows, "missing")
	assert.Equal(t, "", gotMin)
	assert.Equal(t, "", gotMax)

	rowsWithEmpty := []map[string]interface{}{
		{"d": "alpha"},
		{"d": ""},
		{"d": "   "},
		{"d": "zeta"},
	}
	gotMin, gotMax = minMaxStringFieldFromMaps(rowsWithEmpty, "d")
	assert.Equal(t, "alpha", gotMin)
	assert.Equal(t, "zeta", gotMax)

	gotMin, gotMax = minMaxStringFieldFromMaps([]map[string]interface{}{}, "d")
	assert.Equal(t, "", gotMin)
	assert.Equal(t, "", gotMax)
}

func TestMysqlDatabaseNameFromDSN(t *testing.T) {
	assert.Equal(t, "", mysqlDatabaseNameFromDSN("noslash"))
	assert.Equal(t, "mydb", mysqlDatabaseNameFromDSN("user:pass@tcp(localhost:3306)/mydb"))
	assert.Equal(t, "mydb", mysqlDatabaseNameFromDSN("user:pass@tcp(localhost:3306)/mydb?parseTime=true"))
	assert.Equal(t, "my db", mysqlDatabaseNameFromDSN("user:pass@tcp(localhost:3306)/my db"))
}

func TestPostgresDatabaseNameFromDSN(t *testing.T) {
	assert.Equal(t, "mydb", postgresDatabaseNameFromDSN("host=localhost port=5432 dbname=mydb user=postgres"))
	assert.Equal(t, "", postgresDatabaseNameFromDSN("host=localhost port=5432 user=postgres"))
	assert.Equal(t, "", postgresDatabaseNameFromDSN(""))
}

func TestNormalizeCursorValue(t *testing.T) {
	assert.Equal(t, int64(5), normalizeCursorValue(float64(5)))
	assert.Equal(t, int64(0), normalizeCursorValue(float64(0)))
	assert.Equal(t, 3.14, normalizeCursorValue(float64(3.14)))
	assert.Equal(t, "hello", normalizeCursorValue("hello"))
	assert.Equal(t, nil, normalizeCursorValue(nil))
}

func TestMinInt(t *testing.T) {
	assert.Equal(t, 3, minInt(3, 7))
	assert.Equal(t, 2, minInt(5, 2))
	assert.Equal(t, 4, minInt(4, 4))
}

func TestMigrationError_Error(t *testing.T) {
	var e *MigrationError
	assert.Equal(t, "", e.Error())

	e = &MigrationError{Code: "MIG-001", Message: "connection failed"}
	assert.Equal(t, "MIG-001: connection failed", e.Error())

	cause := errors.New("dial tcp: connection refused")
	e = &MigrationError{Code: "MIG-001", Message: "connection failed", Cause: cause}
	assert.Equal(t, "MIG-001: connection failed: dial tcp: connection refused", e.Error())
}

func TestMigrationError_Unwrap(t *testing.T) {
	var e *MigrationError
	assert.Nil(t, e.Unwrap())

	cause := errors.New("root cause")
	e = &MigrationError{Code: "MIG-001", Message: "msg", Cause: cause}
	assert.Equal(t, cause, e.Unwrap())
}

func TestConfig_BuildSourceDSN(t *testing.T) {
	cfg := Config{Source: SourceConfig{DSN: "pre-built-dsn"}}
	assert.Equal(t, "pre-built-dsn", cfg.BuildSourceDSN())

	cfg = Config{Source: SourceConfig{Type: "sqlite", Database: "/path/to/db.sqlite"}}
	assert.Equal(t, "/path/to/db.sqlite", cfg.BuildSourceDSN())

	cfg = Config{Source: SourceConfig{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "pass",
		Database: "testdb",
	}}
	assert.Equal(t, "root:pass@tcp(localhost:3306)/testdb", cfg.BuildSourceDSN())

	cfg = Config{Source: SourceConfig{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "pass",
		Database: "testdb",
		Params:   map[string]string{"parseTime": "true", "charset": "utf8"},
	}}
	dsn := cfg.BuildSourceDSN()
	assert.Contains(t, dsn, "root:pass@tcp(localhost:3306)/testdb?")
	assert.Contains(t, dsn, "parseTime=true")
	assert.Contains(t, dsn, "charset=utf8")

	cfg = Config{Source: SourceConfig{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "pguser",
		Password: "pgpass",
		Database: "pgdb",
	}}
	dsn = cfg.BuildSourceDSN()
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=pguser")
	assert.Contains(t, dsn, "password=pgpass")
	assert.Contains(t, dsn, "dbname=pgdb")
	assert.Contains(t, dsn, "sslmode=disable")

	cfg = Config{Source: SourceConfig{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "pguser",
		Password: "pgpass",
		Database: "pgdb",
		Params:   map[string]string{"connect_timeout": "10"},
	}}
	dsn = cfg.BuildSourceDSN()
	assert.Contains(t, dsn, "connect_timeout=10")

	cfg = Config{Source: SourceConfig{Type: "oracle"}}
	assert.Equal(t, "", cfg.BuildSourceDSN())
}

func TestConfig_EffectiveTargetDatabase(t *testing.T) {
	cfg := Config{Target: TargetConfig{DatabaseName: "explicit_db"}}
	assert.Equal(t, "explicit_db", cfg.EffectiveTargetDatabase())

	cfg = Config{Source: SourceConfig{Database: "source_db"}}
	assert.Equal(t, "source_db", cfg.EffectiveTargetDatabase())

	cfg = Config{Source: SourceConfig{Type: "sqlite", DSN: "/data/my_project.db"}}
	assert.Equal(t, "my_project", cfg.EffectiveTargetDatabase())

	cfg = Config{}
	assert.Equal(t, "migration_target", cfg.EffectiveTargetDatabase())
}

func TestConfig_Validate_AllBranches(t *testing.T) {
	cfg := Config{
		Source: SourceConfig{Type: "mysql", DSN: "root@tcp(localhost)/db"},
		Data: DataConfig{
			BatchSize:           100,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			CheckpointInterval: 100,
			RollbackOnFailure:  RollbackTable,
		},
	}
	require.NoError(t, cfg.Validate())

	badType := cfg
	badType.Source = SourceConfig{Type: "oracle", DSN: "x"}
	require.Error(t, badType.Validate())
	assert.Contains(t, badType.Validate().Error(), "source.type")

	bothDSNAndFields := Config{
		Source:  SourceConfig{Type: "mysql", DSN: "dsn", Host: "h", Port: 3306, User: "u", Password: "p", Database: "d"},
		Data:    DataConfig{BatchSize: 100, MaxConcurrentTables: 1},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable},
	}
	err := bothDSNAndFields.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")

	neitherDSNNorFields := Config{
		Source:  SourceConfig{Type: "mysql"},
		Data:    DataConfig{BatchSize: 100, MaxConcurrentTables: 1},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable},
	}
	err = neitherDSNNorFields.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be provided")

	sqliteMissing := Config{
		Source:  SourceConfig{Type: "sqlite", Host: "localhost"},
		Data:    DataConfig{BatchSize: 100, MaxConcurrentTables: 1},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable},
	}
	err = sqliteMissing.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sqlite")

	nonSqliteMissingFields := Config{
		Source:  SourceConfig{Type: "mysql", Host: "localhost"},
		Data:    DataConfig{BatchSize: 100, MaxConcurrentTables: 1},
		Options: OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable},
	}
	err = nonSqliteMissingFields.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host/user/database")

	badBatch := cfg
	badBatch.Data = DataConfig{BatchSize: 0, MaxConcurrentTables: 1}
	badBatch.Options = OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable}
	err = badBatch.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch_size")

	badStrategy := cfg
	badStrategy.Data = DataConfig{
		BatchSize:           100,
		PaginationStrategy:  "invalid",
		MaxConcurrentTables: 1,
	}
	badStrategy.Options = OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable}
	err = badStrategy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pagination_strategy")

	badConcurrent := cfg
	badConcurrent.Data = DataConfig{BatchSize: 100, MaxConcurrentTables: 0}
	badConcurrent.Options = OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: RollbackTable}
	err = badConcurrent.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_concurrent_tables")

	badCheckpoint := cfg
	badCheckpoint.Data = DataConfig{BatchSize: 100, MaxConcurrentTables: 1}
	badCheckpoint.Options = OptionsConfig{CheckpointInterval: 0, RollbackOnFailure: RollbackTable}
	err = badCheckpoint.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checkpoint_interval")

	badRollback := cfg
	badRollback.Data = DataConfig{BatchSize: 100, MaxConcurrentTables: 1}
	badRollback.Options = OptionsConfig{CheckpointInterval: 100, RollbackOnFailure: "bad"}
	err = badRollback.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback_on_failure")
}

func TestConfig_ApplyTypeMapOverrideFile(t *testing.T) {
	cfg := Config{Mapping: MappingConfig{Overrides: map[string]string{}}}
	err := cfg.ApplyTypeMapOverrideFile("")
	assert.NoError(t, err)
	err = cfg.ApplyTypeMapOverrideFile("   ")
	assert.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"VARCHAR":"string","INT":"number"}`), 0o600))

	cfg = Config{Mapping: MappingConfig{Overrides: map[string]string{}}}
	err = cfg.ApplyTypeMapOverrideFile(path)
	require.NoError(t, err)
	assert.Equal(t, "string", cfg.Mapping.Overrides["VARCHAR"])
	assert.Equal(t, "number", cfg.Mapping.Overrides["INT"])

	cfg = Config{Mapping: MappingConfig{Overrides: nil}}
	err = cfg.ApplyTypeMapOverrideFile(path)
	require.NoError(t, err)
	assert.Equal(t, "string", cfg.Mapping.Overrides["VARCHAR"])

	err = cfg.ApplyTypeMapOverrideFile(filepath.Join(dir, "nonexistent.json"))
	assert.Error(t, err)

	badPath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(badPath, []byte(`not json`), 0o600))
	err = cfg.ApplyTypeMapOverrideFile(badPath)
	assert.Error(t, err)
}

func TestCheckConfigFileWarnings(t *testing.T) {
	assert.Nil(t, CheckConfigFileWarnings(""))
	assert.Nil(t, CheckConfigFileWarnings("   "))

	warnings := CheckConfigFileWarnings("/nonexistent/path/config.yaml")
	assert.NotNil(t, warnings)

	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o600))
	result := CheckConfigFileWarnings(path)
	if os.PathSeparator == '\\' {
		assert.Nil(t, result)
	} else {
		assert.Nil(t, result) // 0o600 has no group/other bits
	}
}

func TestRetryWithBackoff_AllAttemptsExhausted(t *testing.T) {
	callCount := 0
	err := retryWithBackoff(3, time.Millisecond, func() error {
		callCount++
		return fmt.Errorf("fail %d", callCount)
	})
	require.Error(t, err)
	assert.Equal(t, "fail 3", err.Error())
	assert.Equal(t, 3, callCount)
}

func TestRetryWithBackoff_SucceedsOnFirstTry(t *testing.T) {
	callCount := 0
	err := retryWithBackoff(3, time.Millisecond, func() error {
		callCount++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryWithBackoff_ZeroAttempts(t *testing.T) {
	callCount := 0
	err := retryWithBackoff(0, time.Millisecond, func() error {
		callCount++
		return fmt.Errorf("fail")
	})
	require.Error(t, err)
	assert.Equal(t, 1, callCount)
}

func TestDefaultStateDir(t *testing.T) {
	dir := defaultStateDir()
	assert.NotEmpty(t, dir)
}
