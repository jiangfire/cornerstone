package services

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
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
	fixture := testutil.SetupBenchmarkFixture(t, testutil.BenchmarkSeedConfig{
		RecordCount:     5000,
		ExtraFieldCount: 12,
	})

	statement := explainListRecordsStatement(fixture.DB.Name())
	rows, err := fixture.DB.Raw(statement, fixture.Table.ID).Rows()
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

	planText := strings.Join(details, " | ")
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
