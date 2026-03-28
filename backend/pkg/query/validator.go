package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// Validator 查询权限验证器
type Validator struct {
	db            *gorm.DB
	allowedTables AllowedTables
}

// NewValidator 创建验证器
func NewValidator(db *gorm.DB) *Validator {
	return &Validator{
		db:            db,
		allowedTables: DefaultAllowedTables,
	}
}

// NewValidatorWithTables 创建带自定义表白名单的验证器
func NewValidatorWithTables(db *gorm.DB, tables AllowedTables) *Validator {
	return &Validator{
		db:            db,
		allowedTables: tables,
	}
}

// ValidateRequest 验证查询请求
func (v *Validator) ValidateRequest(ctx context.Context, req *QueryRequest, userID string) error {
	if req == nil {
		return errors.New("查询请求不能为空")
	}

	// 1. 验证表访问权限
	if err := v.CheckTableAccess(ctx, userID, req.From); err != nil {
		return err
	}

	// 2. 验证字段访问权限
	for _, field := range req.Select {
		if field == "*" {
			continue
		}
		if err := v.checkFieldReference(ctx, userID, req.From, req.Join, field); err != nil {
			return err
		}
	}

	// 3. 验证 JOIN 表权限
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

	// 4. 验证 WHERE 条件字段权限
	if req.Where != nil {
		if err := v.validateWhereFields(ctx, userID, req.From, req.Join, req.Where); err != nil {
			return err
		}
	}

	// 5. 验证 ORDER BY 字段权限
	for _, order := range req.OrderBy {
		if err := v.checkFieldReference(ctx, userID, req.From, req.Join, order.Field); err != nil {
			return err
		}
	}

	// 6. 验证 GROUP BY 字段权限
	for _, group := range req.GroupBy {
		if err := v.checkFieldReference(ctx, userID, req.From, req.Join, group); err != nil {
			return err
		}
	}

	// 7. 验证聚合函数字段权限
	for _, agg := range req.Aggregate {
		if agg.Field != "" && agg.Field != "*" {
			if err := v.checkFieldReference(ctx, userID, req.From, req.Join, agg.Field); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateWhereFields 验证 WHERE 条件中的字段权限
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

// validateConditionFields 验证单个条件的字段权限
func (v *Validator) validateConditionFields(ctx context.Context, userID string, table string, joins []JoinClause, cond Condition) error {
	// 验证主字段
	if cond.Field != "" {
		if err := v.checkFieldReference(ctx, userID, table, joins, cond.Field); err != nil {
			return err
		}
	}

	// 递归验证嵌套条件
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

// parseFieldReference 解析字段引用，返回表名和字段名
func (v *Validator) parseFieldReference(field string) (string, string) {
	parts := strings.Split(field, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", field
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

// CheckTableAccess 检查用户是否有表访问权限
func (v *Validator) CheckTableAccess(ctx context.Context, userID string, table string) error {
	// 1. 检查表是否在白名单中
	if !v.allowedTables.IsTableAllowed(table) {
		return fmt.Errorf("表 '%s' 不在允许访问的列表中", table)
	}

	// 2. 特殊表权限检查
	switch table {
	case "databases", "records", "tables", "fields", "plugin_bindings", "plugin_executions":
		// 这些表需要检查数据库访问权限
		return v.checkDataAccess(ctx, userID)
	case "files":
		return v.checkDataAccess(ctx, userID)
	case "database_access", "field_permissions":
		// 权限表需要管理员权限
		return v.checkAdminAccess(ctx, userID)
	case "organizations", "organization_members":
		return v.checkOrganizationAccess(ctx, userID)
	case "plugins":
		return nil
	case "activity_logs":
		return nil
	case "users":
		// 用户表可以访问（但只能看自己的敏感信息）
		return nil
	default:
		return nil
	}
}

// CheckFieldAccess 检查用户是否有字段访问权限
func (v *Validator) CheckFieldAccess(ctx context.Context, userID string, table, field string) error {
	if baseField, ok := splitJSONBaseField(field); ok && isJSONColumnCandidate(baseField) {
		field = baseField
	}

	// 1. 检查字段是否在白名单中
	if !v.allowedTables.IsFieldAllowed(table, field) {
		return fmt.Errorf("字段 '%s.%s' 不在允许访问的列表中", table, field)
	}

	// 2. 特殊字段权限检查
	// 例如：users 表的 password 字段不允许查询
	if table == "users" && field == "password" {
		return errors.New("无权访问 password 字段")
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

func (v *Validator) isJSONFieldReference(table, field string) bool {
	baseField, ok := splitJSONBaseField(field)
	if !ok {
		return false
	}
	return v.allowedTables.IsFieldAllowed(table, baseField)
}

// checkDataAccess 检查数据访问权限
func (v *Validator) checkDataAccess(ctx context.Context, userID string) error {
	// 检查用户是否至少有一个数据库的访问权限
	var count int64
	if err := v.db.Model(&models.DatabaseAccess{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return fmt.Errorf("检查数据访问权限失败: %w", err)
	}
	if count == 0 {
		return errors.New("您没有访问任何数据库的权限")
	}
	return nil
}

// checkAdminAccess 检查管理员权限
func (v *Validator) checkAdminAccess(ctx context.Context, userID string) error {
	// 检查用户是否是任何数据库的所有者或管理员
	var count int64
	if err := v.db.Model(&models.DatabaseAccess{}).
		Where("user_id = ? AND role IN ?", userID, []string{"owner", "admin"}).
		Count(&count).Error; err != nil {
		return fmt.Errorf("检查管理员权限失败: %w", err)
	}
	if count == 0 {
		return errors.New("需要管理员权限才能访问此资源")
	}
	return nil
}

// checkOrganizationAccess 检查组织访问权限
func (v *Validator) checkOrganizationAccess(ctx context.Context, userID string) error {
	var count int64
	if err := v.db.Model(&models.Organization{}).
		Where("owner_id = ?", userID).
		Or("id IN (SELECT organization_id FROM organization_members WHERE user_id = ?)", userID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("检查组织权限失败: %w", err)
	}
	if count == 0 {
		return errors.New("您没有访问任何组织的权限")
	}
	return nil
}

func (v *Validator) getAccessibleDatabaseIDs(userID string, roles ...string) ([]string, error) {
	query := v.db.Model(&models.DatabaseAccess{}).Where("user_id = ?", userID)
	if len(roles) > 0 {
		query = query.Where("role IN ?", roles)
	}

	var dbIDs []string
	if err := query.Pluck("database_id", &dbIDs).Error; err != nil {
		return nil, err
	}
	return dbIDs, nil
}

func (v *Validator) getAccessibleTableIDsForDatabases(databaseIDs []string) ([]string, error) {
	if len(databaseIDs) == 0 {
		return []string{}, nil
	}

	var tableIDs []string
	if err := v.db.Model(&models.Table{}).Where("database_id IN ?", databaseIDs).Pluck("id", &tableIDs).Error; err != nil {
		return nil, err
	}
	return tableIDs, nil
}

func (v *Validator) getAccessibleOrganizationIDs(userID string) ([]string, error) {
	var orgIDs []string
	if err := v.db.Model(&models.Organization{}).
		Where("owner_id = ?", userID).
		Or("id IN (SELECT organization_id FROM organization_members WHERE user_id = ?)", userID).
		Pluck("id", &orgIDs).Error; err != nil {
		return nil, err
	}
	return orgIDs, nil
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

// GetAllowedTables 返回用户可访问的表列表
func (v *Validator) GetAllowedTables(ctx context.Context, userID string) ([]string, error) {
	// 首先获取用户有权限的数据库列表
	var dbAccess []models.DatabaseAccess
	if err := v.db.Where("user_id = ?", userID).Find(&dbAccess).Error; err != nil {
		return nil, fmt.Errorf("获取数据库权限失败: %w", err)
	}

	// 收集所有可访问的表
	allowed := make([]string, 0)
	for table := range v.allowedTables {
		switch table {
		case "databases", "records", "tables", "fields", "plugin_bindings", "plugin_executions", "files":
			// 需要数据访问权限
			if len(dbAccess) > 0 {
				allowed = append(allowed, table)
			}
		case "database_access", "field_permissions":
			for _, access := range dbAccess {
				if access.Role == "owner" || access.Role == "admin" {
					allowed = append(allowed, table)
					break
				}
			}
		case "organizations", "organization_members":
			orgIDs, err := v.getAccessibleOrganizationIDs(userID)
			if err != nil {
				return nil, fmt.Errorf("获取组织权限失败: %w", err)
			}
			if len(orgIDs) > 0 {
				allowed = append(allowed, table)
			}
		case "plugins", "activity_logs", "users":
			allowed = append(allowed, table)
		default:
			allowed = append(allowed, table)
		}
	}

	return allowed, nil
}

// AutoFilterByPermission 根据用户权限自动添加过滤条件
func (v *Validator) AutoFilterByPermission(req *QueryRequest, userID string) error {
	if req.Where == nil {
		req.Where = &WhereClause{}
	}

	switch req.From {
	case "databases":
		dbIDs, err := v.getAccessibleDatabaseIDs(userID)
		if err != nil {
			return err
		}
		if len(dbIDs) == 0 {
			return errors.New("您没有访问任何数据库的权限")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "id"), dbIDs)
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
	case "fields", "plugin_bindings", "plugin_executions":
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
	case "database_access":
		dbIDs, err := v.getAccessibleDatabaseIDs(userID, "owner", "admin")
		if err != nil {
			return err
		}
		if len(dbIDs) == 0 {
			return errors.New("需要管理员权限才能访问数据库权限数据")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "database_id"), dbIDs)
	case "field_permissions":
		dbIDs, err := v.getAccessibleDatabaseIDs(userID, "owner", "admin")
		if err != nil {
			return err
		}
		if len(dbIDs) == 0 {
			return errors.New("需要管理员权限才能访问字段权限数据")
		}
		tableIDs, err := v.getAccessibleTableIDsForDatabases(dbIDs)
		if err != nil {
			return err
		}
		if len(tableIDs) == 0 {
			return errors.New("管理员权限下没有可访问的表")
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, "table_id"), tableIDs)
	case "organizations", "organization_members":
		orgIDs, err := v.getAccessibleOrganizationIDs(userID)
		if err != nil {
			return err
		}
		if len(orgIDs) == 0 {
			return errors.New("您没有访问任何组织的权限")
		}
		field := "id"
		if req.From == "organization_members" {
			field = "organization_id"
		}
		appendInCondition(req.Where, qualifyBaseField(req.From, field), orgIDs)
	case "plugins":
		req.Where.And = append([]Condition{{
			Field: qualifyBaseField(req.From, "created_by"),
			Op:    "eq",
			Value: userID,
		}}, req.Where.And...)
	case "activity_logs":
		req.Where.Or = append(req.Where.Or,
			Condition{Field: qualifyBaseField(req.From, "user_id"), Op: "eq", Value: userID},
			Condition{Field: qualifyBaseField(req.From, "user_id"), Op: "like", Value: "system:%"},
		)
	}

	return nil
}

// getAccessibleTableIDs 获取用户可访问的表 ID 列表
func (v *Validator) getAccessibleTableIDs(userID string) ([]string, error) {
	// 获取用户有权限的数据库
	var dbAccess []models.DatabaseAccess
	if err := v.db.Where("user_id = ?", userID).Find(&dbAccess).Error; err != nil {
		return nil, err
	}

	if len(dbAccess) == 0 {
		return []string{}, nil
	}

	dbIDs := make([]string, len(dbAccess))
	for i, access := range dbAccess {
		dbIDs[i] = access.DatabaseID
	}

	// 获取这些数据库下的所有表
	var tables []models.Table
	if err := v.db.Where("database_id IN ?", dbIDs).Find(&tables).Error; err != nil {
		return nil, err
	}

	tableIDs := make([]string, len(tables))
	for i, table := range tables {
		tableIDs[i] = table.ID
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
	if err := v.db.Model(&models.Record{}).Where("table_id IN ?", tableIDs).Pluck("id", &recordIDs).Error; err != nil {
		return nil, err
	}

	return recordIDs, nil
}

// FilterFieldsByPermission 根据字段权限过滤查询结果
func (v *Validator) FilterFieldsByPermission(ctx context.Context, data []map[string]interface{}, table, userID string) ([]map[string]interface{}, error) {
	// 获取允许的字段列表
	allowedFields := v.allowedTables.GetAllowedFields(table)
	if len(allowedFields) == 0 {
		return data, nil // 没有限制
	}

	allowedMap := make(map[string]bool)
	for _, f := range allowedFields {
		allowedMap[f] = true
	}

	// 过滤每个数据项的字段
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

// GetSelectableFields 返回表允许的显式可选字段列表
func (v *Validator) GetSelectableFields(table string) []string {
	fields := v.allowedTables.GetAllowedFields(table)
	if len(fields) == 0 {
		return nil
	}

	result := make([]string, len(fields))
	copy(result, fields)
	return result
}
