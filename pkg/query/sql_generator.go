package query

import (
	"fmt"
	"strings"
)

// SQLGenerator generates SQL queries.
type SQLGenerator struct {
	dbType  string // "sqlite", "postgres", "mysql"
	maxRows int64
}

// NewSQLGenerator creates a SQL generator (legacy compat, takes isSQLite bool).
func NewSQLGenerator(isSQLite bool) *SQLGenerator {
	dbType := "postgres"
	if isSQLite {
		dbType = "sqlite"
	}
	return &SQLGenerator{dbType: dbType, maxRows: DefaultLimits.MaxRows}
}

// NewSQLGeneratorWithDBType creates a SQL generator with the given DB type.
func NewSQLGeneratorWithDBType(dbType string) *SQLGenerator {
	return &SQLGenerator{dbType: dbType, maxRows: DefaultLimits.MaxRows}
}

// NewSQLGeneratorWithConfig creates a SQL generator with a custom maxRows (legacy compat).
func NewSQLGeneratorWithConfig(isSQLite bool, maxRows int64) *SQLGenerator {
	dbType := "postgres"
	if isSQLite {
		dbType = "sqlite"
	}
	return &SQLGenerator{dbType: dbType, maxRows: maxRows}
}

// Generate generates the SQL query.
func (g *SQLGenerator) Generate(req *QueryRequest) (*SQLQuery, error) {
	if req == nil {
		return nil, fmt.Errorf("query request cannot be nil")
	}

	// 1. Generate main query
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

// generateSingleQuery generates a single query (without UNION).
func (g *SQLGenerator) generateSingleQuery(req *QueryRequest) (*SQLQuery, error) {
	query := &SQLQuery{
		Params: make([]interface{}, 0),
	}

	// 1. Generate SELECT clause
	selectClause, err := g.generateSelect(req)
	if err != nil {
		return nil, err
	}

	// 2. Generate FROM clause
	fromClause := g.generateFrom(req)

	// 3. Generate JOIN clause
	joinClause, err := g.generateJoins(req)
	if err != nil {
		return nil, err
	}

	// 4. Generate WHERE clause
	whereClause, whereParams, err := g.generateWhere(req.Where)
	if err != nil {
		return nil, err
	}
	query.Params = append(query.Params, whereParams...)

	// 5. Generate GROUP BY clause
	groupByClause, err := g.generateGroupBy(req)
	if err != nil {
		return nil, err
	}

	// 6. Generate HAVING clause
	havingClause, havingParams, err := g.generateWhere(req.Having)
	if err != nil {
		return nil, err
	}
	query.Params = append(query.Params, havingParams...)

	// 7. Generate ORDER BY clause
	orderByClause, err := g.generateOrderBy(req)
	if err != nil {
		return nil, err
	}

	// 8. Generate pagination clause
	limitClause, limitParams := g.generateLimit(req)
	query.Params = append(query.Params, limitParams...)

	// Assemble SQL
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

// GenerateCount generates a COUNT query.
func (g *SQLGenerator) GenerateCount(req *QueryRequest) (*SQLQuery, error) {
	if req == nil {
		return nil, fmt.Errorf("query request cannot be nil")
	}

	if g.canUseDirectCount(req) {
		return g.generateDirectCount(req)
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

func (g *SQLGenerator) canUseDirectCount(req *QueryRequest) bool {
	if req == nil {
		return false
	}
	if len(req.Union) > 0 || len(req.Intersect) > 0 {
		return false
	}
	if len(req.GroupBy) > 0 || len(req.Aggregate) > 0 || req.Having != nil {
		return false
	}
	return true
}

func (g *SQLGenerator) generateDirectCount(req *QueryRequest) (*SQLQuery, error) {
	query := &SQLQuery{
		Params: make([]interface{}, 0),
	}

	fromClause := g.generateFrom(req)
	joinClause, err := g.generateJoins(req)
	if err != nil {
		return nil, err
	}

	whereClause, whereParams, err := g.generateWhere(req.Where)
	if err != nil {
		return nil, err
	}
	query.Params = append(query.Params, whereParams...)

	sql := "SELECT COUNT(*) FROM" + fromClause[len(" FROM"):] + joinClause
	if whereClause != "" {
		sql += " WHERE " + whereClause
	}
	query.SQL = sql
	return query, nil
}

// generateSelect generates the SELECT clause.
func (g *SQLGenerator) generateSelect(req *QueryRequest) (string, error) {
	var fields []string

	// Handle aggregate queries
	if len(req.Aggregate) > 0 {
		fields = make([]string, 0, len(req.Select)+len(req.Aggregate))

		// Add regular fields
		for _, f := range req.Select {
			expr, err := g.generateFieldExpression(f)
			if err != nil {
				return "", err
			}
			fields = append(fields, expr)
		}

		// Add aggregate functions
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

// generateAggregate generates aggregate function SQL.
func (g *SQLGenerator) generateAggregate(agg AggregateFunc) (string, error) {
	funcName := strings.ToUpper(agg.Func)
	field := agg.Field

	if err := ValidateIdentifier(agg.As); err != nil {
		return "", fmt.Errorf("aggregate.as %w", err)
	}

	// Handle special aggregate functions
	switch funcName {
	case "COUNT_DISTINCT":
		if field == "" || field == "*" {
			return "", fmt.Errorf("count_distinct requires a field")
		}
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		return fmt.Sprintf("COUNT(DISTINCT %s) AS %s", fieldExpr, g.quoteIdentifier(agg.As)), nil

	case "STDDEV", "STDDEV_POP", "STDDEV_SAMP":
		if field == "" || field == "*" {
			return "", fmt.Errorf("%s requires a numeric field", funcName)
		}
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		if g.dbType == "sqlite" {
			// SQLite lacks native STDDEV; compute via formula
			// stddev = sqrt(avg(x^2) - avg(x)^2)
			return fmt.Sprintf("SQRT(AVG(%s * %s) - AVG(%s) * AVG(%s)) AS %s", fieldExpr, fieldExpr, fieldExpr, fieldExpr, g.quoteIdentifier(agg.As)), nil
		}
		// MySQL 8.0+ does not support STDDEV (Oracle compat alias); map to STDDEV_SAMP
		mysqlFunc := funcName
		if g.dbType == "mysql" && funcName == "STDDEV" {
			mysqlFunc = "STDDEV_SAMP"
		}
		return fmt.Sprintf("%s(%s) AS %s", mysqlFunc, fieldExpr, g.quoteIdentifier(agg.As)), nil

	case "VARIANCE", "VAR_POP", "VAR_SAMP":
		if field == "" || field == "*" {
			return "", fmt.Errorf("%s requires a numeric field", funcName)
		}
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		if g.dbType == "sqlite" {
			// SQLite lacks native VARIANCE; compute via formula
			// variance = avg(x^2) - avg(x)^2
			return fmt.Sprintf("(AVG(%s * %s) - AVG(%s) * AVG(%s)) AS %s", fieldExpr, fieldExpr, fieldExpr, fieldExpr, g.quoteIdentifier(agg.As)), nil
		}
		// MySQL 8.0+ does not support VARIANCE (Oracle compat alias); map to VAR_SAMP
		mysqlFunc := funcName
		if g.dbType == "mysql" && funcName == "VARIANCE" {
			mysqlFunc = "VAR_SAMP"
		}
		return fmt.Sprintf("%s(%s) AS %s", mysqlFunc, fieldExpr, g.quoteIdentifier(agg.As)), nil

	default:
		// Standard aggregate functions: COUNT, SUM, AVG, MIN, MAX
		if field == "" || field == "*" {
			return fmt.Sprintf("%s(*) AS %s", funcName, g.quoteIdentifier(agg.As)), nil
		}

		// Handle JSON field path
		fieldExpr, err := g.generateFieldExpression(field)
		if err != nil {
			return "", fmt.Errorf("aggregate.field %w", err)
		}
		return fmt.Sprintf("%s(%s) AS %s", funcName, fieldExpr, g.quoteIdentifier(agg.As)), nil
	}
}

// generateFrom generates the FROM clause.
func (g *SQLGenerator) generateFrom(req *QueryRequest) string {
	return " FROM " + g.quoteIdentifier(req.From)
}

// generateJoins generates JOIN clauses.
// The legacy implementation concatenated join.On directly via fmt.Sprintf into SQL,
// allowing authenticated users to inject payloads like `1=1; DROP TABLE users; --`.
// Now only structured JoinCondition is used, both sides pass ValidateIdentifier,
// and the operator is whitelisted. See docs/REVIEW-FIX-PLAN-2026-05.md P1-3.
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
			return "", fmt.Errorf("invalid_join_condition: join[%d] missing on", i)
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

// generateWhere generates the WHERE clause.
func (g *SQLGenerator) generateWhere(where *WhereClause) (string, []interface{}, error) {
	if where == nil {
		return "", nil, nil
	}

	var conditions []string
	var params []interface{}

	// Process AND conditions
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

	// Process OR conditions
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

// generateCondition generates SQL for a single condition.
func (g *SQLGenerator) generateCondition(cond Condition) (string, []interface{}, error) {
	var params []interface{}

	// Handle nested AND
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

	// Handle nested OR
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

	// Handle field expression
	fieldExpr, err := g.generateFieldExpression(cond.Field)
	if err != nil {
		return "", nil, err
	}

	// Generate SQL based on operator
	op := cond.Op
	if op == "" {
		op = "eq"
	}

	// Handle NOT
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
			// Auto-add % wildcard
			if !strings.Contains(str, "%") {
				value = "%" + str + "%"
			}
		}
		return notPrefix + fieldExpr + " LIKE ?", []interface{}{value}, nil
	case "in":
		values, ok := cond.Value.([]interface{})
		if !ok {
			return "", nil, fmt.Errorf("'in' operator requires an array value")
		}
		if len(values) == 0 {
			return "", nil, fmt.Errorf("'in' operator array cannot be empty")
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
			return "", nil, fmt.Errorf("'between' operator requires an array with two values")
		}
		return notPrefix + fieldExpr + " BETWEEN ? AND ?", []interface{}{values[0], values[1]}, nil
	case "is_null":
		if value, ok := cond.Value.(bool); ok && value {
			return fieldExpr + " IS " + notPrefix + "NULL", nil, nil
		}
		return fieldExpr + " IS " + notPrefix + "NULL", nil, nil
	default:
		return "", nil, fmt.Errorf("unknown operator: %s", op)
	}
}

// generateFieldExpression generates a field expression.
//
// The legacy implementation concatenated field names/paths from `data->>name` and
// `JSON_EXTRACT(data, '$.name')` directly into SQL, allowing quote characters to
// break out of literals. Now every segment passes ValidateIdentifier / ValidateJSONPathSegment.
// See docs/REVIEW-FIX-PLAN-2026-05.md P1-4.
func (g *SQLGenerator) generateFieldExpression(field string) (string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", fmt.Errorf("field name cannot be empty")
	}

	// Postgres-style `data->>name` or `data->name`: equivalent to JSON path on the data column
	if strings.Contains(field, "->") {
		parts := strings.SplitN(field, "->", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid field name %q: JSON reference format error", field)
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

	// Handle fields with table alias
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

// generateJSONFieldExpression generates a JSON field expression.
// The caller must ensure jsonField and path have already passed ValidateIdentifier / ValidateJSONPath;
// redundant validation here serves as defense-in-depth.
func (g *SQLGenerator) generateJSONFieldExpression(jsonField, path string) (string, error) {
	if err := ValidateIdentifier(jsonField); err != nil {
		return "", err
	}
	if err := ValidateJSONPath(path); err != nil {
		return "", err
	}
	switch g.dbType {
	case "sqlite", "mysql":
		// SQLite / MySQL: JSON_EXTRACT(data, '$.status')
		return fmt.Sprintf("JSON_EXTRACT(%s, '$.%s')", g.quoteQualifiedIdentifier(jsonField), path), nil
	default:
		// PostgreSQL: data->>'status' returns text
		return fmt.Sprintf("%s->>'%s'", g.quoteQualifiedIdentifier(jsonField), path), nil
	}
}

func isJSONColumnCandidate(field string) bool {
	return field == "data" || field == "options" || field == "config" || field == "config_values"
}

// generateGroupBy generates the GROUP BY clause.
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

// generateOrderBy generates the ORDER BY clause.
func (g *SQLGenerator) generateOrderBy(req *QueryRequest) (string, error) {
	if len(req.OrderBy) == 0 {
		// Default sort by created_at descending
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

// generateLimit generates the pagination clause.
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

// quoteIdentifier quotes an identifier.
func (g *SQLGenerator) quoteIdentifier(name string) string {
	if name == "*" {
		return name
	}
	// MySQL uses backticks, PostgreSQL/SQLite use double quotes
	if g.dbType == "mysql" {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
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

// SQLQuery is a generated SQL query.
type SQLQuery struct {
	SQL    string
	Params []interface{}
}
