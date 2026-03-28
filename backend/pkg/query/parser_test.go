package query

import (
	"testing"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "basic query",
			json: `{"from": "records", "select": ["id", "data"], "page": 1, "size": 20}`,
		},
		{
			name: "query with where",
			json: `{"from": "records", "where": {"and": [{"field": "id", "op": "eq", "value": "test"}]}}`,
		},
		{
			name: "query with join",
			json: `{"from": "records", "join": [{"type": "left", "table": "users", "on": "records.created_by = users.id"}]}`,
		},
		{
			name: "query with aggregate",
			json: `{"from": "records", "aggregate": [{"func": "count", "as": "total"}]}`,
		},
		{
			name:    "missing from",
			json:    `{"select": ["id"]}}`,
			wantErr: true,
		},
		{
			name:    "invalid page size",
			json:    `{"from": "records", "size": 99999}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parser.Parse([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && req == nil {
				t.Error("Parse() returned nil request without error")
			}
		})
	}
}

func TestParser_ParseSimplified(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		json    string
		wantErr bool
		check   func(t *testing.T, req *QueryRequest)
	}{
		{
			name: "simplified syntax",
			json: `{"table": "records", "filter": {"status": "active"}, "sort": "-created_at"}`,
			check: func(t *testing.T, req *QueryRequest) {
				if req.From != "records" {
					t.Errorf("expected From='records', got '%s'", req.From)
				}
				if req.Where == nil || len(req.Where.And) == 0 {
					t.Error("expected Where to be set")
				}
				if len(req.OrderBy) == 0 {
					t.Error("expected OrderBy to be set")
				}
			},
		},
		{
			name: "filter with operator",
			json: `{"table": "records", "filter": {"total": {"gt": 100}}}`,
			check: func(t *testing.T, req *QueryRequest) {
				if req.Where == nil {
					t.Fatal("expected Where to be set")
				}
				if len(req.Where.And) == 0 {
					t.Fatal("expected Where.And to have conditions")
				}
				cond := req.Where.And[0]
				if cond.Op != "gt" {
					t.Errorf("expected Op='gt', got '%s'", cond.Op)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parser.Parse([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, req)
			}
		})
	}
}

func TestParser_parseSimplifiedSort(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		sort      string
		wantField string
		wantDir   string
	}{
		{"-created_at", "created_at", "desc"},
		{"+created_at", "created_at", "asc"},
		{"created_at", "created_at", "asc"},
		{"-total,+name", "total", "desc"},
	}

	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			orderBy, err := parser.parseSimplifiedSort(tt.sort)
			if err != nil {
				t.Errorf("parseSimplifiedSort() error = %v", err)
				return
			}
			if len(orderBy) == 0 {
				t.Fatal("expected at least one order clause")
			}
			if orderBy[0].Field != tt.wantField {
				t.Errorf("expected Field='%s', got '%s'", tt.wantField, orderBy[0].Field)
			}
			if orderBy[0].Dir != tt.wantDir {
				t.Errorf("expected Dir='%s', got '%s'", tt.wantDir, orderBy[0].Dir)
			}
		})
	}
}

func TestParser_parseSimplifiedFilter(t *testing.T) {
	parser := NewParser()

	filter := map[string]interface{}{
		"status": "active",
		"total": map[string]interface{}{
			"gt": 100,
		},
	}

	where, err := parser.parseSimplifiedFilter(filter)
	if err != nil {
		t.Fatalf("parseSimplifiedFilter() error = %v", err)
	}

	if len(where.And) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(where.And))
	}

	// Check that both conditions exist (order may vary due to map iteration)
	foundStatus := false
	foundTotal := false
	for _, cond := range where.And {
		if cond.Field == "status" && cond.Op == "eq" && cond.Value == "active" {
			foundStatus = true
		}
		if cond.Field == "total" && cond.Op == "gt" {
			foundTotal = true
		}
	}
	if !foundStatus {
		t.Error("expected to find status = 'active' condition")
	}
	if !foundTotal {
		t.Error("expected to find total > 100 condition")
	}
}

func TestParser_parseSimplifiedFilterRejectsInvalidOperatorObject(t *testing.T) {
	parser := NewParser()

	filter := map[string]interface{}{
		"status": map[string]interface{}{
			"contains": "active",
		},
	}

	_, err := parser.parseSimplifiedFilter(filter)
	if err == nil {
		t.Fatal("expected parseSimplifiedFilter() to fail")
	}
	if err.Error() != "字段 'status' 包含无效操作符" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsValidOperator(t *testing.T) {
	validOps := []string{"eq", "ne", "gt", "gte", "lt", "lte", "like", "in", "between", "is_null"}
	for _, op := range validOps {
		if !isValidOperator(op) {
			t.Errorf("isValidOperator(%q) = false, want true", op)
		}
	}

	invalidOps := []string{"invalid", "equals", "contains", ""}
	for _, op := range invalidOps {
		if isValidOperator(op) {
			t.Errorf("isValidOperator(%q) = true, want false", op)
		}
	}
}

func TestIsValidAggregateFunc(t *testing.T) {
	validFuncs := []string{"count", "sum", "avg", "min", "max"}
	for _, fn := range validFuncs {
		if !isValidAggregateFunc(fn) {
			t.Errorf("isValidAggregateFunc(%q) = false, want true", fn)
		}
	}
}

func TestIsValidJoinType(t *testing.T) {
	validTypes := []string{"left", "right", "inner", "outer"}
	for _, typ := range validTypes {
		if !isValidJoinType(typ) {
			t.Errorf("isValidJoinType(%q) = false, want true", typ)
		}
	}
}

func TestParser_Validate(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		req     *QueryRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &QueryRequest{
				From:   "records",
				Select: []string{"id", "data"},
				Page:   1,
				Size:   20,
			},
		},
		{
			name: "too many joins",
			req: &QueryRequest{
				From: "records",
				Join: []JoinClause{
					{Type: "left", Table: "users", On: "a=b"},
					{Type: "left", Table: "users", On: "a=b"},
					{Type: "left", Table: "users", On: "a=b"},
					{Type: "left", Table: "users", On: "a=b"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid aggregate func",
			req: &QueryRequest{
				From:      "records",
				Aggregate: []AggregateFunc{{Func: "invalid", As: "test"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvertValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected interface{}
	}{
		{float64(42), int64(42)},
		{float64(42.5), float64(42.5)},
		{"123", int64(123)},
		{"123.45", float64(123.45)},
		{"true", true},
		{"hello", "hello"},
		{true, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := ConvertValue(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertValue(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParser_ParseBatch(t *testing.T) {
	parser := NewParser()

	jsonData := `{
		"queries": {
			"orders": {"from": "records", "filter": {"status": "paid"}, "size": 5},
			"stats": {"from": "records", "aggregate": [{"func": "count", "as": "total"}]}
		}
	}`

	req, err := parser.ParseBatch([]byte(jsonData))
	if err != nil {
		t.Fatalf("ParseBatch() error = %v", err)
	}

	if len(req.Queries) != 2 {
		t.Errorf("expected 2 queries, got %d", len(req.Queries))
	}

	if _, ok := req.Queries["orders"]; !ok {
		t.Error("expected 'orders' query")
	}

	if _, ok := req.Queries["stats"]; !ok {
		t.Error("expected 'stats' query")
	}
}

func TestParser_ParseBatchIncludesFailingQueryNameForNormalizeError(t *testing.T) {
	parser := NewParser()

	jsonData := `{
		"queries": {
			"safe": {"from": "records", "size": 5},
			"broken": {"select": ["id"], "size": 5}
		}
	}`

	_, err := parser.ParseBatch([]byte(jsonData))
	if err == nil {
		t.Fatal("expected ParseBatch() to fail")
	}
	if err.Error() != "查询 'broken' 格式错误: 必须指定表名 (from 或 table)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParser_ParseBatchIncludesFailingQueryNameForValidateError(t *testing.T) {
	parser := NewParser()

	jsonData := `{
		"queries": {
			"safe": {"from": "records", "size": 5},
			"oversize": {"from": "records", "size": 1001}
		}
	}`

	_, err := parser.ParseBatch([]byte(jsonData))
	if err == nil {
		t.Fatal("expected ParseBatch() to fail")
	}
	if err.Error() != "查询 'oversize' 验证失败: 每页大小不能超过 1000" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFromMap(t *testing.T) {
	parser := NewParser()

	data := map[string]interface{}{
		"table":  "records",
		"filter": map[string]interface{}{"status": "active"},
		"page":   1,
		"size":   10,
	}

	req, err := parser.ParseFromMap(data)
	if err != nil {
		t.Fatalf("ParseFromMap() error = %v", err)
	}

	if req.From != "records" {
		t.Errorf("expected From='records', got '%s'", req.From)
	}
}

func BenchmarkParser_Parse(b *testing.B) {
	parser := NewParser()
	jsonData := []byte(`{"from": "records", "select": ["id", "data"], "where": {"and": [{"field": "status", "op": "eq", "value": "active"}]}, "orderBy": [{"field": "created_at", "dir": "desc"}], "page": 1, "size": 20}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(jsonData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
