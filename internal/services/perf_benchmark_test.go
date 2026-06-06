package services

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/jiangfire/cornerstone/internal/testutil"
)

func BenchmarkRecordServiceListRecords(b *testing.B) {
	fixture := testutil.SetupBenchmarkFixture(b, testutil.BenchmarkSeedConfig{
		RecordCount:     5000,
		ExtraFieldCount: 12,
	})
	service := NewRecordService(fixture.DB)
	userID := fixture.ScopedToken.ID

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

	if !strings.Contains(planText, "idx_records_table_deleted_created") {
		t.Fatalf("expected idx_records_table_deleted_created in plan, got: %s", planText)
	}
	if fixture.DB.Name() == "sqlite" && strings.Contains(planText, "USE TEMP B-TREE FOR ORDER BY") {
		t.Fatalf("expected plan to avoid temp b-tree sort, got: %s", planText)
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
