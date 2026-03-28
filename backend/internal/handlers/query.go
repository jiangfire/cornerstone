package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
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
func (h *QueryHandler) Query(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req query.QueryRequest

	// 尝试从 POST body 解析
	if c.Request.Method == "POST" && c.Request.Body != nil {
		body, err := io.ReadAll(c.Request.Body)
		if err == nil && len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				types.BadRequest(c, "请求格式错误: "+err.Error())
				return
			}
		}
	}

	// 如果 body 为空，尝试从 URL 参数解析
	if req.From == "" && req.Table == "" {
		q := c.Query("q")
		if q == "" {
			types.BadRequest(c, "缺少查询参数")
			return
		}

		if err := json.Unmarshal([]byte(q), &req); err != nil {
			types.BadRequest(c, "查询格式错误: "+err.Error())
			return
		}
	}

	// 执行查询
	result, err := h.executor.Execute(c.Request.Context(), &req, userID)
	if err != nil {
		// 区分权限错误和其他错误
		if isPermissionError(err) {
			types.Forbidden(c, err.Error())
			return
		}
		types.BadRequest(c, err.Error())
		return
	}

	types.Success(c, result)
}

// QueryExplain 查询解释接口（返回生成的 SQL，用于调试）
// POST /api/query/explain
func (h *QueryHandler) QueryExplain(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req query.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			types.BadRequest(c, "请求格式错误: "+err.Error())
			return
		}
		// 尝试从 URL 参数解析
		q := c.Query("q")
		if q == "" {
			types.BadRequest(c, "缺少查询参数")
			return
		}
		if err := json.Unmarshal([]byte(q), &req); err != nil {
			types.BadRequest(c, "查询格式错误: "+err.Error())
			return
		}
	}

	// 在权限过滤后的上下文中生成 SQL
	sqlQuery, err := h.executor.ExplainAuthorized(c.Request.Context(), &req, userID)
	if err != nil {
		if isPermissionError(err) {
			types.Forbidden(c, err.Error())
			return
		}
		types.BadRequest(c, err.Error())
		return
	}

	types.Success(c, gin.H{
		"sql":    sqlQuery.SQL,
		"params": sqlQuery.Params,
	})
}

// QueryValidate 查询验证接口（验证查询权限，不执行）
// POST /api/query/validate
func (h *QueryHandler) QueryValidate(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req query.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			types.BadRequest(c, "请求格式错误: "+err.Error())
			return
		}
		// 尝试从 URL 参数解析
		q := c.Query("q")
		if q == "" {
			types.BadRequest(c, "缺少查询参数")
			return
		}
		if err := json.Unmarshal([]byte(q), &req); err != nil {
			types.BadRequest(c, "查询格式错误: "+err.Error())
			return
		}
	}

	// 验证查询
	if err := h.executor.Validate(c.Request.Context(), &req, userID); err != nil {
		types.Forbidden(c, err.Error())
		return
	}

	types.SuccessWithMessage(c, "查询验证通过", nil)
}

// BatchQuery 批量查询接口
// POST /api/query/batch
func (h *QueryHandler) BatchQuery(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req query.BatchQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.BadRequest(c, "请求格式错误: "+err.Error())
		return
	}

	// 执行批量查询
	result, err := h.executor.ExecuteBatch(c.Request.Context(), &req, userID)
	if err != nil {
		if isPermissionError(err) {
			types.Forbidden(c, err.Error())
			return
		}
		types.BadRequest(c, err.Error())
		return
	}

	types.Success(c, result)
}

// ListTables 获取可访问的表列表
// GET /api/query/tables
func (h *QueryHandler) ListTables(c *gin.Context) {
	userID := middleware.GetUserID(c)

	tables, err := h.executor.GetValidator().GetAllowedTables(c.Request.Context(), userID)
	if err != nil {
		types.InternalServerError(c, "获取表列表失败: "+err.Error())
		return
	}

	types.Success(c, gin.H{
		"tables": tables,
	})
}

// GetTableSchema 获取表结构信息
// GET /api/query/schema/:table
func (h *QueryHandler) GetTableSchema(c *gin.Context) {
	userID := middleware.GetUserID(c)
	table := c.Param("table")

	validator := h.executor.GetValidator()

	// 检查表是否允许访问
	if err := validator.CheckTableAccess(c.Request.Context(), userID, table); err != nil {
		types.Forbidden(c, err.Error())
		return
	}

	fields := query.DefaultAllowedTables.GetAllowedFields(table)

	types.Success(c, gin.H{
		"table":  table,
		"fields": fields,
	})
}

// SimplifiedQuery 简化查询接口（URL 参数形式）
// GET /api/query/simple?table=records&filter={}&sort=-created_at&page=1&size=20
func (h *QueryHandler) SimplifiedQuery(c *gin.Context) {
	userID := middleware.GetUserID(c)

	table := c.Query("table")
	if table == "" {
		types.BadRequest(c, "必须指定表名")
		return
	}

	// 解析 filter
	var filter map[string]interface{}
	filterStr := c.Query("filter")
	if filterStr != "" {
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			types.BadRequest(c, "filter 格式错误: "+err.Error())
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
			types.Forbidden(c, err.Error())
			return
		}
		types.BadRequest(c, err.Error())
		return
	}

	types.Success(c, result)
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
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
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
