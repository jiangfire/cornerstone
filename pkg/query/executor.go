package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jiangfire/cornerstone/pkg/db"
	"gorm.io/gorm"
)

// Executor executes queries.
type Executor struct {
	db        *gorm.DB
	parser    *Parser
	validator *Validator
	generator *SQLGenerator
	limits    QueryLimits
}

// NewExecutor creates a query executor.
func NewExecutor(database *gorm.DB) *Executor {
	return &Executor{
		db:        database,
		parser:    NewParser(),
		validator: NewValidator(database),
		generator: NewSQLGeneratorWithDBType(dbType(database)),
		limits:    DefaultLimits,
	}
}

// NewExecutorWithConfig creates an executor with custom config.
func NewExecutorWithConfig(database *gorm.DB, limits QueryLimits, tables AllowedTables) *Executor {
	return &Executor{
		db:        database,
		parser:    NewParserWithLimits(limits),
		validator: NewValidatorWithTables(database, tables),
		generator: NewSQLGeneratorWithDBType(dbType(database)),
		limits:    limits,
	}
}

// dbType returns the database type.
func dbType(gormDB *gorm.DB) string {
	if gormDB == nil {
		return ""
	}
	return gormDB.Name()
}

// Execute runs a query.
func (e *Executor) Execute(ctx context.Context, req *QueryRequest, userID string) (*QueryResult, error) {
	req = cloneQueryRequest(req)

	// 1. Normalize and validate request
	if err := e.Prepare(ctx, req, userID); err != nil {
		return nil, err
	}

	// 2. generate query SQL
	query, err := e.generator.Generate(req)
	if err != nil {
		return nil, fmt.Errorf("SQL generation failed: %w", err)
	}

	// 3. generate COUNT SQL
	countQuery, err := e.generator.GenerateCount(req)
	if err != nil {
		return nil, fmt.Errorf("COUNT SQL generation failed: %w", err)
	}

	// 4. execute query
	data, err := e.executeQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	// 5. get total count
	total, err := e.executeCount(ctx, countQuery)
	if err != nil {
		return nil, fmt.Errorf("total count query failed: %w", err)
	}

	// 6. Build result
	result := &QueryResult{
		Data:    data,
		Total:   total,
		Page:    req.Page,
		Size:    req.Size,
		HasMore: int64((req.Page-1)*req.Size+len(data)) < total,
	}

	return result, nil
}

// Prepare normalizes, authorizes, and injects permission filters.
func (e *Executor) Prepare(ctx context.Context, req *QueryRequest, userID string) error {
	if err := e.normalize(req); err != nil {
		return err
	}

	scope, err := e.validator.newAccessScope(userID)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	if err := e.validator.validateRequestWithScope(ctx, req, userID, scope); err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	if err := e.validator.autoFilterByPermissionWithScope(req, scope); err != nil {
		return err
	}

	e.expandWildcardSelections(req)

	return nil
}

// ExecuteRaw executes a raw JSON query.
func (e *Executor) ExecuteRaw(ctx context.Context, jsonData []byte, userID string) (*QueryResult, error) {
	// 1. Parse request
	req, err := e.parser.Parse(jsonData)
	if err != nil {
		return nil, err
	}

	// 2. Execute query
	return e.Execute(ctx, req, userID)
}

// ExecuteFromMap executes a query from a map.
func (e *Executor) ExecuteFromMap(ctx context.Context, data map[string]interface{}, userID string) (*QueryResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("serialization failed: %w", err)
	}
	return e.ExecuteRaw(ctx, jsonData, userID)
}

// ExecuteBatch executes a batch of queries.
func (e *Executor) ExecuteBatch(ctx context.Context, req *BatchQueryRequest, userID string) (*BatchQueryResult, error) {
	results := make(map[string]*QueryResult)

	for name, query := range req.Queries {
		result, err := e.Execute(ctx, &query, userID)
		if err != nil {
			return nil, fmt.Errorf("query '%s' execution failed: %w", name, err)
		}
		results[name] = result
	}

	return &BatchQueryResult{Results: results}, nil
}

// ExecuteBatchRaw executes a raw JSON batch query.
func (e *Executor) ExecuteBatchRaw(ctx context.Context, jsonData []byte, userID string) (*BatchQueryResult, error) {
	req, err := e.parser.ParseBatch(jsonData)
	if err != nil {
		return nil, err
	}
	return e.ExecuteBatch(ctx, req, userID)
}

// executeQuery executes a query and returns results.
func (e *Executor) executeQuery(ctx context.Context, query *SQLQuery) ([]map[string]interface{}, error) {
	rows, err := e.db.WithContext(ctx).Raw(query.SQL, query.Params...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return e.scanRows(rows)
}

// executeCount executes a COUNT query.
func (e *Executor) executeCount(ctx context.Context, query *SQLQuery) (int64, error) {
	var total int64
	err := e.db.WithContext(ctx).Raw(query.SQL, query.Params...).Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

// scanRows scans query results.
func (e *Executor) scanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, 16)
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		// Scan row
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Build result map
		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			val := values[i]

			// Handle byte arrays (e.g., JSONB)
			if b, ok := val.([]byte); ok {
				row[col] = decodeScannedBytes(b)
			} else {
				row[col] = val
			}
			values[i] = nil
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func decodeScannedBytes(b []byte) interface{} {
	if !looksLikeJSONValue(b) {
		return string(b)
	}

	var jsonVal interface{}
	if err := json.Unmarshal(b, &jsonVal); err == nil {
		return jsonVal
	}
	return string(b)
}

func looksLikeJSONValue(b []byte) bool {
	if len(b) == 0 {
		return false
	}

	switch b[0] {
	case '{', '[', '"':
		return true
	case 't':
		return string(b) == "true"
	case 'f':
		return string(b) == "false"
	case 'n':
		return string(b) == "null"
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return looksLikeJSONNumber(b)
	default:
		return false
	}
}

func looksLikeJSONNumber(b []byte) bool {
	s := string(b)
	if strings.ContainsAny(s, " :-/") {
		return false
	}

	seenDigit := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= '0' && ch <= '9':
			seenDigit = true
		case ch == '-' && i == 0:
		case ch == '.' || ch == 'e' || ch == 'E' || ch == '+':
		default:
			return false
		}
	}
	return seenDigit
}

// Validate validates a query request (without executing).
func (e *Executor) Validate(ctx context.Context, req *QueryRequest, userID string) error {
	req = cloneQueryRequest(req)
	return e.Prepare(ctx, req, userID)
}

// Explain explains a query (returns generated SQL).
func (e *Executor) Explain(req *QueryRequest) (*SQLQuery, error) {
	return e.generator.Generate(req)
}

// ExplainAuthorized generates SQL after applying permission filters.
func (e *Executor) ExplainAuthorized(ctx context.Context, req *QueryRequest, userID string) (*SQLQuery, error) {
	req = cloneQueryRequest(req)
	if err := e.Prepare(ctx, req, userID); err != nil {
		return nil, err
	}
	return e.generator.Generate(req)
}

// GetParser returns the parser.
func (e *Executor) GetParser() *Parser {
	return e.parser
}

// GetValidator returns the validator.
func (e *Executor) GetValidator() *Validator {
	return e.validator
}

// GetGenerator returns the SQL generator.
func (e *Executor) GetGenerator() *SQLGenerator {
	return e.generator
}

// DB returns the database connection.
func (e *Executor) DB() *gorm.DB {
	return e.db
}

// WithDB creates an executor copy using the given database.
func (e *Executor) WithDB(database *gorm.DB) *Executor {
	return &Executor{
		db:        database,
		parser:    e.parser,
		validator: e.validator,
		generator: e.generator,
		limits:    e.limits,
	}
}

// SimplifiedQuery is a simplified query interface for simple single-table queries.
func (e *Executor) SimplifiedQuery(ctx context.Context, table string, filter map[string]interface{}, sort string, page, size int, userID string) (*QueryResult, error) {
	req := &QueryRequest{
		Table:  table,
		Filter: filter,
		Sort:   sort,
		Page:   page,
		Size:   size,
	}

	// Normalize
	if err := e.normalize(req); err != nil {
		return nil, err
	}

	return e.Execute(ctx, req, userID)
}

// normalize normalizes the request (internal use, skips parser validation).
func (e *Executor) normalize(req *QueryRequest) error {
	// Handle simplified syntax
	if req.Table != "" {
		req.From = req.Table
	}

	// Set defaults
	if req.From == "" {
		return fmt.Errorf("table name is required")
	}

	if req.Page <= 0 {
		req.Page = 1
	}

	if req.Size <= 0 {
		req.Size = 20
	}

	// Convert simplified filter to Where
	if len(req.Filter) > 0 && req.Where == nil {
		where, err := e.parser.parseSimplifiedFilter(req.Filter)
		if err != nil {
			return err
		}
		req.Where = where
	}

	// Convert simplified sort to OrderBy
	if req.Sort != "" && len(req.OrderBy) == 0 {
		orderBy, err := e.parser.parseSimplifiedSort(req.Sort)
		if err != nil {
			return err
		}
		req.OrderBy = orderBy
	}

	// Default to all fields if select is not specified
	if len(req.Select) == 0 {
		req.Select = []string{"*"}
	}

	return nil
}

func (e *Executor) expandWildcardSelections(req *QueryRequest) {
	if req == nil {
		return
	}

	if len(req.Select) == 1 && req.Select[0] == "*" {
		if fields := e.validator.GetSelectableFields(req.From); len(fields) > 0 {
			req.Select = fields
		}
	}
}

func cloneQueryRequest(req *QueryRequest) *QueryRequest {
	if req == nil {
		return nil
	}

	cloned := *req
	cloned.Select = append([]string(nil), req.Select...)
	cloned.GroupBy = append([]string(nil), req.GroupBy...)
	cloned.Aggregate = append([]AggregateFunc(nil), req.Aggregate...)
	cloned.OrderBy = append([]OrderByClause(nil), req.OrderBy...)
	cloned.Join = cloneJoinClauses(req.Join)
	cloned.Where = cloneWhereClause(req.Where)
	cloned.Having = cloneWhereClause(req.Having)
	cloned.Union = cloneQueryRequestSlice(req.Union)
	cloned.Intersect = cloneQueryRequestSlice(req.Intersect)
	cloned.Filter = cloneStringAnyMap(req.Filter)
	return &cloned
}

func cloneQueryRequestSlice(items []QueryRequest) []QueryRequest {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]QueryRequest, len(items))
	for i := range items {
		item := cloneQueryRequest(&items[i])
		cloned[i] = *item
	}
	return cloned
}

func cloneJoinClauses(items []JoinClause) []JoinClause {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]JoinClause, len(items))
	for i := range items {
		cloned[i] = items[i]
		cloned[i].Select = append([]string(nil), items[i].Select...)
	}
	return cloned
}

func cloneWhereClause(where *WhereClause) *WhereClause {
	if where == nil {
		return nil
	}
	cloned := &WhereClause{
		And: cloneConditions(where.And),
		Or:  cloneConditions(where.Or),
		Raw: cloneStringAnyMap(where.Raw),
	}
	return cloned
}

func cloneConditions(items []Condition) []Condition {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]Condition, len(items))
	for i := range items {
		cloned[i] = items[i]
		cloned[i].And = cloneConditions(items[i].And)
		cloned[i].Or = cloneConditions(items[i].Or)
	}
	return cloned
}

func cloneStringAnyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

// GlobalExecutor is the global executor instance (singleton).
var GlobalExecutor *Executor

// InitGlobalExecutor initializes the global executor.
func InitGlobalExecutor(database *gorm.DB) {
	GlobalExecutor = NewExecutor(database)
}

// GetGlobalExecutor returns the global executor.
func GetGlobalExecutor() *Executor {
	if GlobalExecutor == nil {
		// Try to initialize from global DB
		if database := db.DB(); database != nil {
			InitGlobalExecutor(database)
		}
	}
	return GlobalExecutor
}
