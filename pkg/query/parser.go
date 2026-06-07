package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parser parses query requests.
type Parser struct {
	limits QueryLimits
}

// NewParser creates a parser.
func NewParser() *Parser {
	return &Parser{
		limits: DefaultLimits,
	}
}

// NewParserWithLimits creates a parser with custom limits.
func NewParserWithLimits(limits QueryLimits) *Parser {
	return &Parser{
		limits: limits,
	}
}

// Parse parses a query request.
func (p *Parser) Parse(data []byte) (*QueryRequest, error) {
	var req QueryRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	// Normalize request
	if err := p.normalize(&req); err != nil {
		return nil, err
	}

	// Validate request
	if err := p.validate(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// ParseFromMap parses a query request from a map.
func (p *Parser) ParseFromMap(data map[string]interface{}) (*QueryRequest, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("serialization failed: %w", err)
	}
	return p.Parse(jsonData)
}

// normalize converts simplified syntax to full syntax.
func (p *Parser) normalize(req *QueryRequest) error {
	// Handle simplified syntax
	if req.Table != "" {
		req.From = req.Table
	}

	// Set defaults
	if req.From == "" {
		return errors.New("table name is required (from or table)")
	}

	if req.Page <= 0 {
		req.Page = 1
	}

	if req.Size <= 0 {
		req.Size = 20
	}

	// Convert simplified filter to Where
	if len(req.Filter) > 0 && req.Where == nil {
		where, err := p.parseSimplifiedFilter(req.Filter)
		if err != nil {
			return err
		}
		req.Where = where
	}

	// Convert simplified sort to OrderBy
	if req.Sort != "" && len(req.OrderBy) == 0 {
		orderBy, err := p.parseSimplifiedSort(req.Sort)
		if err != nil {
			return err
		}
		req.OrderBy = orderBy
	}

	// Default to all fields if select is not specified
	if len(req.Select) == 0 {
		req.Select = []string{"*"}
	}

	// Normalize sort direction
	for i := range req.OrderBy {
		req.OrderBy[i].Dir = strings.ToLower(req.OrderBy[i].Dir)
		if req.OrderBy[i].Dir == "" {
			req.OrderBy[i].Dir = "asc"
		}
	}

	return nil
}

// parseSimplifiedFilter parses simplified filter syntax.
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

// parseFilterField parses a single filter field.
func (p *Parser) parseFilterField(field string, value interface{}) (Condition, error) {
	// Check if value is an operator object
	if obj, ok := value.(map[string]interface{}); ok {
		// Support {"field": {"op": "value"}} or {"field": {"in": ["a", "b"]}}
		for op, val := range obj {
			switch op {
			case "eq", "ne", "gt", "gte", "lt", "lte", "like", "in", "between", "is_null":
				return Condition{
					Field: field,
					Op:    op,
					Value: val,
				}, nil
			default:
				// Possibly shorthand {"status": {"in": ["a", "b"]}}
				if isOperator(op) {
					return Condition{
						Field: field,
						Op:    op,
						Value: val,
					}, nil
				}
			}
		}

		return Condition{}, fmt.Errorf("field '%s' contains invalid operator", field)
	}

	// Default to eq operator
	return Condition{
		Field: field,
		Op:    "eq",
		Value: value,
	}, nil
}

// parseSimplifiedSort parses simplified sort syntax.
func (p *Parser) parseSimplifiedSort(sort string) ([]OrderByClause, error) {
	parts := strings.Split(sort, ",")
	orderBy := make([]OrderByClause, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for - prefix indicating descending order
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

// validate validates a query request.
func (p *Parser) validate(req *QueryRequest) error {
	// Validate table name
	if req.From == "" {
		return errors.New("table name cannot be empty")
	}

	// Validate pagination params
	if req.Size > p.limits.MaxPageSize {
		return fmt.Errorf("page size cannot exceed %d", p.limits.MaxPageSize)
	}

	// Validate JOIN count
	if len(req.Join) > p.limits.MaxJoins {
		return fmt.Errorf("JOIN count cannot exceed %d", p.limits.MaxJoins)
	}

	// Validate field count
	if len(req.Select) > p.limits.MaxFields {
		return fmt.Errorf("field count cannot exceed %d", p.limits.MaxFields)
	}

	// Validate WHERE nesting depth
	if req.Where != nil {
		if err := p.validateWhereDepth(req.Where, 0); err != nil {
			return err
		}
	}

	// Validate HAVING nesting depth
	if req.Having != nil {
		if err := p.validateWhereDepth(req.Having, 0); err != nil {
			return err
		}
	}

	// Validate UNION queries
	for i, unionReq := range req.Union {
		if err := p.validate(&unionReq); err != nil {
			return fmt.Errorf("union[%d]: %w", i, err)
		}
	}
	for i, intersectReq := range req.Intersect {
		if err := p.validate(&intersectReq); err != nil {
			return fmt.Errorf("intersect[%d]: %w", i, err)
		}
	}

	// Validate aggregate functions
	for _, agg := range req.Aggregate {
		if !isValidAggregateFunc(agg.Func) {
			return fmt.Errorf("invalid aggregate function: %s", agg.Func)
		}
		if agg.As == "" {
			return fmt.Errorf("aggregate function %s must specify an alias (as)", agg.Func)
		}
	}

	// Validate JOIN type and ON condition structure
	for i, join := range req.Join {
		if !isValidJoinType(join.Type) {
			return fmt.Errorf("invalid JOIN type: %s", join.Type)
		}
		if join.Table == "" {
			return errors.New("JOIN must specify table")
		}
		if err := ValidateIdentifier(join.Table); err != nil {
			return fmt.Errorf("JOIN[%d].table %w", i, err)
		}
		if join.As != "" {
			if err := ValidateIdentifier(join.As); err != nil {
				return fmt.Errorf("JOIN[%d].as %w", i, err)
			}
		}
		if join.On.IsZero() {
			return errors.New("invalid_join_condition: JOIN must specify on{left, op, right}")
		}
		if err := ValidateJoinOp(join.On.Op); err != nil {
			return fmt.Errorf("JOIN[%d].on.op %w", i, err)
		}
		if err := ValidateIdentifier(join.On.Left); err != nil {
			return fmt.Errorf("JOIN[%d].on.left %w", i, err)
		}
		if err := ValidateIdentifier(join.On.Right); err != nil {
			return fmt.Errorf("JOIN[%d].on.right %w", i, err)
		}
	}

	// Validate all user-provided field names (Select / OrderBy / GroupBy / Aggregate / Where).
	// Only syntactic validation is done here; permissions / whitelist are handled by Validator.
	for i, field := range req.Select {
		if field == "*" {
			continue
		}
		if err := validateFieldExpression(field); err != nil {
			return fmt.Errorf("select[%d] %w", i, err)
		}
	}
	for i, order := range req.OrderBy {
		if err := validateFieldExpression(order.Field); err != nil {
			return fmt.Errorf("orderBy[%d] %w", i, err)
		}
	}
	for i, group := range req.GroupBy {
		if err := validateFieldExpression(group); err != nil {
			return fmt.Errorf("groupBy[%d] %w", i, err)
		}
	}
	for i, agg := range req.Aggregate {
		if agg.Field == "" || agg.Field == "*" {
			continue
		}
		if err := validateFieldExpression(agg.Field); err != nil {
			return fmt.Errorf("aggregate[%d].field %w", i, err)
		}
		if err := ValidateIdentifier(agg.As); err != nil {
			return fmt.Errorf("aggregate[%d].as %w", i, err)
		}
	}
	for _, join := range req.Join {
		for i, field := range join.Select {
			if field == "*" {
				continue
			}
			if err := validateFieldExpression(field); err != nil {
				return fmt.Errorf("join[%s].select[%d] %w", join.Table, i, err)
			}
		}
	}
	if req.From != "" {
		if err := ValidateIdentifier(req.From); err != nil {
			return fmt.Errorf("from %w", err)
		}
	}
	if req.Where != nil {
		if err := validateWhereFieldNames(req.Where); err != nil {
			return err
		}
	}
	if req.Having != nil {
		if err := validateWhereFieldNames(req.Having); err != nil {
			return err
		}
	}

	return nil
}

// validateFieldExpression validates field names like `id`, `tables.id`, `data.status`, or `data->>name`.
// Rejects nested `->`, and names containing `[`, `*`, `'`, `"`, or spaces.
func validateFieldExpression(field string) error {
	field = strings.TrimSpace(field)
	if field == "" {
		return errors.New("field name cannot be empty")
	}
	// Postgres JSON arrow syntax `data->>key` or `data->key`: split and validate
	if strings.Contains(field, "->") {
		// Disallow nested `->`, e.g. `data->>a->>b`; delegate to sql_generator JSON path expression instead
		parts := strings.SplitN(field, "->", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid field name %q: JSON reference format error", field)
		}
		base := strings.TrimSpace(parts[0])
		path := strings.TrimSpace(parts[1])
		path = strings.TrimPrefix(path, ">") // Allow both `->>` and `->`
		// Strip quotes: `data->>'key'` is valid SQL, remove quotes before validating segments
		path = strings.Trim(path, "'\"")
		if err := ValidateIdentifier(base); err != nil {
			return err
		}
		if err := ValidateJSONPath(path); err != nil {
			return err
		}
		return nil
	}
	return ValidateIdentifier(field)
}

// validateWhereFieldNames recursively validates field names in WhereClause.
func validateWhereFieldNames(where *WhereClause) error {
	for _, cond := range where.And {
		if err := validateConditionFieldName(cond); err != nil {
			return err
		}
	}
	for _, cond := range where.Or {
		if err := validateConditionFieldName(cond); err != nil {
			return err
		}
	}
	return nil
}

func validateConditionFieldName(cond Condition) error {
	// Distinguish leaf conditions from group conditions: group conditions (only And/Or)
	// may have an empty Field because they are purely for nesting; leaf nodes must have
	// a valid field, otherwise SQL generation will fail with an empty field and could
	// become a validation bypass vector.
	isGroup := len(cond.And) > 0 || len(cond.Or) > 0
	if !isGroup {
		if err := validateFieldExpression(cond.Field); err != nil {
			return fmt.Errorf("where %w", err)
		}
	} else if cond.Field != "" {
		if err := validateFieldExpression(cond.Field); err != nil {
			return fmt.Errorf("where %w", err)
		}
	}
	for _, nested := range cond.And {
		if err := validateConditionFieldName(nested); err != nil {
			return err
		}
	}
	for _, nested := range cond.Or {
		if err := validateConditionFieldName(nested); err != nil {
			return err
		}
	}
	return nil
}

// validateWhereDepth validates WHERE condition nesting depth.
func (p *Parser) validateWhereDepth(where *WhereClause, depth int) error {
	if depth > p.limits.MaxDepth {
		return fmt.Errorf("WHERE nesting depth cannot exceed %d", p.limits.MaxDepth)
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

// validateConditionDepth validates condition nesting depth.
func (p *Parser) validateConditionDepth(cond Condition, depth int) error {
	if depth > p.limits.MaxDepth {
		return fmt.Errorf("WHERE nesting depth cannot exceed %d", p.limits.MaxDepth)
	}

	// Validate operator
	if cond.Op != "" && !isValidOperator(cond.Op) {
		return fmt.Errorf("invalid operator: %s", cond.Op)
	}

	// Recursively validate nested conditions
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

// isOperator checks whether a string is an operator.
func isOperator(s string) bool {
	return isValidOperator(s)
}

// isValidOperator checks whether an operator is valid.
func isValidOperator(op string) bool {
	validOps := []string{"eq", "ne", "gt", "gte", "lt", "lte", "like", "in", "between", "is_null"}
	for _, valid := range validOps {
		if op == valid {
			return true
		}
	}
	return false
}

// isValidAggregateFunc checks whether an aggregate function is valid.
func isValidAggregateFunc(fn string) bool {
	validFuncs := []string{"count", "sum", "avg", "min", "max", "count_distinct", "stddev", "stddev_pop", "stddev_samp", "variance", "var_pop", "var_samp"}
	for _, valid := range validFuncs {
		if fn == valid {
			return true
		}
	}
	return false
}

// isValidJoinType checks whether a JOIN type is valid.
func isValidJoinType(joinType string) bool {
	validTypes := []string{"left", "right", "inner", "outer"}
	for _, valid := range validTypes {
		if joinType == valid {
			return true
		}
	}
	return false
}

// ParseBatch parses a batch query request.
func (p *Parser) ParseBatch(data []byte) (*BatchQueryRequest, error) {
	var req BatchQueryRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	// Validate each query
	for name, query := range req.Queries {
		if err := p.normalize(&query); err != nil {
			return nil, fmt.Errorf("query '%s' format error: %w", name, err)
		}
		if err := p.validate(&query); err != nil {
			return nil, fmt.Errorf("query '%s' validation failed: %w", name, err)
		}
		req.Queries[name] = query
	}

	return &req, nil
}

// ConvertValue converts a value to an appropriate type.
func ConvertValue(value interface{}) interface{} {
	switch v := value.(type) {
	case float64:
		// Check if integer
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
		// Try to parse as number
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
