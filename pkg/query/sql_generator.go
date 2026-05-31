package query

import (
	"fmt"
	"strings"
)

// SQLGenerator SQL 生成器
type SQLGenerator struct {
	isSQLite bool
	maxRows  int64
}

// NewSQLGenerator 创建 SQL 生成器
func NewSQLGenerator(isSQLite bool) *SQLGenerator {
	return &SQLGenerator{isSQLite: isSQLite, maxRows: DefaultLimits.MaxRows}
}

// NewSQLGeneratorWithConfig 创建带自定义 maxRows 的 SQL 生成器
func NewSQLGeneratorWithConfig(isSQLite bool, maxRows int64) *SQLGenerator {
	return &SQLGenerator{isSQLite: isSQLite, maxRows: maxRows}
}

// Generate 生成 SQL 查询
func (g *SQLGenerator) Generate(req *QueryRequest) (*SQLQuery, error) {
	if req == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	// 1. 生成主查询
	mainQuery, err := g.generateSingleQuery(req)
	if err != nil {
		return nil, err
	}

	return g.combineQueries(req, mainQuery)
}

func (g *SQLGenerator) combineQueries(req *QueryRequest, mainQuery *SQLQuery) (*SQLQuery, error) {
	if len(req.Union) == 0 && len(req.Intersect) == 0 {
		return mainQuery, nil
	}

	parts := []string{mainQuery.SQL}
	allParams := append([]interface{}{}, mainQuery.Params...)

	for i, unionReq := range req.Union {
		unionQuery, err := g.generateSingleQuery(&unionReq)
		if err != nil {
			return nil, fmt.Errorf("union[%d]: %w", i, err)
		}
		parts = append(parts, unionQuery.SQL)
		allParams = append(allParams, unionQuery.Params...)
	}

	finalSQL := strings.Join(parts, " UNION ")
	if len(req.Intersect) > 0 {
		intersects := []string{finalSQL}
		for i, intersectReq := range req.Intersect {
			intersectQuery, err := g.generateSingleQuery(&intersectReq)
			if err != nil {
				return nil, fmt.Errorf("intersect[%d]: %w", i, err)
			}
			intersects = append(intersects, intersectQuery.SQL)
			allParams = append(allParams, intersectQuery.Params...)
		}
		finalSQL = strings.Join(intersects, " INTERSECT ")
	}

	if len(req.OrderBy) > 0 || req.Size > 0 {
		orderByClause, err := g.generateOrderBy(req)
		if err != nil {
			return nil, err
		}
		limitClause, limitParams := g.generateLimit(req)
		allParams = append(allParams, limitParams...)

		wrappedSQL := "SELECT * FROM (" + finalSQL + ") AS combined_result"
		if orderByClause != "" {
			wrappedSQL += " ORDER BY " + orderByClause
		}
		finalSQL = wrappedSQL + limitClause
	}

	return &SQLQuery{SQL: finalSQL, Params: allParams}, nil
}

// generateSingleQuery 生成单个查询（不包含 UNION）
func (g *SQLGenerator) generateSingleQuery(req *QueryRequest) (*SQLQuery, error) {
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
	groupByClause, err := g.generateGroupBy(req)
	if err != nil {
		return nil, err
	}

	// 6. 生成 HAVING 子句
	havingClause, havingParams, err := g.generateWhere(req.Having)
	if err != nil {
		return nil, err
	}
	query.Params = append(query.Params, havingParams...)

	// 7. 生成 ORDER BY 子句
	orderByClause, err := g.generateOrderBy(req)
	if err != nil {
		return nil, err
	}

	// 8. 生成分页子句
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
	if havingClause != "" {
		sql += " HAVING " + havingClause
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
			expr, err := g.generateFieldExpression(f)
			if err != nil {
				return "", err
			}
			fields = append(fields, expr)
		}

		// 添加聚合函数
		for _, agg := range req.Aggregate {
			aggSQL, err := g.generateAggregate(agg)
			if err != nil {
				return "", err
			}
			fields = append(fields, aggSQL)
		}
	} else {
		fields = make([]string, 0, len(req.Select))
		for _, f := range req.Select {
			expr, err := g.generateFieldExpression(f)
			if err != nil {
				return "", err
			}
			fields = append(fields, expr)
		}
	}

	if len(fields) == 0 {
		fields = []string{"*"}
	}

	return "SELECT " + strings.Join(fields, ", "), nil
}

// generateAggregate 生成聚合函数 SQL
func (g *SQLGenerator) generateAggregate(agg AggregateFunc) (string, error) {
	funcName := strings.ToUpper(agg.Func)
	field := agg.Field

	if err := ValidateIdentifier(agg.As); err != nil {
		return "", fmt.Errorf("aggregate.as %w", err)
	}

	// 处理特殊聚合函数
	switch funcName {
	case "COUNT_DISTINCT":
		if field == "" || field == "*" {
			return "", fmt.Errorf("count_distinct 需要指定字段")
		}
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		return fmt.Sprintf("COUNT(DISTINCT %s) AS %s", fieldExpr, g.quoteIdentifier(agg.As)), nil

	case "STDDEV", "STDDEV_POP", "STDDEV_SAMP":
		if field == "" || field == "*" {
			return "", fmt.Errorf("%s 需要指定数值字段", funcName)
		}
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		if g.isSQLite {
			// SQLite 没有原生 STDDEV，用公式计算
			// stddev = sqrt(avg(x^2) - avg(x)^2)
			return fmt.Sprintf("SQRT(AVG(%s * %s) - AVG(%s) * AVG(%s)) AS %s", fieldExpr, fieldExpr, fieldExpr, fieldExpr, g.quoteIdentifier(agg.As)), nil
		}
		return fmt.Sprintf("%s(%s) AS %s", funcName, fieldExpr, g.quoteIdentifier(agg.As)), nil

	case "VARIANCE", "VAR_POP", "VAR_SAMP":
		if field == "" || field == "*" {
			return "", fmt.Errorf("%s 需要指定数值字段", funcName)
		}
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		if g.isSQLite {
			// SQLite 没有原生 VARIANCE，用公式计算
			// variance = avg(x^2) - avg(x)^2
			return fmt.Sprintf("(AVG(%s * %s) - AVG(%s) * AVG(%s)) AS %s", fieldExpr, fieldExpr, fieldExpr, fieldExpr, g.quoteIdentifier(agg.As)), nil
		}
		return fmt.Sprintf("%s(%s) AS %s", funcName, fieldExpr, g.quoteIdentifier(agg.As)), nil

	default:
		// 标准聚合函数: COUNT, SUM, AVG, MIN, MAX
		if field == "" || field == "*" {
			return fmt.Sprintf("%s(*) AS %s", funcName, g.quoteIdentifier(agg.As)), nil
		}

		// 处理 JSON 字段路径
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		return fmt.Sprintf("%s(%s) AS %s", funcName, fieldExpr, g.quoteIdentifier(agg.As)), nil
	}
}

// generateFrom 生成 FROM 子句
func (g *SQLGenerator) generateFrom(req *QueryRequest) string {
	return " FROM " + g.quoteIdentifier(req.From)
}

// generateJoins 生成 JOIN 子句。
// 历史实现把 join.On 原样 `fmt.Sprintf` 到 SQL，认证用户可通过
// `1=1; DROP TABLE users; --` 等载荷注入；现在改为只用结构化 JoinCondition，
// 两侧都过 ValidateIdentifier、op 走白名单。详见 docs/REVIEW-FIX-PLAN-2026-05.md P1-3。
func (g *SQLGenerator) generateJoins(req *QueryRequest) (string, error) {
	if len(req.Join) == 0 {
		return "", nil
	}

	var joins []string
	for i, join := range req.Join {
		joinType := strings.ToUpper(join.Type)
		if joinType == "" {
			joinType = "LEFT"
		}

		if err := ValidateIdentifier(join.Table); err != nil {
			return "", fmt.Errorf("join[%d].table %w", i, err)
		}
		tableRef := g.quoteIdentifier(join.Table)
		if join.As != "" {
			if err := ValidateIdentifier(join.As); err != nil {
				return "", fmt.Errorf("join[%d].as %w", i, err)
			}
			tableRef += " AS " + g.quoteIdentifier(join.As)
		}

		if join.On.IsZero() {
			return "", fmt.Errorf("invalid_join_condition: join[%d] 缺少 on", i)
		}
		if err := ValidateIdentifier(join.On.Left); err != nil {
			return "", fmt.Errorf("join[%d].on.left %w", i, err)
		}
		if err := ValidateIdentifier(join.On.Right); err != nil {
			return "", fmt.Errorf("join[%d].on.right %w", i, err)
		}
		if err := ValidateJoinOp(join.On.Op); err != nil {
			return "", fmt.Errorf("join[%d].on.op %w", i, err)
		}

		onSQL := g.quoteQualifiedIdentifier(join.On.Left) +
			" " + join.On.Op + " " +
			g.quoteQualifiedIdentifier(join.On.Right)

		joins = append(joins, fmt.Sprintf(" %s JOIN %s ON %s", joinType, tableRef, onSQL))
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
	fieldExpr, err := g.generateFieldExpression(cond.Field)
	if err != nil {
		return "", nil, err
	}

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

// generateFieldExpression 生成字段表达式。
//
// 历史实现把 `data->>name` 与 `JSON_EXTRACT(data, '$.name')` 中的字段名/路径直接拼进 SQL，
// 引号字符可直接破出字面量；现在每个段都过 ValidateIdentifier / ValidateJSONPathSegment，
// 详见 docs/REVIEW-FIX-PLAN-2026-05.md P1-4。
func (g *SQLGenerator) generateFieldExpression(field string) (string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", fmt.Errorf("字段名不能为空")
	}

	// Postgres 形式 `data->>name` 或 `data->name`：等价于 data 列上的 JSON 路径
	if strings.Contains(field, "->") {
		parts := strings.SplitN(field, "->", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("非法字段名 %q：JSON 引用格式错误", field)
		}
		base := strings.TrimSpace(parts[0])
		path := strings.TrimSpace(parts[1])
		path = strings.TrimPrefix(path, ">")
		path = strings.Trim(path, "'\"")
		if err := ValidateIdentifier(base); err != nil {
			return "", err
		}
		if err := ValidateJSONPath(path); err != nil {
			return "", err
		}
		return g.generateJSONFieldExpression(base, path)
	}

	// 处理带表别名的字段
	if strings.Contains(field, ".") {
		if err := ValidateIdentifier(field); err != nil {
			return "", err
		}
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

			return g.quoteQualifiedIdentifier(strings.Join(parts, ".")), nil
		}
	}

	if err := ValidateIdentifier(field); err != nil {
		return "", err
	}
	return g.quoteIdentifier(field), nil
}

// generateJSONFieldExpression 生成 JSON 字段表达式。
// 调用者必须保证 jsonField 与 path 已经过 ValidateIdentifier / ValidateJSONPath；
// 此处仍重复校验一次作为防御性深度防御（in-depth defense）。
func (g *SQLGenerator) generateJSONFieldExpression(jsonField, path string) (string, error) {
	if err := ValidateIdentifier(jsonField); err != nil {
		return "", err
	}
	if err := ValidateJSONPath(path); err != nil {
		return "", err
	}
	if g.isSQLite {
		// SQLite: JSON_EXTRACT(data, '$.status')
		return fmt.Sprintf("JSON_EXTRACT(%s, '$.%s')", g.quoteQualifiedIdentifier(jsonField), path), nil
	}
	// PostgreSQL: data->>'status' 返回 text
	return fmt.Sprintf("%s->>'%s'", g.quoteQualifiedIdentifier(jsonField), path), nil
}

func isJSONColumnCandidate(field string) bool {
	return field == "data" || field == "options" || field == "config" || field == "config_values"
}

// generateGroupBy 生成 GROUP BY 子句
func (g *SQLGenerator) generateGroupBy(req *QueryRequest) (string, error) {
	if len(req.GroupBy) == 0 {
		return "", nil
	}

	fields := make([]string, len(req.GroupBy))
	for i, f := range req.GroupBy {
		expr, err := g.generateFieldExpression(f)
		if err != nil {
			return "", err
		}
		fields[i] = expr
	}

	return strings.Join(fields, ", "), nil
}

// generateOrderBy 生成 ORDER BY 子句
func (g *SQLGenerator) generateOrderBy(req *QueryRequest) (string, error) {
	if len(req.OrderBy) == 0 {
		// 默认按 created_at 降序
		return "", nil
	}

	orders := make([]string, len(req.OrderBy))
	for i, o := range req.OrderBy {
		fieldExpr, err := g.generateFieldExpression(o.Field)
		if err != nil {
			return "", err
		}
		dir := strings.ToUpper(o.Dir)
		if dir != "ASC" && dir != "DESC" {
			dir = "ASC"
		}
		orders[i] = fieldExpr + " " + dir
	}

	return strings.Join(orders, ", "), nil
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

	if g.maxRows > 0 && req.Size > int(g.maxRows) {
		req.Size = int(g.maxRows)
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
