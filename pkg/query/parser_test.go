package query

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_BasicQuery(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id", "name"},
		"where": map[string]interface{}{
			"and": []interface{}{
				map[string]interface{}{"field": "status", "op": "eq", "value": "active"},
			},
		},
		"orderBy": []interface{}{
			map[string]interface{}{"field": "id", "dir": "desc"},
		},
		"page": float64(2),
		"size": float64(50),
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, "records", req.From)
	assert.Equal(t, []string{"id", "name"}, req.Select)
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	assert.Equal(t, "status", req.Where.And[0].Field)
	assert.Equal(t, "eq", req.Where.And[0].Op)
	assert.Equal(t, "active", req.Where.And[0].Value)
	require.Len(t, req.OrderBy, 1)
	assert.Equal(t, "id", req.OrderBy[0].Field)
	assert.Equal(t, "desc", req.OrderBy[0].Dir)
	assert.Equal(t, 2, req.Page)
	assert.Equal(t, 50, req.Size)
}

func TestParse_SimplifiedSyntax(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"table": "records",
		"filter": map[string]interface{}{
			"status": "active",
		},
		"sort": "-created_at,name",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, "records", req.From)
	assert.Equal(t, []string{"*"}, req.Select)
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	assert.Equal(t, "status", req.Where.And[0].Field)
	assert.Equal(t, "eq", req.Where.And[0].Op)
	assert.Equal(t, "active", req.Where.And[0].Value)
	require.Len(t, req.OrderBy, 2)
	assert.Equal(t, "created_at", req.OrderBy[0].Field)
	assert.Equal(t, "desc", req.OrderBy[0].Dir)
	assert.Equal(t, "name", req.OrderBy[1].Field)
	assert.Equal(t, "asc", req.OrderBy[1].Dir)
	assert.Equal(t, 1, req.Page)
	assert.Equal(t, 20, req.Size)
}

func TestParse_MissingFrom(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"select": []string{"id"},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "from")
}

func TestParse_PageSizeExceedsMax(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"size": float64(2000),
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "1000")
}

func TestParse_NestedWhereConditions(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
		"where": map[string]interface{}{
			"and": []interface{}{
				map[string]interface{}{
					"field": "status",
					"op":    "eq",
					"value": "active",
				},
				map[string]interface{}{
					"or": []interface{}{
						map[string]interface{}{"field": "priority", "op": "gt", "value": float64(5)},
						map[string]interface{}{"field": "flagged", "op": "eq", "value": true},
					},
				},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)

	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 2)
	assert.Equal(t, "status", req.Where.And[0].Field)
	require.Len(t, req.Where.And[1].Or, 2)
	assert.Equal(t, "priority", req.Where.And[1].Or[0].Field)
	assert.Equal(t, "gt", req.Where.And[1].Or[0].Op)
	assert.Equal(t, "flagged", req.Where.And[1].Or[1].Field)
}

func TestParse_InvalidOperator(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
		"where": map[string]interface{}{
			"and": []interface{}{
				map[string]interface{}{"field": "status", "op": "invalid_op", "value": "x"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_op")
}

func TestParse_AggregateFunctionsWithAs(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"aggregate": []interface{}{
			map[string]interface{}{"func": "count", "field": "id", "as": "total"},
			map[string]interface{}{"func": "sum", "field": "amount", "as": "total_amount"},
			map[string]interface{}{"func": "avg", "field": "score", "as": "avg_score"},
			map[string]interface{}{"func": "min", "field": "price", "as": "min_price"},
			map[string]interface{}{"func": "max", "field": "price", "as": "max_price"},
		},
		"groupBy": []string{"category"},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)

	require.Len(t, req.Aggregate, 5)
	assert.Equal(t, "count", req.Aggregate[0].Func)
	assert.Equal(t, "total", req.Aggregate[0].As)
	assert.Equal(t, "sum", req.Aggregate[1].Func)
	assert.Equal(t, "avg", req.Aggregate[2].Func)
	assert.Equal(t, "min", req.Aggregate[3].Func)
	assert.Equal(t, "max", req.Aggregate[4].Func)
}

func TestParse_AggregateMissingAs(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"aggregate": []interface{}{
			map[string]interface{}{"func": "count", "field": "id"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "as")
}

func TestParse_InvalidAggregateFunction(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"aggregate": []interface{}{
			map[string]interface{}{"func": "median", "field": "score", "as": "med"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "median")
}

func TestParse_JoinValid(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id", "data"},
		"join": []interface{}{
			map[string]interface{}{
				"type":   "left",
				"table":  "tables",
				"as":     "t",
				"on":     map[string]interface{}{"left": "records.table_id", "op": "=", "right": "t.id"},
				"select": []string{"name"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)

	require.Len(t, req.Join, 1)
	assert.Equal(t, "left", req.Join[0].Type)
	assert.Equal(t, "tables", req.Join[0].Table)
	assert.Equal(t, "t", req.Join[0].As)
	assert.Equal(t, "records.table_id", req.Join[0].On.Left)
	assert.Equal(t, "=", req.Join[0].On.Op)
	assert.Equal(t, "t.id", req.Join[0].On.Right)
}

func TestParse_JoinInvalidType(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"join": []interface{}{
			map[string]interface{}{
				"type":  "cross",
				"table": "tables",
				"on":    map[string]interface{}{"left": "a.id", "op": "=", "right": "b.id"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JOIN 类型")
}

func TestParse_JoinMissingTable(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"join": []interface{}{
			map[string]interface{}{
				"type": "left",
				"on":   map[string]interface{}{"left": "a.id", "op": "=", "right": "b.id"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table")
}

func TestParse_JoinMissingOn(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"join": []interface{}{
			map[string]interface{}{
				"type":  "left",
				"table": "tables",
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "on")
}

func TestParse_JoinInvalidOnOp(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"join": []interface{}{
			map[string]interface{}{
				"type":  "left",
				"table": "tables",
				"on":    map[string]interface{}{"left": "a.id", "op": "!=", "right": "b.id"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestParse_JoinStringOn(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"join": []interface{}{
			map[string]interface{}{
				"type":  "left",
				"table": "tables",
				"on":    "a.id = b.id",
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_join_condition")
}

func TestParse_UnionQuery(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id", "name"},
		"union": []interface{}{
			map[string]interface{}{
				"from":   "records",
				"select": []string{"id", "name"},
				"where": map[string]interface{}{
					"and": []interface{}{
						map[string]interface{}{"field": "status", "op": "eq", "value": "archived"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)

	require.Len(t, req.Union, 1)
	assert.Equal(t, "records", req.Union[0].From)
}

func TestParse_UnionInvalidQuery(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
		"union": []interface{}{
			map[string]interface{}{
				"select": []string{"id"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "union")
}

func TestParse_IntersectQuery(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
		"intersect": []interface{}{
			map[string]interface{}{
				"from":   "records",
				"select": []string{"id"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	require.Len(t, req.Intersect, 1)
}

func TestParse_IntersectInvalidQuery(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
		"intersect": []interface{}{
			map[string]interface{}{
				"size": float64(99999),
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "intersect")
}

func TestParseBatch_MultipleQueries(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"queries": map[string]interface{}{
			"active": map[string]interface{}{
				"from":   "records",
				"select": []string{"id"},
				"filter": map[string]interface{}{
					"status": "active",
				},
			},
			"archived": map[string]interface{}{
				"from":   "records",
				"select": []string{"id"},
				"filter": map[string]interface{}{
					"status": "archived",
				},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.ParseBatch(data)
	require.NoError(t, err)

	require.Len(t, req.Queries, 2)
	_, ok := req.Queries["active"]
	assert.True(t, ok)
	_, ok = req.Queries["archived"]
	assert.True(t, ok)
}

func TestParseBatch_ErrorInOneQuery(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"queries": map[string]interface{}{
			"good": map[string]interface{}{
				"from":   "records",
				"select": []string{"id"},
			},
			"bad": map[string]interface{}{
				"select": []string{"id"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.ParseBatch(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}

func Test_parseSimplifiedSort(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		sort     string
		expected []OrderByClause
	}{
		{
			"ascending default",
			"name",
			[]OrderByClause{{Field: "name", Dir: "asc"}},
		},
		{
			"explicit ascending",
			"+name",
			[]OrderByClause{{Field: "name", Dir: "asc"}},
		},
		{
			"descending",
			"-created_at",
			[]OrderByClause{{Field: "created_at", Dir: "desc"}},
		},
		{
			"comma separated",
			"-created_at,name,+priority",
			[]OrderByClause{
				{Field: "created_at", Dir: "desc"},
				{Field: "name", Dir: "asc"},
				{Field: "priority", Dir: "asc"},
			},
		},
		{
			"trailing comma",
			"name,",
			[]OrderByClause{{Field: "name", Dir: "asc"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.parseSimplifiedSort(tt.sort)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_parseFilterField_OperatorObjects(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		field    string
		value    interface{}
		expected Condition
	}{
		{
			"eq",
			"status",
			map[string]interface{}{"eq": "active"},
			Condition{Field: "status", Op: "eq", Value: "active"},
		},
		{
			"ne",
			"status",
			map[string]interface{}{"ne": "deleted"},
			Condition{Field: "status", Op: "ne", Value: "deleted"},
		},
		{
			"gt",
			"age",
			map[string]interface{}{"gt": float64(18)},
			Condition{Field: "age", Op: "gt", Value: float64(18)},
		},
		{
			"gte",
			"age",
			map[string]interface{}{"gte": float64(18)},
			Condition{Field: "age", Op: "gte", Value: float64(18)},
		},
		{
			"lt",
			"age",
			map[string]interface{}{"lt": float64(65)},
			Condition{Field: "age", Op: "lt", Value: float64(65)},
		},
		{
			"lte",
			"age",
			map[string]interface{}{"lte": float64(65)},
			Condition{Field: "age", Op: "lte", Value: float64(65)},
		},
		{
			"like",
			"name",
			map[string]interface{}{"like": "%test%"},
			Condition{Field: "name", Op: "like", Value: "%test%"},
		},
		{
			"in",
			"status",
			map[string]interface{}{"in": []interface{}{"a", "b"}},
			Condition{Field: "status", Op: "in", Value: []interface{}{"a", "b"}},
		},
		{
			"between",
			"age",
			map[string]interface{}{"between": []interface{}{float64(10), float64(20)}},
			Condition{Field: "age", Op: "between", Value: []interface{}{float64(10), float64(20)}},
		},
		{
			"is_null",
			"email",
			map[string]interface{}{"is_null": true},
			Condition{Field: "email", Op: "is_null", Value: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.parseFilterField(tt.field, tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_parseFilterField_InvalidOperator(t *testing.T) {
	p := NewParser()
	_, err := p.parseFilterField("status", map[string]interface{}{"unknown_op": "value"})
	assert.Error(t, err)
}

func Test_parseFilterField_DefaultEq(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name  string
		value interface{}
	}{
		{"string", "active"},
		{"float", float64(42)},
		{"bool", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.parseFilterField("status", tt.value)
			require.NoError(t, err)
			assert.Equal(t, "status", result.Field)
			assert.Equal(t, "eq", result.Op)
			assert.Equal(t, tt.value, result.Value)
		})
	}
}

func Test_validateFieldExpression(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		wantErr bool
	}{
		{"simple field", "id", false},
		{"qualified field", "tables.id", false},
		{"three segment", "schema.tables.id", false},
		{"underscore prefix", "_private", false},
		{"json path arrow", "data->>name", false},
		{"json path arrow single", "data->key", false},
		{"json path with dot", "data->>address.city", false},
		{"empty string", "", true},
		{"starts with digit", "1field", true},
		{"contains space", "field name", true},
		{"contains semicolon", "field;name", true},
		{"contains quote", "field'name", true},
		{"contains double quote", "field\"name", true},
		{"contains bracket", "field[0]", true},
		{"empty segment from dot", ".field", true},
		{"trailing dot", "field.", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFieldExpression(tt.field)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"float64 integer", float64(42), int64(42)},
		{"float64 fractional", float64(3.14), float64(3.14)},
		{"string integer", "42", int64(42)},
		{"string float", "3.14", float64(3.14)},
		{"string bool true", "true", true},
		{"string bool false", "false", false},
		{"string non-numeric", "hello", "hello"},
		{"int passthrough", 42, 42},
		{"nil passthrough", nil, nil},
		{"bool passthrough", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParse_DefaultValues(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, 1, req.Page)
	assert.Equal(t, 20, req.Size)
	assert.Equal(t, []string{"*"}, req.Select)
}

func TestParse_TableOverwritesFrom(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":  "records",
		"table": "tables",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, "tables", req.From)
}

func TestParse_TableUsedWhenNoFrom(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"table": "tables",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, "tables", req.From)
}

func TestParse_OrderByDefaultDir(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"orderBy": []interface{}{
			map[string]interface{}{"field": "name"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	require.Len(t, req.OrderBy, 1)
	assert.Equal(t, "asc", req.OrderBy[0].Dir)
}

func TestParse_InvalidJSON(t *testing.T) {
	p := NewParser()
	_, err := p.Parse([]byte(`{invalid`))
	assert.Error(t, err)
}

func TestParseFromMap(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
	}

	req, err := p.ParseFromMap(input)
	require.NoError(t, err)
	assert.Equal(t, "records", req.From)
	assert.Equal(t, []string{"id"}, req.Select)
}

func TestParse_HavingClause(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":     "records",
		"select":   []string{"category"},
		"groupBy":  []string{"category"},
		"aggregate": []interface{}{
			map[string]interface{}{"func": "count", "field": "id", "as": "cnt"},
		},
		"having": map[string]interface{}{
			"and": []interface{}{
				map[string]interface{}{"field": "cnt", "op": "gt", "value": float64(5)},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, req.Having)
	require.Len(t, req.Having.And, 1)
	assert.Equal(t, "cnt", req.Having.And[0].Field)
	assert.Equal(t, "gt", req.Having.And[0].Op)
}

func TestParse_WhereWithOr(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
		"where": map[string]interface{}{
			"or": []interface{}{
				map[string]interface{}{"field": "status", "op": "eq", "value": "active"},
				map[string]interface{}{"field": "status", "op": "eq", "value": "pending"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.Or, 2)
}

func TestParseBatch_InvalidJSON(t *testing.T) {
	p := NewParser()
	_, err := p.ParseBatch([]byte(`{invalid`))
	assert.Error(t, err)
}

func TestNewParserWithLimits(t *testing.T) {
	limits := QueryLimits{
		MaxJoins:    5,
		MaxPageSize: 500,
		MaxDepth:    3,
		MaxFields:   50,
	}
	p := NewParserWithLimits(limits)
	assert.Equal(t, limits, p.limits)
}

func TestParse_CustomLimitsMaxFields(t *testing.T) {
	limits := QueryLimits{
		MaxPageSize: 1000,
		MaxFields:   2,
	}
	p := NewParserWithLimits(limits)

	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"a", "b", "c"},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段数")
}

func TestParse_NegativePageDefaultsToOne(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"page": float64(-1),
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, 1, req.Page)
}

func TestParse_ZeroSizeDefaultsTo20(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"size": float64(0),
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, 20, req.Size)
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "id", false},
		{"valid underscore", "_id", false},
		{"valid qualified", "table.field", false},
		{"valid alphanumeric", "field123", false},
		{"empty", "", true},
		{"starts with digit", "1field", true},
		{"contains hyphen", "my-field", true},
		{"contains space", "my field", true},
		{"dot dot", "a..b", true},
		{"single dot", ".", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIdentifier(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateJoinOp(t *testing.T) {
	assert.NoError(t, ValidateJoinOp("="))
	assert.NoError(t, ValidateJoinOp("<>"))
	assert.Error(t, ValidateJoinOp("!="))
	assert.Error(t, ValidateJoinOp(">"))
	assert.Error(t, ValidateJoinOp("<"))
}

func TestIsValidJoinType(t *testing.T) {
	assert.True(t, isValidJoinType("left"))
	assert.True(t, isValidJoinType("right"))
	assert.True(t, isValidJoinType("inner"))
	assert.True(t, isValidJoinType("outer"))
	assert.False(t, isValidJoinType("cross"))
	assert.False(t, isValidJoinType("full"))
}

func TestIsValidOperator(t *testing.T) {
	validOps := []string{"eq", "ne", "gt", "gte", "lt", "lte", "like", "in", "between", "is_null"}
	for _, op := range validOps {
		assert.True(t, isValidOperator(op), "expected %q to be valid", op)
	}
	assert.False(t, isValidOperator("invalid"))
	assert.False(t, isValidOperator(""))
}

func TestJoinCondition_IsZero(t *testing.T) {
	assert.True(t, JoinCondition{}.IsZero())
	assert.False(t, JoinCondition{Left: "a.id"}.IsZero())
	assert.False(t, JoinCondition{Op: "="}.IsZero())
}

func TestJoinCondition_UnmarshalJSON_StringRejected(t *testing.T) {
	var jc JoinCondition
	err := json.Unmarshal([]byte(`"a.id = b.id"`), &jc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_join_condition")
}

func TestJoinCondition_UnmarshalJSON_Object(t *testing.T) {
	var jc JoinCondition
	err := json.Unmarshal([]byte(`{"left":"a.id","op":"=","right":"b.id"}`), &jc)
	require.NoError(t, err)
	assert.Equal(t, "a.id", jc.Left)
	assert.Equal(t, "=", jc.Op)
	assert.Equal(t, "b.id", jc.Right)
}

func TestJoinCondition_UnmarshalJSON_Null(t *testing.T) {
	var jc JoinCondition
	err := json.Unmarshal([]byte(`null`), &jc)
	require.NoError(t, err)
	assert.True(t, jc.IsZero())
}

func TestParse_JoinAllValidTypes(t *testing.T) {
	for _, joinType := range []string{"left", "right", "inner", "outer"} {
		t.Run(joinType, func(t *testing.T) {
			p := NewParser()
			input := map[string]interface{}{
				"from": "records",
				"join": []interface{}{
					map[string]interface{}{
						"type":  joinType,
						"table": "tables",
						"on":    map[string]interface{}{"left": "a.id", "op": "=", "right": "b.id"},
					},
				},
			}

			data, err := json.Marshal(input)
			require.NoError(t, err)

			req, err := p.Parse(data)
			require.NoError(t, err)
			assert.Equal(t, joinType, req.Join[0].Type)
		})
	}
}

func TestConvertValue_Float32(t *testing.T) {
	result := ConvertValue(float32(10))
	assert.Equal(t, int32(10), result)

	result = ConvertValue(float32(3.14))
	assert.Equal(t, float32(3.14), result)
}

func TestParse_SelectStar(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"*"},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, []string{"*"}, req.Select)
}

func TestParse_SelectInvalidField(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"1invalid"},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestParse_OrderByInvalidField(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"orderBy": []interface{}{
			map[string]interface{}{"field": "bad field!"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestParse_GroupByInvalidField(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":    "records",
		"groupBy": []string{"bad;field"},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestParse_AggregateInvalidField(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"aggregate": []interface{}{
			map[string]interface{}{"func": "count", "field": "bad field!", "as": "cnt"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestParse_AggregateInvalidAs(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"aggregate": []interface{}{
			map[string]interface{}{"func": "count", "field": "id", "as": "1bad"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestParse_AggregateCountStar(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "records",
		"aggregate": []interface{}{
			map[string]interface{}{"func": "count", "field": "*", "as": "total"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	req, err := p.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, "*", req.Aggregate[0].Field)
}

func TestParse_WhereEmptyField(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from":   "records",
		"select": []string{"id"},
		"where": map[string]interface{}{
			"and": []interface{}{
				map[string]interface{}{"field": "", "op": "eq", "value": "x"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestParse_InvalidFromIdentifier(t *testing.T) {
	p := NewParser()
	input := map[string]interface{}{
		"from": "bad table!",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	_, err = p.Parse(data)
	assert.Error(t, err)
}

func TestAllowedTables(t *testing.T) {
	at := DefaultAllowedTables
	assert.True(t, at.IsTableAllowed("records"))
	assert.False(t, at.IsTableAllowed("nonexistent"))
	assert.True(t, at.IsFieldAllowed("records", "id"))
	assert.False(t, at.IsFieldAllowed("records", "nonexistent"))
	assert.NotNil(t, at.GetAllowedFields("records"))
	assert.Nil(t, at.GetAllowedFields("nonexistent"))
}
