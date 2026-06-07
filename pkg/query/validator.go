package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jiangfire/cornerstone/internal/authz"
	"gorm.io/gorm"
)

type Validator struct {
	db            *gorm.DB
	allowedTables AllowedTables
}

type validatorAccessScope struct {
	authorizer *authz.Authorizer

	databaseIDs       []string
	databaseIDsLoaded bool

	tableIDs       []string
	tableIDsLoaded bool

	recordIDs       []string
	recordIDsLoaded bool
}

func NewValidator(db *gorm.DB) *Validator {
	return &Validator{
		db:            db,
		allowedTables: DefaultAllowedTables,
	}
}

func NewValidatorWithTables(db *gorm.DB, tables AllowedTables) *Validator {
	return &Validator{
		db:            db,
		allowedTables: tables,
	}
}

func (v *Validator) ValidateRequest(ctx context.Context, req *QueryRequest, userID string) error {
	scope, err := v.newAccessScope(userID)
	if err != nil {
		return err
	}
	return v.validateRequestWithScope(ctx, req, userID, scope)
}

func (v *Validator) validateRequestWithScope(ctx context.Context, req *QueryRequest, userID string, scope *validatorAccessScope) error {
	if req == nil {
		return errors.New("query request cannot be nil")
	}

	if err := v.checkTableAccessWithScope(ctx, req.From, scope); err != nil {
		return err
	}

	for _, field := range req.Select {
		if field == "*" {
			continue
		}
		if err := v.checkFieldReferenceWithScope(ctx, req.From, req.Join, field, scope); err != nil {
			return err
		}
	}

	for _, join := range req.Join {
		if err := v.checkTableAccessWithScope(ctx, join.Table, scope); err != nil {
			return err
		}
		for _, field := range join.Select {
			if field == "*" {
				continue
			}
			if err := v.checkFieldReferenceWithScope(ctx, join.Table, req.Join, field, scope); err != nil {
				return err
			}
		}
	}

	if req.Where != nil {
		if err := v.validateWhereFieldsWithScope(ctx, req.From, req.Join, req.Where, scope); err != nil {
			return err
		}
	}

	for _, order := range req.OrderBy {
		if err := v.checkFieldReferenceWithScope(ctx, req.From, req.Join, order.Field, scope); err != nil {
			return err
		}
	}

	for _, group := range req.GroupBy {
		if err := v.checkFieldReferenceWithScope(ctx, req.From, req.Join, group, scope); err != nil {
			return err
		}
	}

	for _, agg := range req.Aggregate {
		if agg.Field != "" && agg.Field != "*" {
			if err := v.checkFieldReferenceWithScope(ctx, req.From, req.Join, agg.Field, scope); err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *Validator) validateWhereFieldsWithScope(ctx context.Context, table string, joins []JoinClause, where *WhereClause, scope *validatorAccessScope) error {
	for _, cond := range where.And {
		if err := v.validateConditionFieldsWithScope(ctx, table, joins, cond, scope); err != nil {
			return err
		}
	}
	for _, cond := range where.Or {
		if err := v.validateConditionFieldsWithScope(ctx, table, joins, cond, scope); err != nil {
			return err
		}
	}
	return nil
}

func (v *Validator) validateConditionFieldsWithScope(ctx context.Context, table string, joins []JoinClause, cond Condition, scope *validatorAccessScope) error {
	if cond.Field != "" {
		if err := v.checkFieldReferenceWithScope(ctx, table, joins, cond.Field, scope); err != nil {
			return err
		}
	}

	for _, nested := range cond.And {
		if err := v.validateConditionFieldsWithScope(ctx, table, joins, nested, scope); err != nil {
			return err
		}
	}
	for _, nested := range cond.Or {
		if err := v.validateConditionFieldsWithScope(ctx, table, joins, nested, scope); err != nil {
			return err
		}
	}

	return nil
}

func (v *Validator) resolveReferenceTable(baseTable string, joins []JoinClause, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if ref == baseTable {
		return baseTable
	}
	for _, join := range joins {
		if join.As == ref {
			return join.Table
		}
		if join.Table == ref {
			return join.Table
		}
	}
	return ""
}

func (v *Validator) checkFieldReference(ctx context.Context, userID, baseTable string, joins []JoinClause, field string) error {
	scope, err := v.newAccessScope(userID)
	if err != nil {
		return err
	}
	return v.checkFieldReferenceWithScope(ctx, baseTable, joins, field, scope)
}

func (v *Validator) checkFieldReferenceWithScope(ctx context.Context, baseTable string, joins []JoinClause, field string, scope *validatorAccessScope) error {
	field = strings.TrimSpace(field)
	if field == "" || field == "*" {
		return nil
	}

	parts := strings.Split(field, ".")
	if len(parts) >= 2 {
		if refTable := v.resolveReferenceTable(baseTable, joins, parts[0]); refTable != "" {
			if err := v.checkTableAccessWithScope(ctx, refTable, scope); err != nil {
				return err
			}
			return v.CheckFieldAccess(ctx, "", refTable, strings.Join(parts[1:], "."))
		}

		if v.allowedTables.IsFieldAllowed(baseTable, parts[0]) {
			return v.CheckFieldAccess(ctx, "", baseTable, field)
		}
	}

	return v.CheckFieldAccess(ctx, "", baseTable, field)
}

func (v *Validator) CheckTableAccess(ctx context.Context, userID string, table string) error {
	scope, err := v.newAccessScope(userID)
	if err != nil {
		return err
	}
	return v.checkTableAccessWithScope(ctx, table, scope)
}

func (v *Validator) checkTableAccessWithScope(ctx context.Context, table string, scope *validatorAccessScope) error {
	if !v.allowedTables.IsTableAllowed(table) {
		return fmt.Errorf("table '%s' is not in the allowed list", table)
	}

	switch table {
	case "databases", "records", "tables", "fields", "files":
		return v.checkDataAccess(ctx, scope)
	case "tokens":
		authorizer := scope.authorizer
		if !authorizer.IsMaster() {
			return errors.New("access to tokens denied")
		}
		return nil
	default:
		return nil
	}
}

func (v *Validator) CheckFieldAccess(ctx context.Context, userID string, table, field string) error {
	if baseField, ok := splitJSONBaseField(field); ok && isJSONColumnCandidate(baseField) {
		field = baseField
	}

	if !v.allowedTables.IsFieldAllowed(table, field) {
		return fmt.Errorf("field '%s.%s' is not in the allowed list", table, field)
	}

	return nil
}

func splitJSONBaseField(field string) (string, bool) {
	if strings.Contains(field, "->") {
		return "", false
	}

	parts := strings.Split(field, ".")
	if len(parts) >= 2 {
		return parts[0], true
	}
	return "", false
}

func (v *Validator) checkDataAccess(ctx context.Context, scope *validatorAccessScope) error {
	ids, err := scope.accessibleDatabaseIDs()
	if err != nil {
		return fmt.Errorf("data access permission check failed: %w", err)
	}
	if len(ids) == 0 {
		return errors.New("no databases available")
	}
	return nil
}

func qualifyBaseField(table, field string) string {
	field = strings.TrimSpace(field)
	if field == "" || strings.Contains(field, ".") {
		return field
	}
	table = strings.TrimSpace(table)
	if table == "" {
		return field
	}
	return table + "." + field
}

func (v *Validator) GetAllowedTables(ctx context.Context, userID string) ([]string, error) {
	allowed := make([]string, 0)
	scope, err := v.newAccessScope(userID)
	if err != nil {
		return nil, err
	}
	authorizer := scope.authorizer
	for table := range v.allowedTables {
		if table == "tokens" && !authorizer.IsMaster() {
			continue
		}
		allowed = append(allowed, table)
	}
	return allowed, nil
}

func (v *Validator) AutoFilterByPermission(req *QueryRequest, userID string) error {
	scope, err := v.newAccessScope(userID)
	if err != nil {
		return err
	}
	return v.autoFilterByPermissionWithScope(req, scope)
}

func (v *Validator) autoFilterByPermissionWithScope(req *QueryRequest, scope *validatorAccessScope) error {
	if req.Where == nil {
		req.Where = &WhereClause{}
	}

	switch req.From {
	case "databases":
		dbIDs, err := scope.accessibleDatabaseIDs()
		if err != nil {
			return err
		}
		if len(dbIDs) == 0 {
			return errors.New("no databases available")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "id"), dbIDs)
	case "records":
		tableIDs, err := scope.accessibleTableIDs()
		if err != nil {
			return err
		}
		if len(tableIDs) == 0 {
			return errors.New("you do not have access to any tables")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "table_id"), tableIDs)
	case "tables":
		dbIDs, err := scope.accessibleDatabaseIDs()
		if err != nil {
			return err
		}
		if len(dbIDs) == 0 {
			return errors.New("you do not have access to any databases")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "database_id"), dbIDs)
	case "fields":
		tableIDs, err := scope.accessibleTableIDs()
		if err != nil {
			return err
		}
		if len(tableIDs) == 0 {
			return errors.New("you do not have access to any tables")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "table_id"), tableIDs)
	case "files":
		recordIDs, err := scope.accessibleRecordIDs()
		if err != nil {
			return err
		}
		if len(recordIDs) == 0 {
			req.Where.And = append([]Condition{{
				Field: qualifyBaseField(req.From, "record_id"),
				Op:    "eq",
				Value: "__no_accessible_record__",
			}}, req.Where.And...)
			return nil
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "record_id"), recordIDs)
	}

	return nil
}

func (v *Validator) newAccessScope(userID string) (*validatorAccessScope, error) {
	authorizer, err := authz.NewAuthorizer(v.db, userID)
	if err != nil {
		return nil, err
	}

	return &validatorAccessScope{authorizer: authorizer}, nil
}

func (s *validatorAccessScope) accessibleDatabaseIDs() ([]string, error) {
	if s.databaseIDsLoaded {
		return s.databaseIDs, nil
	}

	ids, err := s.authorizer.AccessibleDatabaseIDs()
	if err != nil {
		return nil, err
	}
	s.databaseIDs = ids
	s.databaseIDsLoaded = true
	return s.databaseIDs, nil
}

func (s *validatorAccessScope) accessibleTableIDs() ([]string, error) {
	if s.tableIDsLoaded {
		return s.tableIDs, nil
	}

	ids, err := s.authorizer.AccessibleTableIDs()
	if err != nil {
		return nil, err
	}
	s.tableIDs = ids
	s.tableIDsLoaded = true
	return s.tableIDs, nil
}

func (s *validatorAccessScope) accessibleRecordIDs() ([]string, error) {
	if s.recordIDsLoaded {
		return s.recordIDs, nil
	}

	ids, err := s.authorizer.AccessibleRecordIDs()
	if err != nil {
		return nil, err
	}
	s.recordIDs = ids
	s.recordIDsLoaded = true
	return s.recordIDs, nil
}

func appendInCondition(where *WhereClause, field string, values []string) {
	if where == nil {
		return
	}

	converted := make([]interface{}, 0, len(values))
	for _, value := range values {
		converted = append(converted, value)
	}

	where.And = append([]Condition{{
		Field: field,
		Op:    "in",
		Value: converted,
	}}, where.And...)
}

func (v *Validator) FilterFieldsByPermission(ctx context.Context, data []map[string]interface{}, table, userID string) ([]map[string]interface{}, error) {
	allowedFields := v.allowedTables.GetAllowedFields(table)
	if len(allowedFields) == 0 {
		return data, nil
	}

	allowedMap := make(map[string]bool)
	for _, f := range allowedFields {
		allowedMap[f] = true
	}

	filtered := make([]map[string]interface{}, len(data))
	for i, item := range data {
		filteredItem := make(map[string]interface{})
		for key, value := range item {
			if allowedMap[key] || allowedMap["*"] {
				filteredItem[key] = value
			}
		}
		filtered[i] = filteredItem
	}

	return filtered, nil
}

func (v *Validator) GetSelectableFields(table string) []string {
	fields := v.allowedTables.GetAllowedFields(table)
	if len(fields) == 0 {
		return nil
	}

	result := make([]string, len(fields))
	copy(result, fields)
	return result
}
