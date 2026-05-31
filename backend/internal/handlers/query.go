package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
	"github.com/jiangfire/cornerstone/backend/pkg/query"
)

// QueryHandler 查询处理器
type QueryHandler struct {
	executor *query.Executor
}

// NewQueryHandler 创建查询处理器
func NewQueryHandler() *QueryHandler {
	return &QueryHandler{
		executor: query.NewExecutor(db.DB()),
	}
}

// Query 统一查询接口
// POST /api/query
// GET /api/query?q={json_string}
//
// @Summary      Execute a query
// @Description  Execute a full Query DSL request using POST body or GET query parameter.
//
//	For POST, send the Query DSL as JSON body.
//	For GET, pass the Query DSL as a URL-encoded JSON string in the "q" query parameter.
//
//	Supports: from, select, where, order_by, limit, offset, group_by, having,
//	aggregates, join, and union clauses.
//
// @Tags         query
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.QueryDSLRequest  true  "Query DSL body"
// @Success      200  {object}  swagger.APIResponse{data=swagger.QueryResult}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid query DSL"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to queried resource"
// @Router       /api/query [post]
// @Router       /api/query [get]
func (h *QueryHandler) Query(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req query.QueryRequest

	// 尝试从 POST body 解析
	if c.Request.Method == "POST" && c.Request.Body != nil {
		body, err := io.ReadAll(c.Request.Body)
		if err == nil && len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				dto.BadRequest(c, "请求格式错误: "+err.Error())
				return
			}
		}
	}

	// 如果 body 为空，尝试从 URL 参数解析
	if req.From == "" && req.Table == "" {
		q := c.Query("q")
		if q == "" {
			dto.BadRequest(c, "缺少查询参数")
			return
		}

		if err := json.Unmarshal([]byte(q), &req); err != nil {
			dto.BadRequest(c, "查询格式错误: "+err.Error())
			return
		}
	}

	// 执行查询
	result, err := h.executor.Execute(c.Request.Context(), &req, userID)
	if err != nil {
		// 区分权限错误和其他错误
		if isPermissionError(err) {
			dto.Forbidden(c, err.Error())
			return
		}
		dto.BadRequest(c, err.Error())
		return
	}

	dto.Success(c, result)
}

// QueryExplain 查询解释接口（返回生成的 SQL，用于调试）
// POST /api/query/explain
//
// @Summary      Explain a query
// @Description  Returns the generated SQL and parameters for a query without executing it.
//
//	Useful for debugging query construction and verifying correctness.
//	The query is validated against the authenticated token's permissions
//	before generating the SQL.
//
// @Tags         query
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.QueryDSLRequest  true  "Query DSL body"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid query DSL"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to queried resource"
// @Router       /api/query/explain [post]
func (h *QueryHandler) QueryExplain(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req query.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			dto.BadRequest(c, "请求格式错误: "+err.Error())
			return
		}
		// 尝试从 URL 参数解析
		q := c.Query("q")
		if q == "" {
			dto.BadRequest(c, "缺少查询参数")
			return
		}
		if err := json.Unmarshal([]byte(q), &req); err != nil {
			dto.BadRequest(c, "查询格式错误: "+err.Error())
			return
		}
	}

	// 在权限过滤后的上下文中生成 SQL
	sqlQuery, err := h.executor.ExplainAuthorized(c.Request.Context(), &req, userID)
	if err != nil {
		if isPermissionError(err) {
			dto.Forbidden(c, err.Error())
			return
		}
		dto.BadRequest(c, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"sql":    sqlQuery.SQL,
		"params": sqlQuery.Params,
	})
}

// QueryValidate 查询验证接口（验证查询权限，不执行）
// POST /api/query/validate
//
// @Summary      Validate a query
// @Description  Validate a query DSL and check access permissions without executing it.
//
//	Returns a success message if the query is valid and the token has
//	the required permissions. Returns a forbidden error if access is denied.
//
// @Tags         query
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.QueryDSLRequest  true  "Query DSL body"
// @Success      200  {object}  swagger.APIResponse
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid query DSL"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to queried resource"
// @Router       /api/query/validate [post]
func (h *QueryHandler) QueryValidate(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req query.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			dto.BadRequest(c, "请求格式错误: "+err.Error())
			return
		}
		// 尝试从 URL 参数解析
		q := c.Query("q")
		if q == "" {
			dto.BadRequest(c, "缺少查询参数")
			return
		}
		if err := json.Unmarshal([]byte(q), &req); err != nil {
			dto.BadRequest(c, "查询格式错误: "+err.Error())
			return
		}
	}

	// 验证查询
	if err := h.executor.Validate(c.Request.Context(), &req, userID); err != nil {
		dto.Forbidden(c, err.Error())
		return
	}

	dto.SuccessWithMessage(c, "查询验证通过", nil)
}

// BatchQuery 批量查询接口
// POST /api/query/batch
//
// @Summary      Execute a batch query
// @Description  Execute multiple queries in a single request.
//
//	Each query in the array is executed independently. Results are returned
//	as a map keyed by query index. All queries are subject to the
//	authenticated token's permissions.
//
// @Tags         query
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.BatchQueryRequest  true  "Batch query request"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid query DSL"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to queried resource"
// @Router       /api/query/batch [post]
func (h *QueryHandler) BatchQuery(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req query.BatchQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.BadRequest(c, "请求格式错误: "+err.Error())
		return
	}

	// 执行批量查询
	result, err := h.executor.ExecuteBatch(c.Request.Context(), &req, userID)
	if err != nil {
		if isPermissionError(err) {
			dto.Forbidden(c, err.Error())
			return
		}
		dto.BadRequest(c, err.Error())
		return
	}

	dto.Success(c, result)
}

// ListTables 获取可访问的表列表
// GET /api/query/tables
//
// @Summary      List queryable tables
// @Description  Returns all tables the authenticated token can query.
//
//	This includes system tables (records, tables, databases, fields, files, tokens)
//	that the token has been granted access to. Use this to discover available
//	tables before constructing queries.
//
// @Tags         query
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      500  {object}  swagger.ErrorResponse
// @Router       /api/query/tables [get]
func (h *QueryHandler) ListTables(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	tables, err := h.executor.GetValidator().GetAllowedTables(c.Request.Context(), userID)
	if err != nil {
		dto.InternalServerError(c, "获取表列表失败: "+err.Error())
		return
	}

	dto.Success(c, gin.H{
		"tables": tables,
	})
}

// GetTableSchema 获取表结构信息
// GET /api/query/schema/:table
//
// @Summary      Get table schema for query
// @Description  Returns the allowed fields for a queryable table.
//
//	Use this to discover which fields can be used in select, where, and
//	order_by clauses. The table name must be one of the allowed query targets
//	for the authenticated token.
//
// @Tags         query
// @Produce      json
// @Security     ApiKeyAuth
// @Param        table  path  string  true  "Table name (records, tables, databases, fields, files, tokens)"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this table"
// @Failure      404  {object}  swagger.ErrorResponse  "Table not found"
// @Router       /api/query/schema/{table} [get]
func (h *QueryHandler) GetTableSchema(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	table := c.Param("table")

	validator := h.executor.GetValidator()

	// 检查表是否允许访问
	if err := validator.CheckTableAccess(c.Request.Context(), userID, table); err != nil {
		dto.Forbidden(c, err.Error())
		return
	}

	fields := query.DefaultAllowedTables.GetAllowedFields(table)

	dto.Success(c, gin.H{
		"table":  table,
		"fields": fields,
	})
}

// SimplifiedQuery 简化查询接口（URL 参数形式）
// GET /api/query/simple?table=records&filter={}&sort=-created_at&page=1&size=20
//
// @Summary      Execute a simplified query
// @Description  Query records using simple URL parameters instead of the full Query DSL.
//
//	A lighter alternative to the full DSL. Supports basic filtering via JSON object,
//	sorting (prefix with "-" for descending), and pagination.
//	Maximum page size is 1000.
//
// @Tags         query
// @Produce      json
// @Security     ApiKeyAuth
// @Param        table   query  string  true   "Table name"
// @Param        filter  query  string  false  "JSON filter object"
// @Param        sort    query  string  false  "Sort expression (prefix with - for desc)"  default(-created_at)
// @Param        page    query  int     false  "Page number"  default(1)
// @Param        size    query  int     false  "Page size (max 1000)"  default(20)
// @Success      200  {object}  swagger.APIResponse{data=swagger.QueryResult}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - missing table or invalid parameters"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to queried table"
// @Router       /api/query/simple [get]
func (h *QueryHandler) SimplifiedQuery(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	table := c.Query("table")
	if table == "" {
		dto.BadRequest(c, "必须指定表名")
		return
	}

	// 解析 filter
	var filter map[string]interface{}
	filterStr := c.Query("filter")
	if filterStr != "" {
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			dto.BadRequest(c, "filter 格式错误: "+err.Error())
			return
		}
	}

	// 解析其他参数
	sort := c.DefaultQuery("sort", "-created_at")
	page := 1
	size := 20

	if p := c.Query("page"); p != "" {
		if v, err := parseInt(p); err == nil && v > 0 {
			page = v
		}
	}

	if s := c.Query("size"); s != "" {
		if v, err := parseInt(s); err == nil && v > 0 && v <= 1000 {
			size = v
		}
	}

	// 执行简化查询
	result, err := h.executor.SimplifiedQuery(c.Request.Context(), table, filter, sort, page, size, userID)
	if err != nil {
		if isPermissionError(err) {
			dto.Forbidden(c, err.Error())
			return
		}
		dto.BadRequest(c, err.Error())
		return
	}

	dto.Success(c, result)
}

// isPermissionError 判断是否为权限错误
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	permissionKeywords := []string{
		"权限", "无权", "拒绝", "denied", "forbidden",
		"不能访问", "不允许", "未授权", "unauthorized",
	}
	for _, keyword := range permissionKeywords {
		if containsString(msg, keyword) {
			return true
		}
	}
	return false
}

// containsString 检查字符串是否包含子串
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsSubstring(s, substr))
}

// containsSubstring 简单子串检查
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// parseInt 解析整数
func parseInt(s string) (int, error) {
	var n int
	if err := json.Unmarshal([]byte(s), &n); err != nil {
		// 尝试手动解析
		n = 0
		for _, ch := range s {
			if ch < '0' || ch > '9' {
				return 0, fmt.Errorf("invalid number: %s", s)
			}
			n = n*10 + int(ch-'0')
		}
	}
	return n, nil
}
