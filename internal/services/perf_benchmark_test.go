package services

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
	"gorm.io/gorm"
)

const (
	mysqlPlainNarrowProjectionQuery = `SELECT id, table_id, created_at
FROM records
WHERE table_id = ? AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 50`
	mysqlForceCompositeIndexQuery = `SELECT id, table_id, created_at
FROM records FORCE INDEX (idx_records_table_deleted_created)
WHERE table_id = ? AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 50`
	mysqlStructuredFilterRawQuery = `SELECT id, table_id, created_at
FROM records
WHERE table_id = ? AND deleted_at IS NULL
  AND JSON_EXTRACT(data, ?) = ?
  AND JSON_EXTRACT(data, ?) = ?
ORDER BY created_at DESC
LIMIT 50`
	mysqlRecordFieldIndexQuery = `SELECT id, table_id, created_at
FROM (
  SELECT record_id
  FROM (
    SELECT record_id, field_id
    FROM record_field_indexes
    WHERE table_id = ? AND deleted_at IS NULL AND field_id = ? AND value_text = ?
    UNION ALL
    SELECT record_id, field_id
    FROM record_field_indexes
    WHERE table_id = ? AND deleted_at IS NULL AND field_id = ? AND value_text = ?
  ) rfi_matches
  GROUP BY record_id
  HAVING COUNT(DISTINCT field_id) = 2
) matched
JOIN records FORCE INDEX (PRIMARY) ON records.id = matched.record_id
WHERE records.table_id = ? AND records.deleted_at IS NULL
ORDER BY records.created_at DESC
LIMIT 50`
	mysqlGeneratedColumnSetup = `ALTER TABLE records
ADD COLUMN bench_status VARCHAR(32) GENERATED ALWAYS AS (JSON_UNQUOTE(JSON_EXTRACT(data, '$.status'))) STORED,
ADD COLUMN bench_category VARCHAR(32) GENERATED ALWAYS AS (JSON_UNQUOTE(JSON_EXTRACT(data, '$.category'))) STORED`
	mysqlGeneratedColumnIndex = `CREATE INDEX idx_records_bench_status_category_created
ON records(table_id, deleted_at, bench_status, bench_category, created_at DESC)`
	mysqlGeneratedColumnQuery = `SELECT id, table_id, created_at
FROM records FORCE INDEX (idx_records_bench_status_category_created)
WHERE table_id = ? AND deleted_at IS NULL AND bench_status = ? AND bench_category = ?
ORDER BY created_at DESC
LIMIT 50`
)

type benchmarkNarrowRecordRow struct {
	ID        string
	TableID   string
	CreatedAt time.Time
}

type benchmarkWideRecordRow struct {
	ID        string
	TableID   string
	Data      models.JSONField
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func BenchmarkRecordServiceListRecords(b *testing.B) {
	fixture := testutil.SetupBenchmarkFixture(b, testutil.BenchmarkSeedConfig{
		RecordCount:     5000,
		ExtraFieldCount: 12,
	})
	service := NewRecordService(fixture.DB)
	userID := fixture.ScopedToken.ID
	fields, err := service.getTableFields(fixture.Table.ID)
	if err != nil {
		b.Fatal(err)
	}
	readableFields, _, err := service.getFieldAccessMaps(fields, userID)
	if err != nil {
		b.Fatal(err)
	}
	filteredRows := make([]models.Record, 0, 50)
	if err := fixture.DB.
		Where("table_id = ? AND deleted_at IS NULL", fixture.Table.ID).
		Order("created_at DESC").
		Limit(50).
		Find(&filteredRows).Error; err != nil {
		b.Fatal(err)
	}
	structuredClauses := mustBuildStructuredBenchmarkClauses(
		b,
		service,
		fields,
		readableFields,
		`{"status":"paid","category":"beta"}`,
	)

	b.Run("no_filter", func(b *testing.B) {
		req := QueryRequest{
			TableID: fixture.Table.ID,
			Limit:   50,
			Offset:  0,
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.ListRecords(req, userID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("no_filter_db_narrow_projection", func(b *testing.B) {
		benchmarkQueryRows[benchmarkNarrowRecordRow](b, func(dest *[]benchmarkNarrowRecordRow) error {
			return fixture.DB.Model(&models.Record{}).
				Select("id, table_id, created_at").
				Where("table_id = ? AND deleted_at IS NULL", fixture.Table.ID).
				Order("created_at DESC").
				Limit(50).
				Scan(dest).Error
		})
	})

	b.Run("no_filter_db_wide_projection", func(b *testing.B) {
		benchmarkQueryRows[benchmarkWideRecordRow](b, func(dest *[]benchmarkWideRecordRow) error {
			return fixture.DB.Model(&models.Record{}).
				Select("id, table_id, data, version, created_at, updated_at").
				Where("table_id = ? AND deleted_at IS NULL", fixture.Table.ID).
				Order("created_at DESC").
				Limit(50).
				Scan(dest).Error
		})
	})

	b.Run("no_filter_go_response_shaping", func(b *testing.B) {
		benchmarkRecordResponseShaping(b, service, fields, readableFields, filteredRows)
	})

	b.Run("structured_filter", func(b *testing.B) {
		req := QueryRequest{
			TableID: fixture.Table.ID,
			Limit:   50,
			Offset:  0,
			Filter:  `{"status":"paid","category":"beta"}`,
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.ListRecords(req, userID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("structured_filter_db_narrow_projection", func(b *testing.B) {
		benchmarkQueryRows[benchmarkNarrowRecordRow](b, func(dest *[]benchmarkNarrowRecordRow) error {
			query := fixture.DB.Model(&models.Record{}).
				Select("id, table_id, created_at").
				Where("table_id = ? AND deleted_at IS NULL", fixture.Table.ID)
			for _, clause := range structuredClauses {
				query = query.Where(clause.sql, clause.args...)
			}
			return query.Order("created_at DESC").Limit(50).Scan(dest).Error
		})
	})

	b.Run("structured_filter_db_wide_projection", func(b *testing.B) {
		benchmarkQueryRows[benchmarkWideRecordRow](b, func(dest *[]benchmarkWideRecordRow) error {
			query := fixture.DB.Model(&models.Record{}).
				Select("id, table_id, data, version, created_at, updated_at").
				Where("table_id = ? AND deleted_at IS NULL", fixture.Table.ID)
			for _, clause := range structuredClauses {
				query = query.Where(clause.sql, clause.args...)
			}
			return query.Order("created_at DESC").Limit(50).Scan(dest).Error
		})
	})

	if fixture.DB.Name() == "mysql" {
		b.Run("mysql_no_filter_db_narrow_projection_raw_sql", func(b *testing.B) {
			benchmarkRawQueryRows[benchmarkNarrowRecordRow](b, fixture.DB, mysqlPlainNarrowProjectionQuery, fixture.Table.ID)
		})

		b.Run("mysql_no_filter_db_narrow_projection_force_composite_index", func(b *testing.B) {
			benchmarkRawQueryRows[benchmarkNarrowRecordRow](b, fixture.DB, mysqlForceCompositeIndexQuery, fixture.Table.ID)
		})

		b.Run("mysql_structured_filter_raw_sql", func(b *testing.B) {
			benchmarkRawQueryRows[benchmarkNarrowRecordRow](
				b,
				fixture.DB,
				mysqlStructuredFilterRawQuery,
				fixture.Table.ID,
				"$.status",
				"paid",
				"$.category",
				"beta",
			)
		})

		b.Run("mysql_structured_filter_record_field_index", func(b *testing.B) {
			statusID := mustBenchmarkFieldID(b, fields, "status")
			categoryID := mustBenchmarkFieldID(b, fields, "category")
			benchmarkRawQueryRows[benchmarkNarrowRecordRow](
				b,
				fixture.DB,
				mysqlRecordFieldIndexQuery,
				mysqlRecordFieldIndexArgs(fixture.Table.ID, statusID, categoryID)...,
			)
		})

		b.Run("mysql_structured_filter_generated_columns", func(b *testing.B) {
			if err := prepareMySQLGeneratedColumnExperiment(fixture.DB); err != nil {
				b.Fatal(err)
			}
			benchmarkRawQueryRows[benchmarkNarrowRecordRow](
				b,
				fixture.DB,
				mysqlGeneratedColumnQuery,
				fixture.Table.ID,
				"paid",
				"beta",
			)
		})
	}
}

func benchmarkQueryRows[T any](b *testing.B, run func(dest *[]T) error) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var rows []T
		if err := run(&rows); err != nil {
			b.Fatal(err)
		}
		if len(rows) == 0 {
			b.Fatal("expected rows")
		}
	}
}

func benchmarkRawQueryRows[T any](b *testing.B, db *gorm.DB, query string, args ...interface{}) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var rows []T
		if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
			b.Fatal(err)
		}
		if len(rows) == 0 {
			b.Fatal("expected rows")
		}
	}
}

func benchmarkRecordResponseShaping(
	b *testing.B,
	service *RecordService,
	fields []models.Field,
	readableFields map[string]models.Field,
	records []models.Record,
) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		responses := make([]RecordResponse, 0, len(records))
		for _, record := range records {
			data := service.filterReadableData(fields, readableFields, parseRecordPayload(record.Data))
			responses = append(responses, RecordResponse{
				ID:        record.ID,
				TableID:   record.TableID,
				Data:      data,
				Version:   record.Version,
				CreatedAt: record.CreatedAt.Format(time.RFC3339),
				UpdatedAt: record.UpdatedAt.Format(time.RFC3339),
			})
		}
		if len(responses) == 0 {
			b.Fatal("expected responses")
		}
	}
}

func mustBuildStructuredBenchmarkClauses(
	b *testing.B,
	service *RecordService,
	fields []models.Field,
	readableFields map[string]models.Field,
	filter string,
) []recordFilterClause {
	b.Helper()
	structured, ok := tryParseStructuredFilter(filter)
	if !ok {
		b.Fatalf("expected structured filter, got %q", filter)
	}
	clauses, refsHidden, err := service.buildStructuredFilterClauses(fields, readableFields, structured)
	if err != nil {
		b.Fatal(err)
	}
	if refsHidden {
		b.Fatalf("structured benchmark filter unexpectedly references hidden field: %q", filter)
	}
	return clauses
}

func mustBenchmarkFieldID(tb testing.TB, fields []models.Field, name string) string {
	tb.Helper()
	for _, field := range fields {
		if field.Name == name {
			return field.ID
		}
	}
	tb.Fatalf("expected benchmark field %q", name)
	return ""
}

func BenchmarkFieldServiceListFields(b *testing.B) {
	fixture := testutil.SetupBenchmarkFixture(b, testutil.BenchmarkSeedConfig{
		RecordCount:     1000,
		ExtraFieldCount: 48,
	})
	service := NewFieldService(fixture.DB)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fields, err := service.ListFields(fixture.Table.ID, fixture.ScopedToken.ID)
		if err != nil {
			b.Fatal(err)
		}
		if len(fields) == 0 {
			b.Fatal("expected fields")
		}
	}
}

func TestExplainPlanListRecords(t *testing.T) {
	skipExplainDiagnosticsUnderRace(t)

	fixture := testutil.SetupBenchmarkFixture(t, testutil.BenchmarkSeedConfig{
		RecordCount:     5000,
		ExtraFieldCount: 12,
	})

	planText := explainListRecords(t, fixture.DB, fixture.Table.ID)
	t.Logf("list records plan: %s", planText)

	switch fixture.DB.Name() {
	case "sqlite":
		if !strings.Contains(planText, "idx_records_table_deleted_created") {
			t.Fatalf("expected idx_records_table_deleted_created in plan, got: %s", planText)
		}
		if strings.Contains(planText, "USE TEMP B-TREE FOR ORDER BY") {
			t.Fatalf("expected plan to avoid temp b-tree sort, got: %s", planText)
		}
	case "mysql", "postgres":
		if !strings.Contains(strings.ToLower(planText), "records") {
			t.Fatalf("expected plan to mention records table, got: %s", planText)
		}
	}
}

func TestExplainPlanListRecordsWithNoiseTables(t *testing.T) {
	skipExplainDiagnosticsUnderRace(t)

	fixture := testutil.SetupBenchmarkFixture(t, testutil.BenchmarkSeedConfig{
		RecordCount:          5000,
		ExtraFieldCount:      12,
		NoiseTableCount:      4,
		NoiseRecordsPerTable: 2500,
	})

	planText := explainListRecords(t, fixture.DB, fixture.Table.ID)
	t.Logf("list records plan with noise tables: %s", planText)

	switch fixture.DB.Name() {
	case "sqlite":
		if !strings.Contains(planText, "idx_records_table_deleted_created") {
			t.Fatalf("expected idx_records_table_deleted_created in plan, got: %s", planText)
		}
	case "mysql", "postgres":
		if !strings.Contains(strings.ToLower(planText), "records") {
			t.Fatalf("expected plan to mention records table, got: %s", planText)
		}
	}
}

func TestExplainPlanListRecordsMySQLExperiments(t *testing.T) {
	skipExplainDiagnosticsUnderRace(t)

	fixture := testutil.SetupBenchmarkFixture(t, testutil.BenchmarkSeedConfig{
		RecordCount:     5000,
		ExtraFieldCount: 12,
	})
	if fixture.DB.Name() != "mysql" {
		t.Skip("mysql-only diagnostics")
	}

	plainPlan := explainListRecords(t, fixture.DB, fixture.Table.ID)
	t.Logf("mysql plain list records plan: %s", plainPlan)

	forcedPlan := explainStatement(t, fixture.DB, "EXPLAIN ANALYZE "+mysqlForceCompositeIndexQuery, fixture.Table.ID)
	t.Logf("mysql forced composite plan: %s", forcedPlan)

	structuredRawPlan := explainStatement(
		t,
		fixture.DB,
		"EXPLAIN ANALYZE "+mysqlStructuredFilterRawQuery,
		fixture.Table.ID,
		"$.status",
		"paid",
		"$.category",
		"beta",
	)
	t.Logf("mysql raw structured filter plan: %s", structuredRawPlan)

	recordFieldIndexPlan := explainStatement(
		t,
		fixture.DB,
		"EXPLAIN ANALYZE "+mysqlRecordFieldIndexQuery,
		mysqlRecordFieldIndexArgs(
			fixture.Table.ID,
			mustBenchmarkFieldID(t, fixture.Fields, "status"),
			mustBenchmarkFieldID(t, fixture.Fields, "category"),
		)...,
	)
	t.Logf("mysql record field index structured filter plan: %s", recordFieldIndexPlan)

	if err := prepareMySQLGeneratedColumnExperiment(fixture.DB); err != nil {
		t.Fatalf("prepare generated column experiment failed: %v", err)
	}
	generatedPlan := explainStatement(
		t,
		fixture.DB,
		"EXPLAIN ANALYZE "+mysqlGeneratedColumnQuery,
		fixture.Table.ID,
		"paid",
		"beta",
	)
	t.Logf("mysql generated column plan: %s", generatedPlan)
}

func skipExplainDiagnosticsUnderRace(t *testing.T) {
	t.Helper()
	if !raceEnabled() || os.Getenv("CORNERSTONE_PERF_EXPLAIN_FULL") == "1" {
		return
	}
	t.Skip("heavy EXPLAIN diagnostics are covered by the Performance workflow; set CORNERSTONE_PERF_EXPLAIN_FULL=1 to run them under -race")
}

func TestMySQLRecordFieldIndexBenchmarkQueryArgs(t *testing.T) {
	args := mysqlRecordFieldIndexArgs("tbl_1", "fld_status", "fld_category")

	if placeholders := strings.Count(mysqlRecordFieldIndexQuery, "?"); placeholders != len(args) {
		t.Fatalf("expected %d query args, got %d", placeholders, len(args))
	}
	expected := []interface{}{
		"tbl_1",
		"fld_status",
		"paid",
		"tbl_1",
		"fld_category",
		"beta",
		"tbl_1",
	}
	for i := range expected {
		if expected[i] != args[i] {
			t.Fatalf("arg %d: expected %v, got %v", i, expected[i], args[i])
		}
	}
}

func mysqlRecordFieldIndexArgs(tableID, statusID, categoryID string) []interface{} {
	return []interface{}{
		tableID,
		statusID,
		"paid",
		tableID,
		categoryID,
		"beta",
		tableID,
	}
}

func prepareMySQLGeneratedColumnExperiment(db *gorm.DB) error {
	if db.Name() != "mysql" {
		return nil
	}

	var count int64
	if err := db.Raw(`
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND COLUMN_NAME = 'bench_status'
`).Scan(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		if err := db.Exec(mysqlGeneratedColumnSetup).Error; err != nil {
			return err
		}
	}

	count = 0
	if err := db.Raw(`
SELECT COUNT(*)
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_bench_status_category_created'
`).Scan(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		if err := db.Exec(mysqlGeneratedColumnIndex).Error; err != nil {
			return err
		}
	}

	return nil
}

func explainListRecordsStatement(dbType string) string {
	baseQuery := `SELECT id, table_id, data, version, created_at, updated_at
FROM records
WHERE table_id = ? AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 50 OFFSET 0`

	switch dbType {
	case "sqlite":
		return "EXPLAIN QUERY PLAN " + baseQuery
	case "mysql", "postgres":
		return "EXPLAIN ANALYZE " + baseQuery
	default:
		return "EXPLAIN " + baseQuery
	}
}

func explainListRecords(t *testing.T, db *gorm.DB, tableID string) string {
	t.Helper()
	return explainStatement(t, db, explainListRecordsStatement(db.Name()), tableID)
}

func explainStatement(t *testing.T, db *gorm.DB, statement string, args ...interface{}) string {
	t.Helper()

	rows, err := db.Raw(statement, args...).Rows()
	if err != nil {
		t.Fatalf("explain failed: %v", err)
	}
	defer rows.Close()

	details, err := collectExplainRows(rows)
	if err != nil {
		t.Fatalf("collect explain rows failed: %v", err)
	}
	if len(details) == 0 {
		t.Fatal("expected explain plan rows")
	}

	return strings.Join(details, " | ")
}

func collectExplainRows(rows *sql.Rows) ([]string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	details := make([]string, 0, 8)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		parts := make([]string, 0, len(columns))
		for _, value := range values {
			if value == nil {
				continue
			}
			switch v := value.(type) {
			case []byte:
				parts = append(parts, string(v))
			default:
				parts = append(parts, fmt.Sprint(v))
			}
		}
		if len(parts) > 0 {
			details = append(details, strings.Join(parts, " "))
		}
	}

	return details, rows.Err()
}
