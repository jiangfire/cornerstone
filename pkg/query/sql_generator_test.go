package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateFuncValidation(t *testing.T) {
	tests := []struct {
		name  string
		funcs []string
		valid bool
	}{
		{"standard functions", []string{"count", "sum", "avg", "min", "max"}, true},
		{"count_distinct", []string{"count_distinct"}, true},
		{"stddev variants", []string{"stddev", "stddev_pop", "stddev_samp"}, true},
		{"variance variants", []string{"variance", "var_pop", "var_samp"}, true},
		{"invalid function", []string{"invalid"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, fn := range tt.funcs {
				result := isValidAggregateFunc(fn)
				assert.Equal(t, tt.valid, result, "isValidAggregateFunc(%q)", fn)
			}
		})
	}
}

func TestSQLGenerator_StdDevSQLite(t *testing.T) {
	g := NewSQLGenerator(true)

	tests := []struct {
		name     string
		agg      AggregateFunc
		expected string
		hasError bool
	}{
		{
			name:     "stddev with field",
			agg:      AggregateFunc{Func: "stddev", Field: "amount", As: "std_amount"},
			expected: "SQRT(AVG(\"amount\" * \"amount\") - AVG(\"amount\") * AVG(\"amount\")) AS \"std_amount\"",
		},
		{
			name:     "stddev without field",
			agg:      AggregateFunc{Func: "stddev", As: "std"},
			hasError: true,
		},
		{
			name:     "variance with field",
			agg:      AggregateFunc{Func: "variance", Field: "price", As: "var_price"},
			expected: "(AVG(\"price\" * \"price\") - AVG(\"price\") * AVG(\"price\")) AS \"var_price\"",
		},
		{
			name:     "count_distinct with field",
			agg:      AggregateFunc{Func: "count_distinct", Field: "user_id", As: "unique_users"},
			expected: "COUNT(DISTINCT \"user_id\") AS \"unique_users\"",
		},
		{
			name:     "count_distinct without field",
			agg:      AggregateFunc{Func: "count_distinct", As: "cnt"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := g.generateAggregate(tt.agg)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSQLGenerator_HavingClause(t *testing.T) {
	g := NewSQLGenerator(true)

	req := &QueryRequest{
		From:    "records",
		Select:  []string{"table_id"},
		GroupBy: []string{"table_id"},
		Aggregate: []AggregateFunc{
			{Func: "count", Field: "*", As: "cnt"},
		},
		Having: &WhereClause{
			And: []Condition{
				{Field: "cnt", Op: "gt", Value: float64(10)},
			},
		},
		Page: 1,
		Size: 20,
	}

	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, "HAVING")
	assert.Contains(t, query.SQL, "GROUP BY")
}

func TestSQLGenerator_UnionQuery(t *testing.T) {
	g := NewSQLGenerator(true)

	req := &QueryRequest{
		From:   "databases",
		Select: []string{"id", "name"},
		Size:   10,
		Union: []QueryRequest{
			{
				From:   "tables",
				Select: []string{"id", "name"},
			},
		},
	}

	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, "UNION")
}

func TestSQLGenerator_IntersectQuery(t *testing.T) {
	g := NewSQLGenerator(true)

	req := &QueryRequest{
		From:   "databases",
		Select: []string{"id", "name"},
		Intersect: []QueryRequest{
			{
				From:   "tables",
				Select: []string{"id", "name"},
			},
		},
	}

	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, "INTERSECT")
}

func TestParser_HavingValidation(t *testing.T) {
	p := NewParser()

	t.Run("valid having clause", func(t *testing.T) {
		req := &QueryRequest{
			From:    "records",
			Select:  []string{"table_id"},
			GroupBy: []string{"table_id"},
			Aggregate: []AggregateFunc{
				{Func: "count", Field: "*", As: "cnt"},
			},
			Having: &WhereClause{
				And: []Condition{
					{Field: "cnt", Op: "gt", Value: float64(5)},
				},
			},
		}

		err := p.validate(req)
		assert.NoError(t, err)
	})

	t.Run("invalid having field", func(t *testing.T) {
		req := &QueryRequest{
			From:   "records",
			Select: []string{"*"},
			Having: &WhereClause{
				And: []Condition{
					{Field: "invalid field!", Op: "eq", Value: "test"},
				},
			},
		}

		err := p.validate(req)
		assert.Error(t, err)
	})
}

func TestParser_UnionValidation(t *testing.T) {
	p := NewParser()

	t.Run("valid union", func(t *testing.T) {
		req := &QueryRequest{
			From:   "databases",
			Select: []string{"id", "name"},
			Union: []QueryRequest{
				{From: "tables", Select: []string{"id", "name"}},
			},
		}

		err := p.validate(req)
		assert.NoError(t, err)
	})

	t.Run("invalid union from", func(t *testing.T) {
		req := &QueryRequest{
			From:   "databases",
			Select: []string{"id"},
			Union: []QueryRequest{
				{Select: []string{"id"}},
			},
		}

		err := p.validate(req)
		assert.Error(t, err)
	})
}

func TestParser_IntersectValidation(t *testing.T) {
	p := NewParser()

	t.Run("valid intersect", func(t *testing.T) {
		req := &QueryRequest{
			From:   "databases",
			Select: []string{"id", "name"},
			Intersect: []QueryRequest{
				{From: "tables", Select: []string{"id", "name"}},
			},
		}

		err := p.validate(req)
		assert.NoError(t, err)
	})

	t.Run("invalid intersect from", func(t *testing.T) {
		req := &QueryRequest{
			From:   "databases",
			Select: []string{"id"},
			Intersect: []QueryRequest{
				{Select: []string{"id"}},
			},
		}

		err := p.validate(req)
		assert.Error(t, err)
	})
}
