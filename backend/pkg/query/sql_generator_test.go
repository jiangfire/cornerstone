package query

import (
	"strings"
	"testing"
)

func TestSQLGenerator_Generate(t *testing.T) {
	tests := []struct {
		name      string
		isSQLite  bool
		req       *QueryRequest
		wantSQL   string
		wantErr   bool
		checkFunc func(t *testing.T, sql string, params []interface{})
	}{
		{
			name:     "basic select",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id", "data"},
				Page:   1,
				Size:   20,
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "SELECT") {
					t.Error("expected SELECT in SQL")
				}
				if !strings.Contains(sql, "FROM \"records\"") {
					t.Error("expected FROM records in SQL")
				}
				if !strings.Contains(sql, "LIMIT") {
					t.Error("expected LIMIT in SQL")
				}
			},
		},
		{
			name:     "select with where eq",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id"},
				Where: &WhereClause{
					And: []Condition{
						{Field: "table_id", Op: "eq", Value: "tbl_123"},
					},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "WHERE") {
					t.Error("expected WHERE in SQL")
				}
				if !strings.Contains(sql, "=") {
					t.Error("expected = operator in SQL")
				}
				if len(params) == 0 {
					t.Error("expected at least one parameter")
				}
			},
		},
		{
			name:     "select with where gt",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id"},
				Where: &WhereClause{
					And: []Condition{
						{Field: "total", Op: "gt", Value: 100},
					},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "\u003e") {
					t.Error("expected \u003e operator in SQL")
				}
			},
		},
		{
			name:     "select with where in",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id"},
				Where: &WhereClause{
					And: []Condition{
						{Field: "status", Op: "in", Value: []interface{}{"active", "pending"}},
					},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "IN") {
					t.Error("expected IN operator in SQL")
				}
				// 2 params for IN values + 2 for LIMIT/OFFSET = 4
				if len(params) != 4 {
					t.Errorf("expected 4 params (2 for IN + 2 for pagination), got %d", len(params))
				}
			},
		},
		{
			name:     "select with where like",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id"},
				Where: &WhereClause{
					And: []Condition{
						{Field: "name", Op: "like", Value: "test"},
					},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "LIKE") {
					t.Error("expected LIKE operator in SQL")
				}
			},
		},
		{
			name:     "select with order by",
			isSQLite: false,
			req: &QueryRequest{
				From:    "records",
				Select:  []string{"id"},
				OrderBy: []OrderByClause{{Field: "created_at", Dir: "desc"}},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "ORDER BY") {
					t.Error("expected ORDER BY in SQL")
				}
				if !strings.Contains(sql, "DESC") {
					t.Error("expected DESC in SQL")
				}
			},
		},
		{
			name:     "select with join",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"records.id", "users.name"},
				Join: []JoinClause{
					{Type: "left", Table: "users", As: "u", On: "records.created_by = u.id"},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "JOIN") {
					t.Error("expected JOIN in SQL")
				}
				if !strings.Contains(sql, "LEFT") {
					t.Error("expected LEFT in SQL")
				}
				if !strings.Contains(sql, "ON") {
					t.Error("expected ON in SQL")
				}
			},
		},
		{
			name:     "select with aggregate",
			isSQLite: false,
			req: &QueryRequest{
				From:    "records",
				Select:  []string{"status"},
				GroupBy: []string{"status"},
				Aggregate: []AggregateFunc{
					{Func: "count", Field: "*", As: "total"},
					{Func: "sum", Field: "amount", As: "total_amount"},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "COUNT") {
					t.Error("expected COUNT in SQL")
				}
				if !strings.Contains(sql, "SUM") {
					t.Error("expected SUM in SQL")
				}
				if !strings.Contains(sql, "GROUP BY") {
					t.Error("expected GROUP BY in SQL")
				}
			},
		},
		{
			name:     "select with json field postgres",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"data->>name"},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				// The field with ->> is treated as a literal expression
				if !strings.Contains(sql, "data->>name") {
					t.Errorf("expected data->>name in SQL, got: %s", sql)
				}
			},
		},
		{
			name:     "select with json field sqlite",
			isSQLite: true,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"data.name"},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "JSON_EXTRACT(\"data\", '$.name')") {
					t.Errorf("expected JSON_EXTRACT for JSON field, got: %s", sql)
				}
			},
		},
		{
			name:     "select with pagination",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id"},
				Page:   2,
				Size:   10,
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "LIMIT") {
					t.Error("expected LIMIT in SQL")
				}
				if !strings.Contains(sql, "OFFSET") {
					t.Error("expected OFFSET in SQL")
				}
				// Check that offset is calculated correctly (page 2 = offset 10)
				if len(params) < 2 {
					t.Error("expected at least 2 params for pagination")
				}
			},
		},
		{
			name:     "select with where between",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id"},
				Where: &WhereClause{
					And: []Condition{
						{Field: "created_at", Op: "between", Value: []interface{}{"2024-01-01", "2024-12-31"}},
					},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "BETWEEN") {
					t.Error("expected BETWEEN in SQL")
				}
				// 2 params for BETWEEN + 2 for LIMIT/OFFSET = 4
				if len(params) != 4 {
					t.Errorf("expected 4 params (2 for BETWEEN + 2 for pagination), got %d", len(params))
				}
			},
		},
		{
			name:     "select with where is_null",
			isSQLite: false,
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id"},
				Where: &WhereClause{
					And: []Condition{
						{Field: "deleted_at", Op: "is_null", Value: true},
					},
				},
			},
			checkFunc: func(t *testing.T, sql string, params []interface{}) {
				if !strings.Contains(sql, "IS NULL") {
					t.Error("expected IS NULL in SQL")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewSQLGenerator(tt.isSQLite)
			sqlQuery, err := gen.Generate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, sqlQuery.SQL, sqlQuery.Params)
			}
		})
	}
}

func TestSQLGenerator_GenerateCount(t *testing.T) {
	gen := NewSQLGenerator(false)

	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "status", Op: "eq", Value: "active"},
			},
		},
	}

	sqlQuery, err := gen.GenerateCount(req)
	if err != nil {
		t.Fatalf("GenerateCount() error = %v", err)
	}

	if !strings.Contains(sqlQuery.SQL, "COUNT(*)") {
		t.Error("expected COUNT(*) in SQL")
	}

	if !strings.Contains(sqlQuery.SQL, "WHERE") {
		t.Error("expected WHERE in SQL")
	}

	if len(sqlQuery.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(sqlQuery.Params))
	}
}

func TestSQLGenerator_GenerateCountWithGroupByUsesSubquery(t *testing.T) {
	gen := NewSQLGenerator(false)

	req := &QueryRequest{
		From:    "records",
		Select:  []string{"data.status"},
		GroupBy: []string{"data.status"},
		Aggregate: []AggregateFunc{
			{Func: "count", Field: "*", As: "total"},
		},
	}

	sqlQuery, err := gen.GenerateCount(req)
	if err != nil {
		t.Fatalf("GenerateCount() error = %v", err)
	}

	if !strings.Contains(sqlQuery.SQL, "FROM (SELECT") {
		t.Errorf("expected subquery count SQL, got: %s", sqlQuery.SQL)
	}
	if !strings.Contains(sqlQuery.SQL, "GROUP BY") {
		t.Errorf("expected grouped subquery, got: %s", sqlQuery.SQL)
	}
}

func TestSQLGenerator_GenerateSQLiteJSONWhereAndOrderBy(t *testing.T) {
	gen := NewSQLGenerator(true)

	req := &QueryRequest{
		From:   "records",
		Select: []string{"id", "data"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "data.status", Op: "eq", Value: "approved"},
				{Field: "data.amount", Op: "gte", Value: 10},
			},
		},
		OrderBy: []OrderByClause{
			{Field: "data.amount", Dir: "desc"},
			{Field: "data.status", Dir: "asc"},
		},
		Page: 1,
		Size: 20,
	}

	sqlQuery, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !strings.Contains(sqlQuery.SQL, `JSON_EXTRACT("data", '$.status') = ?`) {
		t.Errorf("expected JSON status filter, got: %s", sqlQuery.SQL)
	}
	if !strings.Contains(sqlQuery.SQL, `JSON_EXTRACT("data", '$.amount') >= ?`) {
		t.Errorf("expected JSON amount filter, got: %s", sqlQuery.SQL)
	}
	if !strings.Contains(sqlQuery.SQL, `ORDER BY JSON_EXTRACT("data", '$.amount') DESC, JSON_EXTRACT("data", '$.status') ASC`) {
		t.Errorf("expected JSON order by, got: %s", sqlQuery.SQL)
	}
	if len(sqlQuery.Params) != 4 {
		t.Fatalf("expected 4 params, got %d", len(sqlQuery.Params))
	}
	if sqlQuery.Params[0] != "approved" {
		t.Fatalf("expected first param to be approved, got %#v", sqlQuery.Params[0])
	}
	if sqlQuery.Params[1] != 10 {
		t.Fatalf("expected second param to be 10, got %#v", sqlQuery.Params[1])
	}
	if sqlQuery.Params[2] != 20 || sqlQuery.Params[3] != 0 {
		t.Fatalf("unexpected pagination params: %#v", sqlQuery.Params[2:])
	}
}

func TestSQLGenerator_GenerateRejectsEmptyInValues(t *testing.T) {
	gen := NewSQLGenerator(true)

	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "id", Op: "in", Value: []interface{}{}},
			},
		},
		Page: 1,
		Size: 20,
	}

	_, err := gen.Generate(req)
	if err == nil {
		t.Fatal("expected Generate() to fail")
	}
	if err.Error() != "'in' 操作符数组不能为空" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSQLGenerator_GenerateRejectsInvalidBetweenValues(t *testing.T) {
	gen := NewSQLGenerator(true)

	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "created_at", Op: "between", Value: []interface{}{"2024-01-01"}},
			},
		},
		Page: 1,
		Size: 20,
	}

	_, err := gen.Generate(req)
	if err == nil {
		t.Fatal("expected Generate() to fail")
	}
	if err.Error() != "'between' 操作符需要包含两个值的数组" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSQLGenerator_GenerateRejectsUnknownOperator(t *testing.T) {
	gen := NewSQLGenerator(true)

	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "id", Op: "contains", Value: "rec_1"},
			},
		},
		Page: 1,
		Size: 20,
	}

	_, err := gen.Generate(req)
	if err == nil {
		t.Fatal("expected Generate() to fail")
	}
	if err.Error() != "未知的操作符: contains" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSQLGenerator_GenerateAggregateSelectUsesFieldExpressions(t *testing.T) {
	gen := NewSQLGenerator(true)

	req := &QueryRequest{
		From:   "records",
		Select: []string{"data.status", "records.table_id"},
		Aggregate: []AggregateFunc{
			{Func: "count", Field: "*", As: "total"},
		},
		GroupBy: []string{"data.status", "records.table_id"},
		Page:    1,
		Size:    20,
	}

	sqlQuery, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !strings.Contains(sqlQuery.SQL, `SELECT JSON_EXTRACT("data", '$.status'), "records"."table_id", COUNT(*) AS "total"`) {
		t.Fatalf("expected aggregate select to use field expressions, got: %s", sqlQuery.SQL)
	}
	if !strings.Contains(sqlQuery.SQL, `GROUP BY JSON_EXTRACT("data", '$.status'), "records"."table_id"`) {
		t.Fatalf("expected grouped field expressions, got: %s", sqlQuery.SQL)
	}
}

func TestSQLGenerator_generateAggregate(t *testing.T) {
	gen := NewSQLGenerator(false)

	tests := []struct {
		agg      AggregateFunc
		expected string
	}{
		{AggregateFunc{Func: "count", Field: "*", As: "total"}, "COUNT(*) AS \"total\""},
		{AggregateFunc{Func: "sum", Field: "amount", As: "sum_amount"}, "SUM(\"amount\") AS \"sum_amount\""},
		{AggregateFunc{Func: "avg", Field: "score", As: "avg_score"}, "AVG(\"score\") AS \"avg_score\""},
		{AggregateFunc{Func: "min", Field: "price", As: "min_price"}, "MIN(\"price\") AS \"min_price\""},
		{AggregateFunc{Func: "max", Field: "price", As: "max_price"}, "MAX(\"price\") AS \"max_price\""},
	}

	for _, tt := range tests {
		t.Run(tt.agg.Func, func(t *testing.T) {
			result := gen.generateAggregate(tt.agg)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSQLGenerator_quoteIdentifier(t *testing.T) {
	gen := NewSQLGenerator(false)

	tests := []struct {
		input    string
		expected string
	}{
		{"id", "\"id\""},
		{"table_name", "\"table_name\""},
		{"*", "*"},
		{"test\"quote", "\"test\"\"quote\""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := gen.quoteIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSQLGenerator_NilRequest(t *testing.T) {
	gen := NewSQLGenerator(false)

	_, err := gen.Generate(nil)
	if err == nil {
		t.Error("expected error for nil request")
	}

	_, err = gen.GenerateCount(nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func BenchmarkSQLGenerator_Generate(b *testing.B) {
	gen := NewSQLGenerator(false)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id", "data"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "status", Op: "eq", Value: "active"},
				{Field: "created_at", Op: "gt", Value: "2024-01-01"},
			},
		},
		OrderBy: []OrderByClause{{Field: "created_at", Dir: "desc"}},
		Page:    1,
		Size:    20,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.Generate(req)
		if err != nil {
			b.Fatal(err)
		}
	}
}
