package query

import (
	"testing"

	"github.com/jiangfire/cornerstone/internal/testutil"
)

func BenchmarkExecutorExecute(b *testing.B) {
	fixture := testutil.SetupBenchmarkFixture(b, testutil.BenchmarkSeedConfig{
		RecordCount:     5000,
		ExtraFieldCount: 12,
	})
	executor := NewExecutor(fixture.DB)
	userID := fixture.ScopedToken.ID

	b.Run("records_by_table", func(b *testing.B) {
		template := QueryRequest{
			From:   "records",
			Select: []string{"id", "table_id", "created_at"},
			Where: &WhereClause{
				And: []Condition{
					{Field: "table_id", Op: "eq", Value: fixture.Table.ID},
				},
			},
			OrderBy: []OrderByClause{{Field: "created_at", Dir: "desc"}},
			Page:    1,
			Size:    50,
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := template
			result, err := executor.Execute(b.Context(), &req, userID)
			if err != nil {
				b.Fatal(err)
			}
			if len(result.Data) == 0 {
				b.Fatal("expected query rows")
			}
		}
	})

	b.Run("records_json_filter", func(b *testing.B) {
		template := QueryRequest{
			From:   "records",
			Select: []string{"id", "data.status", "created_at"},
			Where: &WhereClause{
				And: []Condition{
					{Field: "table_id", Op: "eq", Value: fixture.Table.ID},
					{Field: "data.status", Op: "eq", Value: "paid"},
				},
			},
			OrderBy: []OrderByClause{{Field: "created_at", Dir: "desc"}},
			Page:    1,
			Size:    50,
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := template
			result, err := executor.Execute(b.Context(), &req, userID)
			if err != nil {
				b.Fatal(err)
			}
			if len(result.Data) == 0 {
				b.Fatal("expected query rows")
			}
		}
	})

	b.Run("records_json_filter_id_only", func(b *testing.B) {
		template := QueryRequest{
			From:   "records",
			Select: []string{"id", "created_at"},
			Where: &WhereClause{
				And: []Condition{
					{Field: "table_id", Op: "eq", Value: fixture.Table.ID},
					{Field: "data.status", Op: "eq", Value: "paid"},
				},
			},
			OrderBy: []OrderByClause{{Field: "created_at", Dir: "desc"}},
			Page:    1,
			Size:    50,
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := template
			result, err := executor.Execute(b.Context(), &req, userID)
			if err != nil {
				b.Fatal(err)
			}
			if len(result.Data) == 0 {
				b.Fatal("expected query rows")
			}
		}
	})

	b.Run("records_json_filter_full_data_projection", func(b *testing.B) {
		template := QueryRequest{
			From:   "records",
			Select: []string{"id", "data", "created_at"},
			Where: &WhereClause{
				And: []Condition{
					{Field: "table_id", Op: "eq", Value: fixture.Table.ID},
					{Field: "data.status", Op: "eq", Value: "paid"},
				},
			},
			OrderBy: []OrderByClause{{Field: "created_at", Dir: "desc"}},
			Page:    1,
			Size:    50,
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := template
			result, err := executor.Execute(b.Context(), &req, userID)
			if err != nil {
				b.Fatal(err)
			}
			if len(result.Data) == 0 {
				b.Fatal("expected query rows")
			}
		}
	})
}
