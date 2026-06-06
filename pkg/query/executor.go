package query

import (
	"context"
	"database/sql"
	"encoding/json"
	stdjson "encoding/json"
	"fmt"
	"strings"

	"github.com/jiangfire/cornerstone/pkg/db"
	"gorm.io/gorm"
)

// Executor 查询执行器
type Executor struct {
	db        *gorm.DB
	parser    *Parser
	validator *Validator
	generator *SQLGenerator
	limits    QueryLimits
}

// NewExecutor 创建查询执行器
func NewExecutor(database *gorm.DB) *Executor {
	return &Executor{
		db:        database,
		parser:    NewParser(),
		validator: NewValidator(database),
		generator: NewSQLGeneratorWithDBType(dbType(database)),
		limits:    DefaultLimits,
	}
}

// NewExecutorWithConfig 创建带自定义配置的查询执行器
func NewExecutorWithConfig(database *gorm.DB, limits QueryLimits, tables AllowedTables) *Executor {
	return &Executor{
		db:        database,
		parser:    NewParserWithLimits(limits),
		validator: NewValidatorWithTables(database, tables),
		generator: NewSQLGeneratorWithDBType(dbType(database)),
		limits:    limits,
	}
}

// dbType 获取数据库类型
func dbType(gormDB *gorm.DB) string {
	if gormDB == nil {
		return ""
	}
	return gormDB.Name()
}

// Execute 执行查询
func (e *Executor) Execute(ctx context.Context, req *QueryRequest, userID string) (*QueryResult, error) {
	req = cloneQueryRequest(req)

	// 1. 规范化和验证请求
	if err := e.Prepare(ctx, req, userID); err != nil {
		return nil, err
	}

	// 2. 生成查询 SQL
	query, err := e.generator.Generate(req)
	if err != nil {
		return nil, fmt.Errorf("SQL生成失败: %w", err)
	}

	// 3. 生成 COUNT SQL
	countQuery, err := e.generator.GenerateCount(req)
	if err != nil {
		return nil, fmt.Errorf("COUNT SQL生成失败: %w", err)
	}

	// 4. 执行查询
	data, err := e.executeQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询执行失败: %w", err)
	}

	// 5. 获取总数
	total, err := e.executeCount(ctx, countQuery)
	if err != nil {
		return nil, fmt.Errorf("总数查询失败: %w", err)
	}

	// 6. 构建结果
	result := &QueryResult{
		Data:    data,
		Total:   total,
		Page:    req.Page,
		Size:    req.Size,
		HasMore: int64((req.Page-1)*req.Size+len(data)) < total,
	}

	return result, nil
}

// Prepare 规范化、鉴权并注入权限过滤条件
func (e *Executor) Prepare(ctx context.Context, req *QueryRequest, userID string) error {
	if err := e.normalize(req); err != nil {
		return err
	}

	scope, err := e.validator.newAccessScope(userID)
	if err != nil {
		return fmt.Errorf("权限验证失败: %w", err)
	}

	if err := e.validator.validateRequestWithScope(ctx, req, userID, scope); err != nil {
		return fmt.Errorf("权限验证失败: %w", err)
	}

	if err := e.validator.autoFilterByPermissionWithScope(req, scope); err != nil {
		return err
	}

	e.expandWildcardSelections(req)

	return nil
}

// ExecuteRaw 执行原始 JSON 查询
func (e *Executor) ExecuteRaw(ctx context.Context, jsonData []byte, userID string) (*QueryResult, error) {
	// 1. 解析请求
	req, err := e.parser.Parse(jsonData)
	if err != nil {
		return nil, err
	}

	// 2. 执行查询
	return e.Execute(ctx, req, userID)
}

// ExecuteFromMap 从 map 执行查询
func (e *Executor) ExecuteFromMap(ctx context.Context, data map[string]interface{}, userID string) (*QueryResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}
	return e.ExecuteRaw(ctx, jsonData, userID)
}

// ExecuteBatch 执行批量查询
func (e *Executor) ExecuteBatch(ctx context.Context, req *BatchQueryRequest, userID string) (*BatchQueryResult, error) {
	results := make(map[string]*QueryResult)

	for name, query := range req.Queries {
		result, err := e.Execute(ctx, &query, userID)
		if err != nil {
			return nil, fmt.Errorf("查询 '%s' 执行失败: %w", name, err)
		}
		results[name] = result
	}

	return &BatchQueryResult{Results: results}, nil
}

// ExecuteBatchRaw 执行原始 JSON 批量查询
func (e *Executor) ExecuteBatchRaw(ctx context.Context, jsonData []byte, userID string) (*BatchQueryResult, error) {
	req, err := e.parser.ParseBatch(jsonData)
	if err != nil {
		return nil, err
	}
	return e.ExecuteBatch(ctx, req, userID)
}

// executeQuery 执行查询并返回结果
func (e *Executor) executeQuery(ctx context.Context, query *SQLQuery) ([]map[string]interface{}, error) {
	rows, err := e.db.WithContext(ctx).Raw(query.SQL, query.Params...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return e.scanRows(rows)
}

// executeCount 执行 COUNT 查询
func (e *Executor) executeCount(ctx context.Context, query *SQLQuery) (int64, error) {
	var total int64
	err := e.db.WithContext(ctx).Raw(query.SQL, query.Params...).Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

// scanRows 扫描查询结果
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
		// 扫描行
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// 构建结果映射
		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			val := values[i]

			// 处理字节数组（如 JSONB）
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
	if err := stdjson.Unmarshal(b, &jsonVal); err == nil {
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

// Validate 验证查询请求（不执行）
func (e *Executor) Validate(ctx context.Context, req *QueryRequest, userID string) error {
	req = cloneQueryRequest(req)
	return e.Prepare(ctx, req, userID)
}

// Explain 解释查询（返回生成的 SQL）
func (e *Executor) Explain(req *QueryRequest) (*SQLQuery, error) {
	return e.generator.Generate(req)
}

// ExplainAuthorized 在权限过滤后的上下文中生成 SQL
func (e *Executor) ExplainAuthorized(ctx context.Context, req *QueryRequest, userID string) (*SQLQuery, error) {
	req = cloneQueryRequest(req)
	if err := e.Prepare(ctx, req, userID); err != nil {
		return nil, err
	}
	return e.generator.Generate(req)
}

// GetParser 获取解析器
func (e *Executor) GetParser() *Parser {
	return e.parser
}

// GetValidator 获取验证器
func (e *Executor) GetValidator() *Validator {
	return e.validator
}

// GetGenerator 获取 SQL 生成器
func (e *Executor) GetGenerator() *SQLGenerator {
	return e.generator
}

// DB 返回数据库连接
func (e *Executor) DB() *gorm.DB {
	return e.db
}

// WithDB 创建使用指定数据库的执行器副本
func (e *Executor) WithDB(database *gorm.DB) *Executor {
	return &Executor{
		db:        database,
		parser:    e.parser,
		validator: e.validator,
		generator: e.generator,
		limits:    e.limits,
	}
}

// SimplifiedQuery 简化查询接口
// 适用于简单的单表查询场景
func (e *Executor) SimplifiedQuery(ctx context.Context, table string, filter map[string]interface{}, sort string, page, size int, userID string) (*QueryResult, error) {
	req := &QueryRequest{
		Table:  table,
		Filter: filter,
		Sort:   sort,
		Page:   page,
		Size:   size,
	}

	// 规范化
	if err := e.normalize(req); err != nil {
		return nil, err
	}

	return e.Execute(ctx, req, userID)
}

// normalize 规范化请求（内部使用，跳过解析器的验证）
func (e *Executor) normalize(req *QueryRequest) error {
	// 处理简化语法
	if req.Table != "" {
		req.From = req.Table
	}

	// 设置默认值
	if req.From == "" {
		return fmt.Errorf("必须指定表名")
	}

	if req.Page <= 0 {
		req.Page = 1
	}

	if req.Size <= 0 {
		req.Size = 20
	}

	// 转换简化语法的 filter 到 Where
	if len(req.Filter) > 0 && req.Where == nil {
		where, err := e.parser.parseSimplifiedFilter(req.Filter)
		if err != nil {
			return err
		}
		req.Where = where
	}

	// 转换简化语法的 sort 到 OrderBy
	if req.Sort != "" && len(req.OrderBy) == 0 {
		orderBy, err := e.parser.parseSimplifiedSort(req.Sort)
		if err != nil {
			return err
		}
		req.OrderBy = orderBy
	}

	// 如果没有指定 select，默认查询所有字段
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

// GlobalExecutor 全局执行器实例（单例模式）
var GlobalExecutor *Executor

// InitGlobalExecutor 初始化全局执行器
func InitGlobalExecutor(database *gorm.DB) {
	GlobalExecutor = NewExecutor(database)
}

// GetGlobalExecutor 获取全局执行器
func GetGlobalExecutor() *Executor {
	if GlobalExecutor == nil {
		// 尝试从全局 DB 初始化
		if database := db.DB(); database != nil {
			InitGlobalExecutor(database)
		}
	}
	return GlobalExecutor
}
