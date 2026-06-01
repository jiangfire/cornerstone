package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_NilRequest(t *testing.T) {
	g := NewSQLGenerator(true)
	query, err := g.Generate(nil)
	assert.Nil(t, query)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询请求不能为空")
}

func TestGenerate_UnionAndIntersect(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "databases",
		Select: []string{"id", "name"},
		Size:   10,
		Union: []QueryRequest{
			{From: "tables", Select: []string{"id", "name"}},
		},
		Intersect: []QueryRequest{
			{From: "fields", Select: []string{"id", "name"}},
		},
	}

	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, "UNION")
	assert.Contains(t, query.SQL, "INTERSECT")
	assert.Contains(t, query.SQL, "combined_result")
}

func TestGenerateSelect_EmptySelect(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{},
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, "SELECT *")
}

func TestGenerateSelect_NilSelect(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From: "records",
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, "SELECT *")
}

func TestGenerateJoins_HappyPath(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Type:  "LEFT",
				Table: "users",
				As:    "u",
				On: JoinCondition{
					Left:  "records.user_id",
					Right: "u.id",
					Op:    "=",
				},
			},
		},
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, `LEFT JOIN "users" AS "u" ON`)
	assert.Contains(t, query.SQL, `"records"."user_id"`)
	assert.Contains(t, query.SQL, `"u"."id"`)
}

func TestGenerateJoins_DefaultJoinType(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Table: "users",
				On: JoinCondition{
					Left:  "records.uid",
					Right: "users.id",
					Op:    "=",
				},
			},
		},
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, `LEFT JOIN "users" ON`)
}

func TestGenerateJoins_InnerJoin(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Type:  "inner",
				Table: "users",
				On: JoinCondition{
					Left:  "records.uid",
					Right: "users.id",
					Op:    "=",
				},
			},
		},
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, `INNER JOIN "users" ON`)
}

func TestGenerateJoins_InvalidTable(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Table: "bad table!",
				On: JoinCondition{
					Left:  "a.id",
					Right: "b.id",
					Op:    "=",
				},
			},
		},
	}
	_, err := g.Generate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "join[0].table")
}

func TestGenerateJoins_InvalidAs(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Table: "users",
				As:    "bad alias!",
				On: JoinCondition{
					Left:  "a.id",
					Right: "b.id",
					Op:    "=",
				},
			},
		},
	}
	_, err := g.Generate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "join[0].as")
}

func TestGenerateJoins_MissingOn(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Table: "users",
			},
		},
	}
	_, err := g.Generate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_join_condition")
}

func TestGenerateJoins_InvalidOnLeft(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Table: "users",
				On: JoinCondition{
					Left:  "bad left!",
					Right: "users.id",
					Op:    "=",
				},
			},
		},
	}
	_, err := g.Generate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "join[0].on.left")
}

func TestGenerateJoins_InvalidOnRight(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Table: "users",
				On: JoinCondition{
					Left:  "records.uid",
					Right: "bad right!",
					Op:    "=",
				},
			},
		},
	}
	_, err := g.Generate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "join[0].on.right")
}

func TestGenerateJoins_InvalidOnOp(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Table: "users",
				On: JoinCondition{
					Left:  "records.uid",
					Right: "users.id",
					Op:    "!=",
				},
			},
		},
	}
	_, err := g.Generate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "join[0].on.op")
}

func TestGenerateJoins_NoJoins(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.NotContains(t, query.SQL, "JOIN")
}

func TestGenerateJoins_MultipleJoins(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Type:  "LEFT",
				Table: "users",
				As:    "u",
				On:    JoinCondition{Left: "records.uid", Right: "u.id", Op: "="},
			},
			{
				Type:  "INNER",
				Table: "orders",
				As:    "o",
				On:    JoinCondition{Left: "records.order_id", Right: "o.id", Op: "="},
			},
		},
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, `LEFT JOIN "users" AS "u"`)
	assert.Contains(t, query.SQL, `INNER JOIN "orders" AS "o"`)
}

func TestGenerateCondition_Ne(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "status", Op: "ne", Value: "inactive"})
	require.NoError(t, err)
	assert.Equal(t, `"status" != ?`, sql)
	assert.Equal(t, []interface{}{"inactive"}, params)
}

func TestGenerateCondition_Gte(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "age", Op: "gte", Value: 18})
	require.NoError(t, err)
	assert.Equal(t, `"age" >= ?`, sql)
	assert.Equal(t, []interface{}{18}, params)
}

func TestGenerateCondition_Lt(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "age", Op: "lt", Value: 65})
	require.NoError(t, err)
	assert.Equal(t, `"age" < ?`, sql)
	assert.Equal(t, []interface{}{65}, params)
}

func TestGenerateCondition_Lte(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "score", Op: "lte", Value: 100})
	require.NoError(t, err)
	assert.Equal(t, `"score" <= ?`, sql)
	assert.Equal(t, []interface{}{100}, params)
}

func TestGenerateCondition_Like(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "name", Op: "like", Value: "john"})
	require.NoError(t, err)
	assert.Equal(t, `"name" LIKE ?`, sql)
	assert.Equal(t, []interface{}{"%john%"}, params)
}

func TestGenerateCondition_LikeWithWildcard(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "name", Op: "like", Value: "%john"})
	require.NoError(t, err)
	assert.Equal(t, `"name" LIKE ?`, sql)
	assert.Equal(t, []interface{}{"%john"}, params)
}

func TestGenerateCondition_In(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{
		Field: "status",
		Op:    "in",
		Value: []interface{}{"active", "pending"},
	})
	require.NoError(t, err)
	assert.Equal(t, `"status" IN (?, ?)`, sql)
	assert.Equal(t, []interface{}{"active", "pending"}, params)
}

func TestGenerateCondition_In_NotArray(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{Field: "status", Op: "in", Value: "active"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'in' 操作符需要数组值")
}

func TestGenerateCondition_In_EmptyArray(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{Field: "status", Op: "in", Value: []interface{}{}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'in' 操作符数组不能为空")
}

func TestGenerateCondition_Between(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{
		Field: "age",
		Op:    "between",
		Value: []interface{}{18, 65},
	})
	require.NoError(t, err)
	assert.Equal(t, `"age" BETWEEN ? AND ?`, sql)
	assert.Equal(t, []interface{}{18, 65}, params)
}

func TestGenerateCondition_Between_Invalid(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{Field: "age", Op: "between", Value: 18})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'between' 操作符需要包含两个值的数组")
}

func TestGenerateCondition_Between_WrongLength(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{
		Field: "age",
		Op:    "between",
		Value: []interface{}{1, 2, 3},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'between' 操作符需要包含两个值的数组")
}

func TestGenerateCondition_IsNull(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "deleted_at", Op: "is_null", Value: true})
	require.NoError(t, err)
	assert.Equal(t, `"deleted_at" IS NULL`, sql)
	assert.Nil(t, params)
}

func TestGenerateCondition_IsNull_False(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "deleted_at", Op: "is_null", Value: false})
	require.NoError(t, err)
	assert.Equal(t, `"deleted_at" IS NULL`, sql)
	assert.Nil(t, params)
}

func TestGenerateCondition_IsNull_NilValue(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "deleted_at", Op: "is_null"})
	require.NoError(t, err)
	assert.Equal(t, `"deleted_at" IS NULL`, sql)
	assert.Nil(t, params)
}

func TestGenerateCondition_UnknownOp(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{Field: "name", Op: "regex", Value: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知的操作符")
}

func TestGenerateCondition_EmptyOp(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "name", Op: "", Value: "test"})
	require.NoError(t, err)
	assert.Equal(t, `"name" = ?`, sql)
	assert.Equal(t, []interface{}{"test"}, params)
}

func TestGenerateCondition_NestedAnd(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{
		And: []Condition{
			{Field: "a", Op: "eq", Value: 1},
			{Field: "b", Op: "eq", Value: 2},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, `("a" = ? AND "b" = ?)`, sql)
	assert.Equal(t, []interface{}{1, 2}, params)
}

func TestGenerateCondition_NestedOr(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{
		Or: []Condition{
			{Field: "a", Op: "eq", Value: 1},
			{Field: "b", Op: "eq", Value: 2},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, `("a" = ? OR "b" = ?)`, sql)
	assert.Equal(t, []interface{}{1, 2}, params)
}

func TestGenerateCondition_NotPrefix(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "name", Op: "eq", Value: "test", Not: true})
	require.NoError(t, err)
	assert.Equal(t, `NOT "name" = ?`, sql)
	assert.Equal(t, []interface{}{"test"}, params)
}

func TestGenerateCondition_NotNe(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, _, err := g.generateCondition(Condition{Field: "status", Op: "ne", Value: "inactive", Not: true})
	require.NoError(t, err)
	assert.Equal(t, `NOT "status" != ?`, sql)
}

func TestGenerateCondition_NotIn(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, _, err := g.generateCondition(Condition{
		Field: "status",
		Op:    "in",
		Value: []interface{}{"active", "pending"},
		Not:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, `NOT "status" IN (?, ?)`, sql)
}

func TestGenerateCondition_NotBetween(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, _, err := g.generateCondition(Condition{
		Field: "age",
		Op:    "between",
		Value: []interface{}{18, 65},
		Not:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, `NOT "age" BETWEEN ? AND ?`, sql)
}

func TestGenerateCondition_NotIsNull(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateCondition(Condition{Field: "deleted_at", Op: "is_null", Value: true, Not: true})
	require.NoError(t, err)
	assert.Equal(t, `"deleted_at" IS NOT NULL`, sql)
	assert.Nil(t, params)
}

func TestGenerateCondition_NotLike(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, _, err := g.generateCondition(Condition{Field: "name", Op: "like", Value: "test", Not: true})
	require.NoError(t, err)
	assert.Equal(t, `NOT "name" LIKE ?`, sql)
}

func TestGenerateCondition_NestedAndError(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{
		And: []Condition{
			{Field: "a", Op: "unknown_op", Value: 1},
		},
	})
	assert.Error(t, err)
}

func TestGenerateCondition_NestedOrError(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{
		Or: []Condition{
			{Field: "a", Op: "unknown_op", Value: 1},
		},
	})
	assert.Error(t, err)
}

func TestGenerateCondition_InvalidField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, _, err := g.generateCondition(Condition{Field: "bad field!", Op: "eq", Value: 1})
	assert.Error(t, err)
}

func TestGenerateWhere_OrConditions(t *testing.T) {
	g := NewSQLGenerator(true)
	where := &WhereClause{
		Or: []Condition{
			{Field: "status", Op: "eq", Value: "active"},
			{Field: "status", Op: "eq", Value: "pending"},
		},
	}
	sql, params, err := g.generateWhere(where)
	require.NoError(t, err)
	assert.Equal(t, `("status" = ? OR "status" = ?)`, sql)
	assert.Equal(t, []interface{}{"active", "pending"}, params)
}

func TestGenerateWhere_AndAndOr(t *testing.T) {
	g := NewSQLGenerator(true)
	where := &WhereClause{
		And: []Condition{
			{Field: "active", Op: "eq", Value: true},
		},
		Or: []Condition{
			{Field: "status", Op: "eq", Value: "active"},
			{Field: "status", Op: "eq", Value: "pending"},
		},
	}
	sql, params, err := g.generateWhere(where)
	require.NoError(t, err)
	assert.Contains(t, sql, `"active" = ?`)
	assert.Contains(t, sql, `("status" = ? OR "status" = ?)`)
	assert.Contains(t, sql, " AND ")
	assert.Equal(t, 3, len(params))
}

func TestGenerateWhere_EmptyClause(t *testing.T) {
	g := NewSQLGenerator(true)
	where := &WhereClause{}
	sql, params, err := g.generateWhere(where)
	require.NoError(t, err)
	assert.Equal(t, "", sql)
	assert.Nil(t, params)
}

func TestGenerateWhere_NilClause(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, params, err := g.generateWhere(nil)
	require.NoError(t, err)
	assert.Equal(t, "", sql)
	assert.Nil(t, params)
}

func TestGenerateWhere_ConditionError(t *testing.T) {
	g := NewSQLGenerator(true)
	where := &WhereClause{
		And: []Condition{
			{Field: "status", Op: "bad_op", Value: "x"},
		},
	}
	_, _, err := g.generateWhere(where)
	assert.Error(t, err)
}

func TestGenerateWhere_OrConditionError(t *testing.T) {
	g := NewSQLGenerator(true)
	where := &WhereClause{
		Or: []Condition{
			{Field: "status", Op: "bad_op", Value: "x"},
		},
	}
	_, _, err := g.generateWhere(where)
	assert.Error(t, err)
}

func TestGenerateFieldExpression_JSONArrowSyntax(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("data->>name")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("data", '$.name')`, sql)
}

func TestGenerateFieldExpression_JSONArrowSingle(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("data->name")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("data", '$.name')`, sql)
}

func TestGenerateFieldExpression_JSONArrowPostgres(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateFieldExpression("data->>name")
	require.NoError(t, err)
	assert.Equal(t, `"data"->>'name'`, sql)
}

func TestGenerateFieldExpression_DotJSONColumn(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("data.status")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("data", '$.status')`, sql)
}

func TestGenerateFieldExpression_DotJSONColumnPostgres(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateFieldExpression("data.status")
	require.NoError(t, err)
	assert.Equal(t, `"data"->>'status'`, sql)
}

func TestGenerateFieldExpression_OptionsColumn(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("options.theme")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("options", '$.theme')`, sql)
}

func TestGenerateFieldExpression_ConfigColumn(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("config.mode")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("config", '$.mode')`, sql)
}

func TestGenerateFieldExpression_ConfigValuesColumn(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("config_values.key")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("config_values", '$.key')`, sql)
}

func TestGenerateFieldExpression_ThreeSegmentPath(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("alias.data.key")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("alias"."data", '$.key')`, sql)
}

func TestGenerateFieldExpression_ThreeSegmentPathPostgres(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateFieldExpression("alias.data.key")
	require.NoError(t, err)
	assert.Equal(t, `"alias"."data"->>'key'`, sql)
}

func TestGenerateFieldExpression_QualifiedNonJSON(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("t1.field")
	require.NoError(t, err)
	assert.Equal(t, `"t1"."field"`, sql)
}

func TestGenerateFieldExpression_SimpleField(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("name")
	require.NoError(t, err)
	assert.Equal(t, `"name"`, sql)
}

func TestGenerateFieldExpression_EmptyField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateFieldExpression("")
	assert.Error(t, err)
}

func TestGenerateFieldExpression_WhitespaceOnlyField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateFieldExpression("   ")
	assert.Error(t, err)
}

func TestGenerateFieldExpression_InvalidField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateFieldExpression("bad field!")
	assert.Error(t, err)
}

func TestGenerateFieldExpression_InvalidJSONArrowBase(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateFieldExpression("bad base!->>name")
	assert.Error(t, err)
}

func TestGenerateFieldExpression_InvalidJSONArrowPath(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateFieldExpression("data->>bad path!")
	assert.Error(t, err)
}

func TestGenerateFieldExpression_InvalidDotIdentifier(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateFieldExpression("t1.bad field!")
	assert.Error(t, err)
}

func TestGenerateJSONFieldExpression_SQLite(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateJSONFieldExpression("data", "status")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("data", '$.status')`, sql)
}

func TestGenerateJSONFieldExpression_SQLite_DottedPath(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateJSONFieldExpression("data", "user.name")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("data", '$.user.name')`, sql)
}

func TestGenerateJSONFieldExpression_Postgres(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateJSONFieldExpression("data", "status")
	require.NoError(t, err)
	assert.Equal(t, `"data"->>'status'`, sql)
}

func TestGenerateJSONFieldExpression_Postgres_DottedPath(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateJSONFieldExpression("data", "user.name")
	require.NoError(t, err)
	assert.Equal(t, `"data"->>'user.name'`, sql)
}

func TestGenerateJSONFieldExpression_QualifiedField(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateJSONFieldExpression("t1.data", "key")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("t1"."data", '$.key')`, sql)
}

func TestGenerateJSONFieldExpression_InvalidField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateJSONFieldExpression("bad field!", "key")
	assert.Error(t, err)
}

func TestGenerateJSONFieldExpression_InvalidPath(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateJSONFieldExpression("data", "bad path!")
	assert.Error(t, err)
}

func TestGenerateAggregate_StandardWithField(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateAggregate(AggregateFunc{Func: "sum", Field: "amount", As: "total"})
	require.NoError(t, err)
	assert.Equal(t, `SUM("amount") AS "total"`, sql)
}

func TestGenerateAggregate_StandardWithoutField(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateAggregate(AggregateFunc{Func: "count", As: "cnt"})
	require.NoError(t, err)
	assert.Equal(t, `COUNT(*) AS "cnt"`, sql)
}

func TestGenerateAggregate_StandardWithStar(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateAggregate(AggregateFunc{Func: "count", Field: "*", As: "cnt"})
	require.NoError(t, err)
	assert.Equal(t, `COUNT(*) AS "cnt"`, sql)
}

func TestGenerateAggregate_InvalidAlias(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateAggregate(AggregateFunc{Func: "count", As: "bad alias!"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aggregate.as")
}

func TestGenerateAggregate_StdDevNonSQLite(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateAggregate(AggregateFunc{Func: "stddev", Field: "amount", As: "std_amount"})
	require.NoError(t, err)
	assert.Equal(t, `STDDEV("amount") AS "std_amount"`, sql)
}

func TestGenerateAggregate_StdDevPopNonSQLite(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateAggregate(AggregateFunc{Func: "stddev_pop", Field: "amount", As: "std_pop"})
	require.NoError(t, err)
	assert.Equal(t, `STDDEV_POP("amount") AS "std_pop"`, sql)
}

func TestGenerateAggregate_StdDevSampNonSQLite(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateAggregate(AggregateFunc{Func: "stddev_samp", Field: "amount", As: "std_samp"})
	require.NoError(t, err)
	assert.Equal(t, `STDDEV_SAMP("amount") AS "std_samp"`, sql)
}

func TestGenerateAggregate_StdDevWithoutField(t *testing.T) {
	g := NewSQLGenerator(false)
	_, err := g.generateAggregate(AggregateFunc{Func: "stddev", As: "std"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "STDDEV")
}

func TestGenerateAggregate_VarianceNonSQLite(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateAggregate(AggregateFunc{Func: "variance", Field: "price", As: "var_price"})
	require.NoError(t, err)
	assert.Equal(t, `VARIANCE("price") AS "var_price"`, sql)
}

func TestGenerateAggregate_VarPopNonSQLite(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateAggregate(AggregateFunc{Func: "var_pop", Field: "price", As: "var_pop"})
	require.NoError(t, err)
	assert.Equal(t, `VAR_POP("price") AS "var_pop"`, sql)
}

func TestGenerateAggregate_VarSampNonSQLite(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateAggregate(AggregateFunc{Func: "var_samp", Field: "price", As: "var_samp"})
	require.NoError(t, err)
	assert.Equal(t, `VAR_SAMP("price") AS "var_samp"`, sql)
}

func TestGenerateAggregate_VarianceWithoutField(t *testing.T) {
	g := NewSQLGenerator(false)
	_, err := g.generateAggregate(AggregateFunc{Func: "variance", As: "var"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VARIANCE")
}

func TestGenerateAggregate_DefaultFuncWithField(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateAggregate(AggregateFunc{Func: "avg", Field: "score", As: "avg_score"})
	require.NoError(t, err)
	assert.Equal(t, `AVG("score") AS "avg_score"`, sql)
}

func TestGenerateAggregate_DefaultFuncInvalidField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateAggregate(AggregateFunc{Func: "sum", Field: "bad field!", As: "total"})
	assert.Error(t, err)
}

func TestGenerateAggregate_StdDevInvalidField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateAggregate(AggregateFunc{Func: "stddev", Field: "bad field!", As: "std"})
	assert.Error(t, err)
}

func TestGenerateAggregate_VarianceInvalidField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateAggregate(AggregateFunc{Func: "variance", Field: "bad field!", As: "var"})
	assert.Error(t, err)
}

func TestGenerateAggregate_CountDistinctInvalidField(t *testing.T) {
	g := NewSQLGenerator(true)
	_, err := g.generateAggregate(AggregateFunc{Func: "count_distinct", Field: "bad field!", As: "cnt"})
	assert.Error(t, err)
}

func TestGenerateAggregate_WithJSONField(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateAggregate(AggregateFunc{Func: "sum", Field: "data.amount", As: "total"})
	require.NoError(t, err)
	assert.Contains(t, sql, "JSON_EXTRACT")
	assert.Contains(t, sql, `$.amount`)
}

func TestGenerateAggregate_CountDistinctWithField(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateAggregate(AggregateFunc{Func: "count_distinct", Field: "user_id", As: "unique_users"})
	require.NoError(t, err)
	assert.Equal(t, `COUNT(DISTINCT "user_id") AS "unique_users"`, sql)
}

func TestIsJSONColumnCandidate(t *testing.T) {
	assert.True(t, isJSONColumnCandidate("data"))
	assert.True(t, isJSONColumnCandidate("options"))
	assert.True(t, isJSONColumnCandidate("config"))
	assert.True(t, isJSONColumnCandidate("config_values"))
	assert.False(t, isJSONColumnCandidate("name"))
	assert.False(t, isJSONColumnCandidate("id"))
	assert.False(t, isJSONColumnCandidate("status"))
}

func TestGenerateJoins_HappyPathWithValidOnOp(t *testing.T) {
	g := NewSQLGenerator(true)
	req := &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Type:  "LEFT",
				Table: "users",
				As:    "u",
				On: JoinCondition{
					Left:  "records.uid",
					Right: "u.id",
					Op:    "<>",
				},
			},
		},
	}
	query, err := g.Generate(req)
	require.NoError(t, err)
	assert.Contains(t, query.SQL, `LEFT JOIN "users" AS "u" ON`)
	assert.Contains(t, query.SQL, `<>`)
}

func TestGenerateFieldExpression_DataArrowNestedPath(t *testing.T) {
	g := NewSQLGenerator(true)
	sql, err := g.generateFieldExpression("data->>user.name")
	require.NoError(t, err)
	assert.Equal(t, `JSON_EXTRACT("data", '$.user.name')`, sql)
}

func TestGenerateFieldExpression_DataArrowNestedPathPostgres(t *testing.T) {
	g := NewSQLGenerator(false)
	sql, err := g.generateFieldExpression("data->>user.name")
	require.NoError(t, err)
	assert.Equal(t, `"data"->>'user.name'`, sql)
}
