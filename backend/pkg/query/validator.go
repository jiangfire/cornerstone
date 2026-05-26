package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type Validator struct {
	db            *gorm.DB
	allowedTables AllowedTables
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
	if req == nil {
		return errors.New("查询请求不能为空")
	}

	if err := v.CheckTableAccess(ctx, userID, req.From); err != nil {
		return err
	}

	for _, field := range req.Select {
		if field == "*" {
			continue
		}
		if err := v.checkFieldReference(ctx, userID, req.From, req.Join, field); err != nil {
			return err
		}
	}

	for _, join := range req.Join {
		if err := v.CheckTableAccess(ctx, userID, join.Table); err != nil {
			return err
		}
		for _, field := range join.Select {
			if field == "*" {
				continue
			}
			if err := v.checkFieldReference(ctx, userID, join.Table, req.Join, field); err != nil {
				return err
			}
		}
	}

	if req.Where != nil {
		if err := v.validateWhereFields(ctx, userID, req.From, req.Join, req.Where); err != nil {
			return err
		}
	}

	for _, order := range req.OrderBy {
		if err := v.checkFieldReference(ctx, userID, req.From, req.Join, order.Field); err != nil {
			return err
		}
	}

	for _, group := range req.GroupBy {
		if err := v.checkFieldReference(ctx, userID, req.From, req.Join, group); err != nil {
			return err
		}
	}

	for _, agg := range req.Aggregate {
		if agg.Field != "" && agg.Field != "*" {
			if err := v.checkFieldReference(ctx, userID, req.From, req.Join, agg.Field); err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *Validator) validateWhereFields(ctx context.Context, userID string, table string, joins []JoinClause, where *WhereClause) error {
	for _, cond := range where.And {
		if err := v.validateConditionFields(ctx, userID, table, joins, cond); err != nil {
			return err
		}
	}
	for _, cond := range where.Or {
		if err := v.validateConditionFields(ctx, userID, table, joins, cond); err != nil {
			return err
		}
	}
	return nil
}

func (v *Validator) validateConditionFields(ctx context.Context, userID string, table string, joins []JoinClause, cond Condition) error {
	if cond.Field != "" {
		if err := v.checkFieldReference(ctx, userID, table, joins, cond.Field); err != nil {
			return err
		}
	}

	for _, nested := range cond.And {
		if err := v.validateConditionFields(ctx, userID, table, joins, nested); err != nil {
			return err
		}
	}
	for _, nested := range cond.Or {
		if err := v.validateConditionFields(ctx, userID, table, joins, nested); err != nil {
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
	field = strings.TrimSpace(field)
	if field == "" || field == "*" {
		return nil
	}

	parts := strings.Split(field, ".")
	if len(parts) >= 2 {
		if refTable := v.resolveReferenceTable(baseTable, joins, parts[0]); refTable != "" {
			if err := v.CheckTableAccess(ctx, userID, refTable); err != nil {
				return err
			}
			return v.CheckFieldAccess(ctx, userID, refTable, strings.Join(parts[1:], "."))
		}

		if v.allowedTables.IsFieldAllowed(baseTable, parts[0]) {
			return v.CheckFieldAccess(ctx, userID, baseTable, field)
		}
	}

	return v.CheckFieldAccess(ctx, userID, baseTable, field)
}

func (v *Validator) CheckTableAccess(ctx context.Context, userID string, table string) error {
	if !v.allowedTables.IsTableAllowed(table) {
		return fmt.Errorf("表 '%s' 不在允许访问的列表中", table)
	}

	switch table {
	case "databases", "records", "tables", "fields", "files":
		return v.checkDataAccess(ctx, userID)
	default:
		return nil
	}
}

func (v *Validator) CheckFieldAccess(ctx context.Context, userID string, table, field string) error {
	if baseField, ok := splitJSONBaseField(field); ok && isJSONColumnCandidate(baseField) {
		field = baseField
	}

	if !v.allowedTables.IsFieldAllowed(table, field) {
		return fmt.Errorf("字段 '%s.%s' 不在允许访问的列表中", table, field)
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

func (v *Validator) checkDataAccess(ctx context.Context, userID string) error {
	var count int64
	if err := v.db.Table("databases").Count(&count).Error; err != nil {
		return fmt.Errorf("检查数据访问权限失败: %w", err)
	}
	if count == 0 {
		return errors.New("没有可用的数据库")
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
	for table := range v.allowedTables {
		allowed = append(allowed, table)
	}
	return allowed, nil
}

func (v *Validator) AutoFilterByPermission(req *QueryRequest, userID string) error {
	if req.Where == nil {
		req.Where = &WhereClause{}
	}

	switch req.From {
	case "databases":
		var count int64
		if err := v.db.Table("databases").Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("没有可用的数据库")
		}
	case "records":
		tableIDs, err := v.getAccessibleTableIDs(userID)
		if err != nil {
			return err
		}
		if len(tableIDs) == 0 {
			return errors.New("您没有访问任何表的权限")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "table_id"), tableIDs)
	case "tables":
		dbIDs, err := v.getAccessibleDatabaseIDs(userID)
		if err != nil {
			return err
		}
		if len(dbIDs) == 0 {
			return errors.New("您没有访问任何数据库的权限")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "database_id"), dbIDs)
	case "fields":
		tableIDs, err := v.getAccessibleTableIDs(userID)
		if err != nil {
			return err
		}
		if len(tableIDs) == 0 {
			return errors.New("您没有访问任何表的权限")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "table_id"), tableIDs)
	case "files":
		recordIDs, err := v.getAccessibleRecordIDs(userID)
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

func (v *Validator) getAccessibleDatabaseIDs(userID string) ([]string, error) {
	var dbIDs []string
	if err := v.db.Table("databases").Pluck("id", &dbIDs).Error; err != nil {
		return nil, err
	}
	return dbIDs, nil
}

func (v *Validator) getAccessibleTableIDs(userID string) ([]string, error) {
	dbIDs, err := v.getAccessibleDatabaseIDs(userID)
	if err != nil {
		return nil, err
	}
	if len(dbIDs) == 0 {
		return []string{}, nil
	}

	var tableIDs []string
	if err := v.db.Table("tables").Where("database_id IN ?", dbIDs).Pluck("id", &tableIDs).Error; err != nil {
		return nil, err
	}
	return tableIDs, nil
}

func (v *Validator) getAccessibleRecordIDs(userID string) ([]string, error) {
	tableIDs, err := v.getAccessibleTableIDs(userID)
	if err != nil {
		return nil, err
	}
	if len(tableIDs) == 0 {
		return []string{}, nil
	}

	var recordIDs []string
	if err := v.db.Table("records").Where("table_id IN ?", tableIDs).Pluck("id", &recordIDs).Error; err != nil {
		return nil, err
	}

	return recordIDs, nil
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
