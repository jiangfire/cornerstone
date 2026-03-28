package query

import (
	"fmt"
	"strings"
)

// SQLGenerator SQL 生成器
type SQLGenerator struct {
	isSQLite bool
}

// NewSQLGenerator 创建 SQL 生成器
func NewSQLGenerator(isSQLite bool) *SQLGenerator {
	return &SQLGenerator{isSQLite: isSQLite}
}

// Generate 生成 SQL 查询
func (g *SQLGenerator) Generate(req *QueryRequest) (*SQLQuery, error) {
	if req == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	query := &SQLQuery{
		Params: make([]interface{}, 0),
	}

	// 1. 生成 SELECT 子句
	selectClause, err := g.generateSelect(req)
	if err != nil {
		return nil, err
	}

	// 2. 生成 FROM 子句
	fromClause := g.generateFrom(req)

	// 3. 生成 JOIN 子句
	joinClause, err := g.generateJoins(req)
	if err != nil {
		return nil, err
	}

	// 4. 生成 WHERE 子句
	whereClause, whereParams, err := g.generateWhere(req.Where)
	if err != nil {
		return nil, err
	}
	query.Params = append(query.Params, whereParams...)

	// 5. 生成 GROUP BY 子句
	groupByClause := g.generateGroupBy(req)

	// 6. 生成 ORDER BY 子句
	orderByClause := g.generateOrderBy(req)

	// 7. 生成分页子句
	limitClause, limitParams := g.generateLimit(req)
	query.Params = append(query.Params, limitParams...)

	// 组装 SQL
	sql := selectClause + fromClause + joinClause
	if whereClause != "" {
		sql += " WHERE " + whereClause
	}
	if groupByClause != "" {
		sql += " GROUP BY " + groupByClause
	}
	if orderByClause != "" {
		sql += " ORDER BY " + orderByClause
	}
	sql += limitClause

	query.SQL = sql
	return query, nil
}

// GenerateCount 生成 COUNT 查询
func (g *SQLGenerator) GenerateCount(req *QueryRequest) (*SQLQuery, error) {
	if req == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	baseReq := *req
	baseReq.Page = 0
	baseReq.Size = -1
	baseReq.OrderBy = nil

	baseQuery, err := g.Generate(&baseReq)
	if err != nil {
		return nil, err
	}

	return &SQLQuery{
		SQL:    "SELECT COUNT(*) as total FROM (" + baseQuery.SQL + ") AS query_count",
		Params: baseQuery.Params,
	}, nil
}

// generateSelect 生成 SELECT 子句
func (g *SQLGenerator) generateSelect(req *QueryRequest) (string, error) {
	var fields []string

	// 处理聚合查询
	if len(req.Aggregate) > 0 {
		fields = make([]string, 0, len(req.Select)+len(req.Aggregate))

		// 添加普通字段
		for _, f := range req.Select {
			fields = append(fields, g.generateFieldExpression(f))
		}

		// 添加聚合函数
		for _, agg := range req.Aggregate {
			aggSQL := g.generateAggregate(agg)
			fields = append(fields, aggSQL)
		}
	} else {
		fields = make([]string, 0, len(req.Select))
		for _, f := range req.Select {
			fields = append(fields, g.generateFieldExpression(f))
		}
	}

	if len(fields) == 0 {
		fields = []string{"*"}
	}

	return "SELECT " + strings.Join(fields, ", "), nil
}

// generateAggregate 生成聚合函数 SQL
func (g *SQLGenerator) generateAggregate(agg AggregateFunc) string {
	funcName := strings.ToUpper(agg.Func)
	field := agg.Field

	if field == "" || field == "*" {
		return fmt.Sprintf("%s(*) AS %s", funcName, g.quoteIdentifier(agg.As))
	}

	// 处理 JSON 字段路径
	fieldExpr := g.generateFieldExpression(field)
	return fmt.Sprintf("%s(%s) AS %s", funcName, fieldExpr, g.quoteIdentifier(agg.As))
}

// generateFrom 生成 FROM 子句
func (g *SQLGenerator) generateFrom(req *QueryRequest) string {
	return " FROM " + g.quoteIdentifier(req.From)
}

// generateJoins 生成 JOIN 子句
func (g *SQLGenerator) generateJoins(req *QueryRequest) (string, error) {
	if len(req.Join) == 0 {
		return "", nil
	}

	var joins []string
	for _, join := range req.Join {
		joinType := strings.ToUpper(join.Type)
		if joinType == "" {
			joinType = "LEFT"
		}

		tableRef := g.quoteIdentifier(join.Table)
		if join.As != "" {
			tableRef += " AS " + g.quoteIdentifier(join.As)
		}

		joinSQL := fmt.Sprintf(" %s JOIN %s ON %s", joinType, tableRef, join.On)
		joins = append(joins, joinSQL)

		// TODO: 处理关联表的字段选择（子查询方式）
	}

	return strings.Join(joins, ""), nil
}

// generateWhere 生成 WHERE 子句
func (g *SQLGenerator) generateWhere(where *WhereClause) (string, []interface{}, error) {
	if where == nil {
		return "", nil, nil
	}

	var conditions []string
	var params []interface{}

	// 处理 AND 条件
	for _, cond := range where.And {
		sql, condParams, err := g.generateCondition(cond)
		if err != nil {
			return "", nil, err
		}
		if sql != "" {
			conditions = append(conditions, sql)
			params = append(params, condParams...)
		}
	}

	// 处理 OR 条件
	if len(where.Or) > 0 {
		var orConditions []string
		var orParams []interface{}

		for _, cond := range where.Or {
			sql, condParams, err := g.generateCondition(cond)
			if err != nil {
				return "", nil, err
			}
			if sql != "" {
				orConditions = append(orConditions, sql)
				orParams = append(orParams, condParams...)
			}
		}

		if len(orConditions) > 0 {
			conditions = append(conditions, "("+strings.Join(orConditions, " OR ")+")")
			params = append(params, orParams...)
		}
	}

	if len(conditions) == 0 {
		return "", nil, nil
	}

	return strings.Join(conditions, " AND "), params, nil
}

// generateCondition 生成单个条件 SQL
func (g *SQLGenerator) generateCondition(cond Condition) (string, []interface{}, error) {
	var params []interface{}

	// 处理嵌套 AND
	if len(cond.And) > 0 {
		var nestedConditions []string
		for _, nested := range cond.And {
			sql, nestedParams, err := g.generateCondition(nested)
			if err != nil {
				return "", nil, err
			}
			nestedConditions = append(nestedConditions, sql)
			params = append(params, nestedParams...)
		}
		return "(" + strings.Join(nestedConditions, " AND ") + ")", params, nil
	}

	// 处理嵌套 OR
	if len(cond.Or) > 0 {
		var nestedConditions []string
		for _, nested := range cond.Or {
			sql, nestedParams, err := g.generateCondition(nested)
			if err != nil {
				return "", nil, err
			}
			nestedConditions = append(nestedConditions, sql)
			params = append(params, nestedParams...)
		}
		return "(" + strings.Join(nestedConditions, " OR ") + ")", params, nil
	}

	// 处理字段表达式
	fieldExpr := g.generateFieldExpression(cond.Field)

	// 根据操作符生成 SQL
	op := cond.Op
	if op == "" {
		op = "eq"
	}

	// 处理 NOT
	notPrefix := ""
	if cond.Not {
		notPrefix = "NOT "
	}

	switch op {
	case "eq":
		return notPrefix + fieldExpr + " = ?", []interface{}{cond.Value}, nil
	case "ne":
		return notPrefix + fieldExpr + " != ?", []interface{}{cond.Value}, nil
	case "gt":
		return notPrefix + fieldExpr + " > ?", []interface{}{cond.Value}, nil
	case "gte":
		return notPrefix + fieldExpr + " >= ?", []interface{}{cond.Value}, nil
	case "lt":
		return notPrefix + fieldExpr + " < ?", []interface{}{cond.Value}, nil
	case "lte":
		return notPrefix + fieldExpr + " <= ?", []interface{}{cond.Value}, nil
	case "like":
		value := cond.Value
		if str, ok := value.(string); ok {
			// 自动添加 % 通配符
			if !strings.Contains(str, "%") {
				value = "%" + str + "%"
			}
		}
		return notPrefix + fieldExpr + " LIKE ?", []interface{}{value}, nil
	case "in":
		values, ok := cond.Value.([]interface{})
		if !ok {
			return "", nil, fmt.Errorf("'in' 操作符需要数组值")
		}
		if len(values) == 0 {
			return "", nil, fmt.Errorf("'in' 操作符数组不能为空")
		}
		placeholders := make([]string, len(values))
		for i := range values {
			placeholders[i] = "?"
			params = append(params, values[i])
		}
		return notPrefix + fieldExpr + " IN (" + strings.Join(placeholders, ", ") + ")", params, nil
	case "between":
		values, ok := cond.Value.([]interface{})
		if !ok || len(values) != 2 {
			return "", nil, fmt.Errorf("'between' 操作符需要包含两个值的数组")
		}
		return notPrefix + fieldExpr + " BETWEEN ? AND ?", []interface{}{values[0], values[1]}, nil
	case "is_null":
		if value, ok := cond.Value.(bool); ok && value {
			return fieldExpr + " IS " + notPrefix + "NULL", nil, nil
		}
		return fieldExpr + " IS " + notPrefix + "NULL", nil, nil
	default:
		return "", nil, fmt.Errorf("未知的操作符: %s", op)
	}
}

// generateFieldExpression 生成字段表达式
func (g *SQLGenerator) generateFieldExpression(field string) string {
	if strings.Contains(field, "->") {
		return field
	}

	// 处理带表别名的字段
	if strings.Contains(field, ".") {
		parts := strings.Split(field, ".")
		if len(parts) >= 2 {
			first := parts[0]
			second := parts[1]

			if isJSONColumnCandidate(first) {
				return g.generateJSONFieldExpression(first, strings.Join(parts[1:], "."))
			}

			if len(parts) >= 3 && isJSONColumnCandidate(second) {
				return g.generateJSONFieldExpression(
					first+"."+second,
					strings.Join(parts[2:], "."),
				)
			}

			return g.quoteQualifiedIdentifier(strings.Join(parts, "."))
		}
	}

	return g.quoteIdentifier(field)
}

// generateJSONFieldExpression 生成 JSON 字段表达式
func (g *SQLGenerator) generateJSONFieldExpression(jsonField, path string) string {
	if g.isSQLite {
		// SQLite: JSON_EXTRACT(data, '$.status')
		return fmt.Sprintf("JSON_EXTRACT(%s, '$.%s')", g.quoteQualifiedIdentifier(jsonField), path)
	}
	// PostgreSQL: data->>'status' (返回文本) 或 data->'status' (返回 JSON)
	// 这里使用 ->> 返回文本形式
	return fmt.Sprintf("%s->>'%s'", g.quoteQualifiedIdentifier(jsonField), path)
}

func isJSONColumnCandidate(field string) bool {
	return field == "data" || field == "options" || field == "config" || field == "config_values"
}

// generateGroupBy 生成 GROUP BY 子句
func (g *SQLGenerator) generateGroupBy(req *QueryRequest) string {
	if len(req.GroupBy) == 0 {
		return ""
	}

	fields := make([]string, len(req.GroupBy))
	for i, f := range req.GroupBy {
		fields[i] = g.generateFieldExpression(f)
	}

	return strings.Join(fields, ", ")
}

// generateOrderBy 生成 ORDER BY 子句
func (g *SQLGenerator) generateOrderBy(req *QueryRequest) string {
	if len(req.OrderBy) == 0 {
		// 默认按 created_at 降序
		return ""
	}

	orders := make([]string, len(req.OrderBy))
	for i, o := range req.OrderBy {
		fieldExpr := g.generateFieldExpression(o.Field)
		dir := strings.ToUpper(o.Dir)
		if dir != "ASC" && dir != "DESC" {
			dir = "ASC"
		}
		orders[i] = fieldExpr + " " + dir
	}

	return strings.Join(orders, ", ")
}

// generateLimit 生成分页子句
func (g *SQLGenerator) generateLimit(req *QueryRequest) (string, []interface{}) {
	if req.Size < 0 {
		return "", nil
	}

	if req.Size <= 0 {
		req.Size = 20
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	offset := (req.Page - 1) * req.Size
	return " LIMIT ? OFFSET ?", []interface{}{req.Size, offset}
}

// quoteIdentifier 引用标识符
func (g *SQLGenerator) quoteIdentifier(name string) string {
	if name == "*" {
		return name
	}
	// 使用双引号引用标识符（PostgreSQL 标准）
	// SQLite 也支持双引号
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}

func (g *SQLGenerator) quoteQualifiedIdentifier(name string) string {
	parts := strings.Split(name, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, g.quoteIdentifier(part))
	}
	return strings.Join(quoted, ".")
}

// SQLQuery 生成的 SQL 查询
type SQLQuery struct {
	SQL    string
	Params []interface{}
}
