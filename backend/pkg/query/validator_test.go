package query

import (
	"context"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupValidatorTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.DatabaseAccess{},
		&models.Database{},
		&models.Table{},
		&models.Record{},
		&models.File{},
		&models.Plugin{},
		&models.PluginBinding{},
		&models.PluginExecution{},
		&models.Organization{},
		&models.OrganizationMember{},
	))

	return db
}

func TestValidator_AutoFilterByPermissionForTables(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_reader",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_blocked",
		DatabaseID: "db_blocked",
		Name:       "BlockedTable",
	}).Error)

	req := &QueryRequest{
		From:   "tables",
		Select: []string{"id", "database_id"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_reader"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "tables.database_id", req.Where.And[0].Field)
}

func TestValidator_GetAllowedTablesRespectsAdminScope(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_viewer",
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	tables, err := validator.GetAllowedTables(context.Background(), "usr_viewer")
	require.NoError(t, err)
	require.NotContains(t, tables, "database_access")
	require.NotContains(t, tables, "field_permissions")
	require.Contains(t, tables, "tables")
	require.Contains(t, tables, "records")
}

func TestValidator_GetAllowedTablesIncludesAdminTablesForOwner(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_owner",
		DatabaseID: "db_owner",
		Role:       "owner",
	}).Error)

	tables, err := validator.GetAllowedTables(context.Background(), "usr_owner")
	require.NoError(t, err)
	require.Contains(t, tables, "database_access")
	require.Contains(t, tables, "field_permissions")
}

func TestValidator_GetAllowedTablesIncludesDatabasesWhenUserHasAccess(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_viewer",
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	tables, err := validator.GetAllowedTables(context.Background(), "usr_viewer")
	require.NoError(t, err)
	require.Contains(t, tables, "databases")
}

func TestValidator_CheckFieldAccessAllowsJSONPathByBaseField(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	err := validator.CheckFieldAccess(context.Background(), "usr_any", "records", "data.status")
	require.NoError(t, err)
}

func TestValidator_AutoFilterByPermissionForFieldPermissionsUsesAdminTables(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_admin",
		DatabaseID: "db_admin",
		Role:       "admin",
	}).Error)
	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_admin",
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_admin_1",
		DatabaseID: "db_admin",
		Name:       "AdminTable1",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_admin_2",
		DatabaseID: "db_admin",
		Name:       "AdminTable2",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_view_only",
		DatabaseID: "db_view",
		Name:       "ViewOnlyTable",
	}).Error)

	req := &QueryRequest{
		From:   "field_permissions",
		Select: []string{"id", "table_id", "role"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_admin"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "field_permissions.table_id", req.Where.And[0].Field)
	require.Equal(t, "in", req.Where.And[0].Op)
	require.ElementsMatch(t, []interface{}{"tbl_admin_1", "tbl_admin_2"}, req.Where.And[0].Value)
}

func TestValidator_AutoFilterByPermissionForDatabaseAccessUsesOwnerAndAdminDatabases(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_manager",
		DatabaseID: "db_owner",
		Role:       "owner",
	}).Error)
	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_manager",
		DatabaseID: "db_admin",
		Role:       "admin",
	}).Error)
	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_manager",
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	req := &QueryRequest{
		From:   "database_access",
		Select: []string{"id", "database_id", "role"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_manager"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "database_access.database_id", req.Where.And[0].Field)
	require.Equal(t, "in", req.Where.And[0].Op)
	require.ElementsMatch(t, []interface{}{"db_owner", "db_admin"}, req.Where.And[0].Value)
}

func TestValidator_AutoFilterByPermissionForDatabasesUsesAccessibleIDs(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_viewer",
		DatabaseID: "db_allowed_1",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_viewer",
		DatabaseID: "db_allowed_2",
		Role:       "editor",
	}).Error)

	req := &QueryRequest{
		From:   "databases",
		Select: []string{"id", "name"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_viewer"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "databases.id", req.Where.And[0].Field)
	require.Equal(t, "in", req.Where.And[0].Op)
	require.ElementsMatch(t, []interface{}{"db_allowed_1", "db_allowed_2"}, req.Where.And[0].Value)
}

func TestValidator_AutoFilterByPermissionForFieldPermissionsUsesOwnerAndAdminTables(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_manager",
		DatabaseID: "db_owner",
		Role:       "owner",
	}).Error)
	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_manager",
		DatabaseID: "db_admin",
		Role:       "admin",
	}).Error)
	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_manager",
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_owner",
		DatabaseID: "db_owner",
		Name:       "OwnerTable",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_admin",
		DatabaseID: "db_admin",
		Name:       "AdminTable",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_view",
		DatabaseID: "db_view",
		Name:       "ViewOnlyTable",
	}).Error)

	req := &QueryRequest{
		From:   "field_permissions",
		Select: []string{"id", "table_id", "role"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_manager"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "field_permissions.table_id", req.Where.And[0].Field)
	require.Equal(t, "in", req.Where.And[0].Op)
	require.ElementsMatch(t, []interface{}{"tbl_owner", "tbl_admin"}, req.Where.And[0].Value)
}

func TestValidator_AutoFilterByPermissionForFieldPermissionsWithoutManagedTablesFails(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_admin",
		DatabaseID: "db_admin",
		Role:       "admin",
	}).Error)

	req := &QueryRequest{
		From:   "field_permissions",
		Select: []string{"id", "table_id", "role"},
		Page:   1,
		Size:   20,
	}

	err := validator.AutoFilterByPermission(req, "usr_admin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "管理员权限下没有可访问的表")
}

func TestValidator_AutoFilterByPermissionForFilesUsesAccessibleRecordIDs(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_reader",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_blocked",
		DatabaseID: "db_blocked",
		Name:       "BlockedTable",
	}).Error)
	require.NoError(t, db.Create(&models.Record{
		ID:        "rec_allowed",
		TableID:   "tbl_allowed",
		Data:      `{"status":"ok"}`,
		CreatedBy: "usr_reader",
		UpdatedBy: "usr_reader",
		Version:   1,
	}).Error)
	require.NoError(t, db.Create(&models.Record{
		ID:        "rec_blocked",
		TableID:   "tbl_blocked",
		Data:      `{"status":"ok"}`,
		CreatedBy: "usr_reader",
		UpdatedBy: "usr_reader",
		Version:   1,
	}).Error)

	req := &QueryRequest{
		From:   "files",
		Select: []string{"id", "record_id", "file_name"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_reader"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "files.record_id", req.Where.And[0].Field)
	require.Equal(t, "in", req.Where.And[0].Op)
	require.ElementsMatch(t, []interface{}{"rec_allowed"}, req.Where.And[0].Value)
}

func TestValidator_AutoFilterByPermissionForFilesWithoutAccessibleRecordsUsesImpossibleCondition(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_reader",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)

	req := &QueryRequest{
		From:   "files",
		Select: []string{"id", "record_id"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_reader"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "files.record_id", req.Where.And[0].Field)
	require.Equal(t, "eq", req.Where.And[0].Op)
	require.Equal(t, "__no_accessible_record__", req.Where.And[0].Value)
}

func TestValidator_CheckFieldAccessRejectsNestedNonJSONField(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	err := validator.CheckFieldAccess(context.Background(), "usr_any", "users", "email.domain")
	require.Error(t, err)
	require.Contains(t, err.Error(), "users.email.domain")
}

func TestValidator_CheckTableAccessRejectsDatabasesWithoutAnyDatabaseAccess(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	err := validator.CheckTableAccess(context.Background(), "usr_none", "databases")
	require.Error(t, err)
	require.Contains(t, err.Error(), "您没有访问任何数据库的权限")
}

func TestValidator_CheckTableAccessRejectsFilesWithoutAnyDatabaseAccess(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	err := validator.CheckTableAccess(context.Background(), "usr_none", "files")
	require.Error(t, err)
	require.Contains(t, err.Error(), "您没有访问任何数据库的权限")
}

func TestValidator_ValidateRequestRejectsAliasedNestedNonJSONField(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_reader",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)

	req := &QueryRequest{
		From:   "tables",
		Select: []string{"tables.id", "db.name.extra"},
		Join: []JoinClause{
			{Type: "left", Table: "databases", As: "db", On: "db.id = tables.database_id"},
		},
		Page: 1,
		Size: 20,
	}

	err := validator.ValidateRequest(context.Background(), req, "usr_reader")
	require.Error(t, err)
	require.Contains(t, err.Error(), "databases.name.extra")
}

func TestValidator_ValidateRequestRejectsAdminTableForViewer(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_viewer",
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	req := &QueryRequest{
		From:   "database_access",
		Select: []string{"id", "database_id", "role"},
		Page:   1,
		Size:   20,
	}

	err := validator.ValidateRequest(context.Background(), req, "usr_viewer")
	require.Error(t, err)
	require.Contains(t, err.Error(), "管理员权限")
}

func TestValidator_AutoFilterByPermissionForActivityLogsPreservesSystemAndUserScope(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	req := &QueryRequest{
		From:   "activity_logs",
		Select: []string{"id", "user_id", "action"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_actor"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.Or, 2)
	require.Equal(t, "activity_logs.user_id", req.Where.Or[0].Field)
	require.Equal(t, "eq", req.Where.Or[0].Op)
	require.Equal(t, "usr_actor", req.Where.Or[0].Value)
	require.Equal(t, "activity_logs.user_id", req.Where.Or[1].Field)
	require.Equal(t, "like", req.Where.Or[1].Op)
	require.Equal(t, "system:%", req.Where.Or[1].Value)
}

func TestValidator_ValidateRequestRejectsSensitiveUserPasswordField(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	req := &QueryRequest{
		From:   "users",
		Select: []string{"id", "password"},
		Page:   1,
		Size:   20,
	}

	err := validator.ValidateRequest(context.Background(), req, "usr_any")
	require.Error(t, err)
	require.Contains(t, err.Error(), "不在允许访问的列表中")
}

func TestValidator_FilterFieldsByPermissionDropsDisallowedFields(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	input := []map[string]interface{}{
		{
			"id":       "usr_1",
			"email":    "user@example.com",
			"password": "secret",
			"unknown":  "hidden",
		},
	}

	filtered, err := validator.FilterFieldsByPermission(context.Background(), input, "users", "usr_any")
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "usr_1", filtered[0]["id"])
	require.Equal(t, "user@example.com", filtered[0]["email"])
	require.NotContains(t, filtered[0], "password")
	require.NotContains(t, filtered[0], "unknown")
}

func TestValidator_AutoFilterByPermissionForPluginsUsesCreatorScope(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	req := &QueryRequest{
		From:   "plugins",
		Select: []string{"id", "name", "created_by"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_creator"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Equal(t, "plugins.created_by", req.Where.And[0].Field)
	require.Equal(t, "eq", req.Where.And[0].Op)
	require.Equal(t, "usr_creator", req.Where.And[0].Value)
}

func TestValidator_AutoFilterByPermissionForPluginsPreservesExistingWhere(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	req := &QueryRequest{
		From:   "plugins",
		Select: []string{"id", "name"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "name", Op: "like", Value: "owned"},
			},
		},
		Page: 1,
		Size: 20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_creator"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 2)
	require.Equal(t, "plugins.created_by", req.Where.And[0].Field)
	require.Equal(t, "name", req.Where.And[1].Field)
}

func TestValidator_AutoFilterByPermissionForPluginBindingsUsesOwnedPluginsAndAccessibleTables(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_plugin_owner",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)
	require.NoError(t, db.Create(&models.Plugin{
		ID:        "plg_owned",
		Name:      "OwnedPlugin",
		Language:  "bash",
		EntryFile: "main.sh",
		CreatedBy: "usr_plugin_owner",
	}).Error)
	require.NoError(t, db.Create(&models.Plugin{
		ID:        "plg_other",
		Name:      "OtherPlugin",
		Language:  "bash",
		EntryFile: "main.sh",
		CreatedBy: "usr_other",
	}).Error)

	req := &QueryRequest{
		From:   "plugin_bindings",
		Select: []string{"id", "plugin_id", "table_id"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_plugin_owner"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 2)
	require.Equal(t, "plugin_bindings.plugin_id", req.Where.And[0].Field)
	require.Equal(t, "in", req.Where.And[0].Op)
	require.ElementsMatch(t, []interface{}{"plg_owned"}, req.Where.And[0].Value)
	require.Equal(t, "plugin_bindings.table_id", req.Where.And[1].Field)
}

func TestValidator_AutoFilterByPermissionForPluginExecutionsWithoutOwnedPluginsUsesImpossibleCondition(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_plugin_viewer",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)

	req := &QueryRequest{
		From:   "plugin_executions",
		Select: []string{"id", "plugin_id", "table_id"},
		Page:   1,
		Size:   20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_plugin_viewer"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 2)
	require.Equal(t, "plugin_executions.plugin_id", req.Where.And[0].Field)
	require.Equal(t, "eq", req.Where.And[0].Op)
	require.Equal(t, "__no_owned_plugin__", req.Where.And[0].Value)
	require.Equal(t, "plugin_executions.table_id", req.Where.And[1].Field)
}

func TestValidator_AutoFilterByPermissionForActivityLogsMergesExistingWhere(t *testing.T) {
	db := setupValidatorTestDB(t)
	validator := NewValidator(db)

	req := &QueryRequest{
		From:   "activity_logs",
		Select: []string{"id", "user_id", "action"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "action", Op: "eq", Value: "query"},
			},
		},
		Page: 1,
		Size: 20,
	}

	require.NoError(t, validator.AutoFilterByPermission(req, "usr_actor"))
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	require.Len(t, req.Where.Or, 2)
	require.Equal(t, "action", req.Where.And[0].Field)
	require.Equal(t, "activity_logs.user_id", req.Where.Or[0].Field)
	require.Equal(t, "activity_logs.user_id", req.Where.Or[1].Field)
}
