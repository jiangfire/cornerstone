package query

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// QueryRequest is a query request supporting both full and simplified syntax.
type QueryRequest struct {
	// Full syntax
	From      string          `json:"from"`      // Primary table
	Select    []string        `json:"select"`    // Selected fields
	Where     *WhereClause    `json:"where"`     // Conditions
	Having    *WhereClause    `json:"having"`    // HAVING conditions (post-aggregate filter)
	Join      []JoinClause    `json:"join"`      // JOINs
	GroupBy   []string        `json:"groupBy"`   // Grouping
	Aggregate []AggregateFunc `json:"aggregate"` // Aggregates
	OrderBy   []OrderByClause `json:"orderBy"`   // Ordering
	Page      int             `json:"page"`      // Page number
	Size      int             `json:"size"`      // Page size

	// Set operations
	Union     []QueryRequest `json:"union,omitempty"`     // UNION queries
	Intersect []QueryRequest `json:"intersect,omitempty"` // INTERSECT queries

	// Simplified syntax
	Table  string                 `json:"table"`  // Primary table (shorthand)
	Filter map[string]interface{} `json:"filter"` // Filter conditions (shorthand)
	Sort   string                 `json:"sort"`   // Sort (shorthand, e.g. "-created_at")
}

// WhereClause is a WHERE condition.
type WhereClause struct {
	And []Condition            `json:"and,omitempty"`
	Or  []Condition            `json:"or,omitempty"`
	Raw map[string]interface{} `json:"-"` // Raw filter from simplified syntax
}

// Condition is a single condition.
type Condition struct {
	Field string      `json:"field"`         // Field name
	Op    string      `json:"op,omitempty"`  // Operator; defaults to eq when omitted
	Value interface{} `json:"value"`         // Value
	Not   bool        `json:"not,omitempty"` // Negation
	And   []Condition `json:"and,omitempty"` // Nested AND
	Or    []Condition `json:"or,omitempty"`  // Nested OR
}

// JoinClause is a JOIN clause.
type JoinClause struct {
	Type   string        `json:"type"`             // Join type: left, right, inner
	Table  string        `json:"table"`            // Joined table
	As     string        `json:"as,omitempty"`     // Alias
	On     JoinCondition `json:"on"`               // Join condition (must be struct; string form deprecated)
	Select []string      `json:"select,omitempty"` // Fields to select from joined table
}

// JoinCondition describes an equi- or non-equi comparison for JOIN ... ON.
// In earlier versions On was a raw string concatenated directly into SQL, creating an injection
// surface. Now it is fixed as a struct requiring both sides to be valid qualified fields.
// All callers must migrate to this shape — old strings are rejected by UnmarshalJSON with
// `invalid_join_condition`. See docs/REVIEW-FIX-PLAN-2026-05.md P1-3.
type JoinCondition struct {
	Left  string `json:"left"`  // E.g. `tables.database_id`
	Op    string `json:"op"`    // Only "=" / "<>" allowed; see ValidateJoinOp
	Right string `json:"right"` // E.g. `db.id`
}

// IsZero reports whether JoinCondition is the zero value (used by parser for non-empty check).
func (jc JoinCondition) IsZero() bool {
	return jc.Left == "" && jc.Op == "" && jc.Right == ""
}

// UnmarshalJSON explicitly rejects the old string form and accepts the new object form.
// This gives callers a meaningful 400 error instead of the generic JSON type error from GORM/Gin.
func (jc *JoinCondition) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '"' {
		return errors.New("invalid_join_condition: 'on' must be a {left, op, right} object; string form is no longer supported")
	}
	// Use a one-time alias type to avoid infinite recursion.
	type rawJoinCondition JoinCondition
	var raw rawJoinCondition
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return fmt.Errorf("invalid_join_condition: %w", err)
	}
	*jc = JoinCondition(raw)
	return nil
}

// AggregateFunc is an aggregate function.
type AggregateFunc struct {
	Func  string `json:"func"`            // Function name: count, sum, avg, min, max
	Field string `json:"field,omitempty"` // Field name
	As    string `json:"as"`              // Alias
}

// OrderByClause is an ORDER BY clause.
type OrderByClause struct {
	Field string `json:"field"`         // Field name
	Dir   string `json:"dir,omitempty"` // Direction: asc, desc (default asc)
}

// QueryResult is a query result.
type QueryResult struct {
	Data    []map[string]interface{} `json:"data"`     // Result rows
	Total   int64                    `json:"total"`    // Total count
	Page    int                      `json:"page"`     // Current page
	Size    int                      `json:"size"`     // Page size
	HasMore bool                     `json:"has_more"` // Whether more pages exist
}

// BatchQueryRequest is a batch query request.
type BatchQueryRequest struct {
	Queries map[string]QueryRequest `json:"queries"` // key: query name
}

// BatchQueryResult is a batch query result.
type BatchQueryResult struct {
	Results map[string]*QueryResult `json:"results"` // key: query name
}

// QueryLimits defines query limit settings.
type QueryLimits struct {
	MaxJoins    int   // Max number of JOIN tables
	MaxPageSize int   // Max page size
	MaxDepth    int   // Max nesting depth for conditions
	MaxRows     int64 // Max rows returned (without pagination)
	MaxFields   int   // Max number of selected fields
}

// DefaultLimits are the default query limits.
var DefaultLimits = QueryLimits{
	MaxJoins:    3,
	MaxPageSize: 1000,
	MaxDepth:    5,
	MaxRows:     10000,
	MaxFields:   100,
}

// AllowedTables is a whitelist of allowed tables and fields.
type AllowedTables map[string][]string

// DefaultAllowedTables are the default allowed tables.
var DefaultAllowedTables = AllowedTables{
	"records":   {"id", "table_id", "data", "version", "created_at", "updated_at"},
	"tables":    {"id", "database_id", "name", "description", "created_at", "updated_at"},
	"databases": {"id", "name", "description", "created_at", "updated_at"},
	"fields":    {"id", "table_id", "name", "type", "required", "options", "description", "created_at", "updated_at"},
	"files":     {"id", "record_id", "field_id", "file_name", "file_size", "file_type", "storage_url", "created_at", "updated_at"},
	"tokens":    {"id", "name", "is_master", "scopes", "expires_at", "created_at"},
}

// IsTableAllowed checks whether a table is allowed.
func (at AllowedTables) IsTableAllowed(table string) bool {
	_, ok := at[table]
	return ok
}

// IsFieldAllowed checks whether a field is allowed.
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

// GetAllowedFields returns the allowed field list for a table.
func (at AllowedTables) GetAllowedFields(table string) []string {
	return at[table]
}
