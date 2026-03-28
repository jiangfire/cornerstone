package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parser 查询解析器
type Parser struct {
	limits QueryLimits
}

// NewParser 创建解析器
func NewParser() *Parser {
	return &Parser{
		limits: DefaultLimits,
	}
}

// NewParserWithLimits 创建带自定义限制的解析器
func NewParserWithLimits(limits QueryLimits) *Parser {
	return &Parser{
		limits: limits,
	}
}

// Parse 解析查询请求
func (p *Parser) Parse(data []byte) (*QueryRequest, error) {
	var req QueryRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	// 规范化请求
	if err := p.normalize(&req); err != nil {
		return nil, err
	}

	// 验证请求
	if err := p.validate(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// ParseFromMap 从 map 解析查询请求
func (p *Parser) ParseFromMap(data map[string]interface{}) (*QueryRequest, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}
	return p.Parse(jsonData)
}

// normalize 规范化请求 - 将简化语法转换为完整语法
func (p *Parser) normalize(req *QueryRequest) error {
	// 处理简化语法
	if req.Table != "" {
		req.From = req.Table
	}

	// 设置默认值
	if req.From == "" {
		return errors.New("必须指定表名 (from 或 table)")
	}

	if req.Page <= 0 {
		req.Page = 1
	}

	if req.Size <= 0 {
		req.Size = 20
	}

	// 转换简化语法的 filter 到 Where
	if len(req.Filter) > 0 && req.Where == nil {
		where, err := p.parseSimplifiedFilter(req.Filter)
		if err != nil {
			return err
		}
		req.Where = where
	}

	// 转换简化语法的 sort 到 OrderBy
	if req.Sort != "" && len(req.OrderBy) == 0 {
		orderBy, err := p.parseSimplifiedSort(req.Sort)
		if err != nil {
			return err
		}
		req.OrderBy = orderBy
	}

	// 如果没有指定 select，默认查询所有字段
	if len(req.Select) == 0 {
		req.Select = []string{"*"}
	}

	// 规范化排序方向
	for i := range req.OrderBy {
		req.OrderBy[i].Dir = strings.ToLower(req.OrderBy[i].Dir)
		if req.OrderBy[i].Dir == "" {
			req.OrderBy[i].Dir = "asc"
		}
	}

	return nil
}

// parseSimplifiedFilter 解析简化语法的 filter
func (p *Parser) parseSimplifiedFilter(filter map[string]interface{}) (*WhereClause, error) {
	where := &WhereClause{
		And: make([]Condition, 0, len(filter)),
	}

	for field, value := range filter {
		cond, err := p.parseFilterField(field, value)
		if err != nil {
			return nil, err
		}
		where.And = append(where.And, cond)
	}

	return where, nil
}

// parseFilterField 解析单个过滤字段
func (p *Parser) parseFilterField(field string, value interface{}) (Condition, error) {
	// 检查是否是操作符对象
	if obj, ok := value.(map[string]interface{}); ok {
		// 支持 {"field": {"op": "value"}} 或 {"field": {"in": ["a", "b"]}}
		for op, val := range obj {
			switch op {
			case "eq", "ne", "gt", "gte", "lt", "lte", "like", "in", "between", "is_null":
				return Condition{
					Field: field,
					Op:    op,
					Value: val,
				}, nil
			default:
				// 可能是简写 {"status": {"in": ["a", "b"]}}
				if isOperator(op) {
					return Condition{
						Field: field,
						Op:    op,
						Value: val,
					}, nil
				}
			}
		}

		return Condition{}, fmt.Errorf("字段 '%s' 包含无效操作符", field)
	}

	// 默认值使用 eq 操作符
	return Condition{
		Field: field,
		Op:    "eq",
		Value: value,
	}, nil
}

// parseSimplifiedSort 解析简化语法的 sort
func (p *Parser) parseSimplifiedSort(sort string) ([]OrderByClause, error) {
	parts := strings.Split(sort, ",")
	orderBy := make([]OrderByClause, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 检查前缀 - 表示降序
		if strings.HasPrefix(part, "-") {
			orderBy = append(orderBy, OrderByClause{
				Field: strings.TrimPrefix(part, "-"),
				Dir:   "desc",
			})
		} else if strings.HasPrefix(part, "+") {
			orderBy = append(orderBy, OrderByClause{
				Field: strings.TrimPrefix(part, "+"),
				Dir:   "asc",
			})
		} else {
			orderBy = append(orderBy, OrderByClause{
				Field: part,
				Dir:   "asc",
			})
		}
	}

	return orderBy, nil
}

// validate 验证查询请求
func (p *Parser) validate(req *QueryRequest) error {
	// 验证表名
	if req.From == "" {
		return errors.New("表名不能为空")
	}

	// 验证分页参数
	if req.Size > p.limits.MaxPageSize {
		return fmt.Errorf("每页大小不能超过 %d", p.limits.MaxPageSize)
	}

	// 验证 JOIN 数量
	if len(req.Join) > p.limits.MaxJoins {
		return fmt.Errorf("JOIN 表数不能超过 %d", p.limits.MaxJoins)
	}

	// 验证查询字段数
	if len(req.Select) > p.limits.MaxFields {
		return fmt.Errorf("查询字段数不能超过 %d", p.limits.MaxFields)
	}

	// 验证 WHERE 条件深度
	if req.Where != nil {
		if err := p.validateWhereDepth(req.Where, 0); err != nil {
			return err
		}
	}

	// 验证聚合函数
	for _, agg := range req.Aggregate {
		if !isValidAggregateFunc(agg.Func) {
			return fmt.Errorf("无效的聚合函数: %s", agg.Func)
		}
		if agg.As == "" {
			return fmt.Errorf("聚合函数 %s 必须指定别名 (as)", agg.Func)
		}
	}

	// 验证 JOIN 类型
	for _, join := range req.Join {
		if !isValidJoinType(join.Type) {
			return fmt.Errorf("无效的 JOIN 类型: %s", join.Type)
		}
		if join.Table == "" || join.On == "" {
			return errors.New("JOIN 必须指定 table 和 on")
		}
	}

	return nil
}

// validateWhereDepth 验证 WHERE 条件嵌套深度
func (p *Parser) validateWhereDepth(where *WhereClause, depth int) error {
	if depth > p.limits.MaxDepth {
		return fmt.Errorf("WHERE 条件嵌套深度不能超过 %d", p.limits.MaxDepth)
	}

	for _, cond := range where.And {
		if err := p.validateConditionDepth(cond, depth+1); err != nil {
			return err
		}
	}

	for _, cond := range where.Or {
		if err := p.validateConditionDepth(cond, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// validateConditionDepth 验证条件嵌套深度
func (p *Parser) validateConditionDepth(cond Condition, depth int) error {
	if depth > p.limits.MaxDepth {
		return fmt.Errorf("WHERE 条件嵌套深度不能超过 %d", p.limits.MaxDepth)
	}

	// 验证操作符
	if cond.Op != "" && !isValidOperator(cond.Op) {
		return fmt.Errorf("无效的操作符: %s", cond.Op)
	}

	// 递归验证嵌套条件
	for _, nested := range cond.And {
		if err := p.validateConditionDepth(nested, depth+1); err != nil {
			return err
		}
	}

	for _, nested := range cond.Or {
		if err := p.validateConditionDepth(nested, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// isOperator 检查字符串是否是操作符
func isOperator(s string) bool {
	return isValidOperator(s)
}

// isValidOperator 验证操作符是否有效
func isValidOperator(op string) bool {
	validOps := []string{"eq", "ne", "gt", "gte", "lt", "lte", "like", "in", "between", "is_null"}
	for _, valid := range validOps {
		if op == valid {
			return true
		}
	}
	return false
}

// isValidAggregateFunc 验证聚合函数是否有效
func isValidAggregateFunc(fn string) bool {
	validFuncs := []string{"count", "sum", "avg", "min", "max"}
	for _, valid := range validFuncs {
		if fn == valid {
			return true
		}
	}
	return false
}

// isValidJoinType 验证 JOIN 类型是否有效
func isValidJoinType(joinType string) bool {
	validTypes := []string{"left", "right", "inner", "outer"}
	for _, valid := range validTypes {
		if joinType == valid {
			return true
		}
	}
	return false
}

// ParseBatch 解析批量查询请求
func (p *Parser) ParseBatch(data []byte) (*BatchQueryRequest, error) {
	var req BatchQueryRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	// 验证每个查询
	for name, query := range req.Queries {
		if err := p.normalize(&query); err != nil {
			return nil, fmt.Errorf("查询 '%s' 格式错误: %w", name, err)
		}
		if err := p.validate(&query); err != nil {
			return nil, fmt.Errorf("查询 '%s' 验证失败: %w", name, err)
		}
		req.Queries[name] = query
	}

	return &req, nil
}

// ConvertValue 转换值为适当类型
func ConvertValue(value interface{}) interface{} {
	switch v := value.(type) {
	case float64:
		// 检查是否为整数
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case float32:
		if v == float32(int32(v)) {
			return int32(v)
		}
		return v
	case string:
		// 尝试解析为数字
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		return v
	default:
		return v
	}
}
