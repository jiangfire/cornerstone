package query

// QueryRequest 查询请求 - 支持完整语法和简化语法
type QueryRequest struct {
	// 完整语法
	From      string          `json:"from"`      // 主表
	Select    []string        `json:"select"`    // 查询字段
	Where     *WhereClause    `json:"where"`     // 条件
	Join      []JoinClause    `json:"join"`      // JOIN
	GroupBy   []string        `json:"groupBy"`   // 分组
	Aggregate []AggregateFunc `json:"aggregate"` // 聚合
	OrderBy   []OrderByClause `json:"orderBy"`   // 排序
	Page      int             `json:"page"`      // 页码
	Size      int             `json:"size"`      // 每页大小

	// 简化语法
	Table  string                 `json:"table"`  // 主表（简写）
	Filter map[string]interface{} `json:"filter"` // 过滤条件（简写）
	Sort   string                 `json:"sort"`   // 排序（简写，如 "-created_at"）
}

// WhereClause WHERE 条件
type WhereClause struct {
	And []Condition            `json:"and,omitempty"`
	Or  []Condition            `json:"or,omitempty"`
	Raw map[string]interface{} `json:"-"` // 简化语法的原始过滤条件
}

// Condition 单个条件
type Condition struct {
	Field string      `json:"field"`           // 字段名
	Op    string      `json:"op,omitempty"`    // 操作符，省略时为 eq
	Value interface{} `json:"value"`           // 值
	Not   bool        `json:"not,omitempty"`   // 是否取反
	And   []Condition `json:"and,omitempty"`   // 嵌套 AND
	Or    []Condition `json:"or,omitempty"`    // 嵌套 OR
}

// JoinClause JOIN 子句
type JoinClause struct {
	Type   string   `json:"type"`              // join 类型: left, right, inner
	Table  string   `json:"table"`             // 关联表
	As     string   `json:"as,omitempty"`      // 别名
	On     string   `json:"on"`                // 关联条件
	Select []string `json:"select,omitempty"`  // 关联表查询字段
}

// AggregateFunc 聚合函数
type AggregateFunc struct {
	Func  string `json:"func"`               // 函数名: count, sum, avg, min, max
	Field string `json:"field,omitempty"`    // 字段名
	As    string `json:"as"`                 // 别名
}

// OrderByClause 排序子句
type OrderByClause struct {
	Field string `json:"field"`              // 字段名
	Dir   string `json:"dir,omitempty"`      // 方向: asc, desc (默认 asc)
}

// QueryResult 查询结果
type QueryResult struct {
	Data    []map[string]interface{} `json:"data"`     // 数据
	Total   int64                    `json:"total"`    // 总数
	Page    int                      `json:"page"`     // 当前页
	Size    int                      `json:"size"`     // 每页大小
	HasMore bool                     `json:"has_more"` // 是否还有更多
}

// BatchQueryRequest 批量查询请求
type BatchQueryRequest struct {
	Queries map[string]QueryRequest `json:"queries"` // key: 查询名称
}

// BatchQueryResult 批量查询结果
type BatchQueryResult struct {
	Results map[string]*QueryResult `json:"results"` // key: 查询名称
}

// QueryLimits 查询限制配置
type QueryLimits struct {
	MaxJoins    int    // 最多 JOIN 表数
	MaxPageSize int    // 最大分页大小
	MaxDepth    int    // 嵌套查询最大深度
	MaxRows     int64  // 最大返回行数（不带分页时）
	MaxFields   int    // 最大查询字段数
}

// DefaultLimits 默认查询限制
var DefaultLimits = QueryLimits{
	MaxJoins:    3,
	MaxPageSize: 1000,
	MaxDepth:    5,
	MaxRows:     10000,
	MaxFields:   100,
}

// AllowedTables 允许的表和字段白名单
type AllowedTables map[string][]string

// DefaultAllowedTables 默认允许的表
var DefaultAllowedTables = AllowedTables{
	"records":          {"id", "table_id", "data", "created_by", "updated_by", "version", "created_at", "updated_at"},
	"users":            {"id", "username", "email", "phone", "bio", "avatar", "created_at", "updated_at"},
	"tables":           {"id", "database_id", "name", "description", "created_at", "updated_at"},
	"databases":        {"id", "name", "description", "owner_id", "is_public", "is_personal", "created_at", "updated_at"},
	"fields":           {"id", "table_id", "name", "type", "required", "options", "created_at", "updated_at"},
	"database_access":  {"id", "user_id", "database_id", "role", "created_at", "updated_at"},
	"field_permissions": {"id", "table_id", "field_id", "role", "can_read", "can_write", "can_delete", "created_at", "updated_at"},
	"organizations":    {"id", "name", "description", "owner_id", "created_at", "updated_at"},
	"organization_members": {"id", "organization_id", "user_id", "role", "joined_at"},
	"activity_logs":    {"id", "user_id", "action", "resource_type", "resource_id", "description", "created_at"},
	"files":            {"id", "record_id", "file_name", "file_size", "file_type", "uploaded_by", "created_at"},
	"plugins":          {"id", "name", "description", "language", "entry_file", "config", "created_by", "created_at"},
	"plugin_bindings":  {"id", "plugin_id", "table_id", "trigger", "created_at"},
	"plugin_executions": {"id", "plugin_id", "table_id", "record_id", "trigger", "status", "output", "error", "duration_ms", "started_at"},
}

// IsTableAllowed 检查表是否允许访问
func (at AllowedTables) IsTableAllowed(table string) bool {
	_, ok := at[table]
	return ok
}

// IsFieldAllowed 检查字段是否允许访问
func (at AllowedTables) IsFieldAllowed(table, field string) bool {
	fields, ok := at[table]
	if !ok {
		return false
	}
	for _, f := range fields {
		if f == field || f == "*" {
			return true
		}
	}
	return false
}

// GetAllowedFields 获取表允许的字段列表
func (at AllowedTables) GetAllowedFields(table string) []string {
	return at[table]
}
