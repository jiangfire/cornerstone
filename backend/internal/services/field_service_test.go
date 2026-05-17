package services

import (
	"strings"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestFieldService_DeleteFieldSoftDeleteAndAllowsRecreate(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_delete")
	database := createResourceDatabase(t, db, owner.ID, "FieldDeleteDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	field, err := service.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "status",
		Type:    "string",
	}, owner.ID)
	require.NoError(t, err)

	require.NoError(t, service.DeleteField(field.ID, owner.ID))

	var stored models.Field
	require.NoError(t, db.Unscoped().Where("id = ?", field.ID).First(&stored).Error)
	require.True(t, stored.DeletedAt.Valid)
	require.Contains(t, stored.Name, "__deleted__")

	_, err = service.GetField(field.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "字段不存在")

	recreated, err := service.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "status",
		Type:    "string",
	}, owner.ID)
	require.NoError(t, err)
	require.NotEqual(t, field.ID, recreated.ID)
}

func TestFieldService_DeniesAccessWhenTableDeleted(t *testing.T) {
	db := setupResourceTestDB(t)
	fieldService := NewFieldService(db)
	tableService := NewTableService(db)

	owner := createResourceUser(t, db, "field_owner_deleted_table")
	database := createResourceDatabase(t, db, owner.ID, "FieldParentDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	field := createResourceField(t, db, table.ID, "status", "string", false, "")

	require.NoError(t, tableService.DeleteTable(table.ID, owner.ID))

	_, err := fieldService.ListFields(table.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "表不存在")

	_, err = fieldService.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "priority",
		Type:    "string",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "表不存在")

	_, err = fieldService.GetField(field.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "表不存在")
}

func TestFieldService_CreateFieldRejectsDuplicateActiveName(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_duplicate")
	database := createResourceDatabase(t, db, owner.ID, "FieldDuplicateDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	createResourceField(t, db, table.ID, "status", "string", false, "")

	_, err := service.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "status",
		Type:    "string",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "该表中已存在同名字段")
}

func TestFieldService_UpdateFieldRejectsDuplicateActiveName(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_update_duplicate")
	database := createResourceDatabase(t, db, owner.ID, "FieldUpdateDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	first := createResourceField(t, db, table.ID, "status", "string", false, "")
	second := createResourceField(t, db, table.ID, "priority", "string", false, "")

	_, err := service.UpdateField(second.ID, UpdateFieldRequest{
		Name: "status",
		Type: "string",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "该表中已存在同名字段")

	current, err := service.GetField(second.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "priority", current.Name)
	require.Equal(t, first.TableID, current.TableID)
}

func TestFieldService_SetFieldPermissionRejectsDeletedField(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_permission_deleted")
	database := createResourceDatabase(t, db, owner.ID, "FieldPermissionDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	field := createResourceField(t, db, table.ID, "status", "string", false, "")

	require.NoError(t, service.DeleteField(field.ID, owner.ID))

	err := service.SetFieldPermission(table.ID, FieldPermissionRequest{
		FieldID:   field.ID,
		Role:      "viewer",
		CanRead:   true,
		CanWrite:  false,
		CanDelete: false,
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "字段不存在")
}

func TestFieldService_SetFieldPermissionPersistsExplicitFalseValues(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_permission_false")
	viewer := createResourceUser(t, db, "field_viewer_permission_false")
	database := createResourceDatabase(t, db, owner.ID, "FieldPermissionFalseDB")
	grantResourceDatabaseAccess(t, db, database.ID, viewer.ID, "viewer")

	table := createResourceTable(t, db, database.ID, "Orders")
	field := createResourceField(t, db, table.ID, "secret", "string", false, "")

	require.NoError(t, service.SetFieldPermission(table.ID, FieldPermissionRequest{
		FieldID:   field.ID,
		Role:      "viewer",
		CanRead:   false,
		CanWrite:  false,
		CanDelete: false,
	}, owner.ID))

	err := service.CheckFieldPermission(viewer.ID, field.ID, "read")
	require.Error(t, err)
	require.Contains(t, err.Error(), "无读取权限")

	var permission models.FieldPermission
	require.NoError(t, db.Where("field_id = ? AND role = ?", field.ID, "viewer").First(&permission).Error)
	require.False(t, permission.CanRead)
	require.False(t, permission.CanWrite)
	require.False(t, permission.CanDelete)
}

func TestFieldService_CreateAndGetFieldWithDescription(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_description_create")
	database := createResourceDatabase(t, db, owner.ID, "FieldDescriptionDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	field, err := service.CreateField(CreateFieldRequest{
		TableID:     table.ID,
		Name:        "status",
		Type:        "string",
		Description: "  订单当前状态，用于业务流转  ",
	}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "订单当前状态，用于业务流转", field.Description)

	detail, err := service.GetField(field.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "订单当前状态，用于业务流转", detail.Description)

	fields, err := service.ListFields(table.ID, owner.ID)
	require.NoError(t, err)
	require.Len(t, fields, 1)
	require.Equal(t, "订单当前状态，用于业务流转", fields[0].Description)
}

func TestFieldService_UpdateFieldDescription(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_description_update")
	database := createResourceDatabase(t, db, owner.ID, "FieldDescriptionUpdateDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	field := createResourceField(t, db, table.ID, "status", "string", false, "")

	updated, err := service.UpdateField(field.ID, UpdateFieldRequest{
		Name:        "status",
		Type:        "string",
		Description: "  审核后的状态字段  ",
	}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "审核后的状态字段", updated.Description)

	detail, err := service.GetField(field.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "审核后的状态字段", detail.Description)
}

func TestFieldService_RejectsTooLongDescription(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_description_invalid")
	database := createResourceDatabase(t, db, owner.ID, "FieldDescriptionInvalidDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	tooLongDescription := strings.Repeat("a", 1001)

	_, err := service.CreateField(CreateFieldRequest{
		TableID:     table.ID,
		Name:        "status",
		Type:        "string",
		Description: tooLongDescription,
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "字段备注长度不能超过1000个字符")
}

func TestFieldService_RejectsDeprecatedFieldTypesForCreate(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_deprecated_create")
	database := createResourceDatabase(t, db, owner.ID, "FieldDeprecatedCreateDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	for _, deprecatedType := range []string{"select", "list", "single_select", "multi_select", "multiselect"} {
		_, err := service.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "field_" + strings.ReplaceAll(deprecatedType, "_", ""),
			Type:    deprecatedType,
		}, owner.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "已废弃")
	}
}

func TestFieldService_RejectsDeprecatedFieldTypesForUpdate(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_deprecated_update")
	database := createResourceDatabase(t, db, owner.ID, "FieldDeprecatedUpdateDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	field := createResourceField(t, db, table.ID, "status", "string", false, "")

	_, err := service.UpdateField(field.ID, UpdateFieldRequest{
		Name: "status",
		Type: "select",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "已废弃")
}

func TestFieldService_ListAndGetFieldsMarkDeprecatedTypes(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_deprecated_list")
	database := createResourceDatabase(t, db, owner.ID, "FieldDeprecatedListDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	legacyField := createResourceField(t, db, table.ID, "status", "single_select", false, `{"options":["draft","done"]}`)
	normalField := createResourceField(t, db, table.ID, "title", "string", false, "")

	fields, err := service.ListFields(table.ID, owner.ID)
	require.NoError(t, err)
	require.Len(t, fields, 2)

	var legacyFound, normalFound bool
	for _, field := range fields {
		switch field.ID {
		case legacyField.ID:
			legacyFound = true
			require.True(t, field.Deprecated)
			require.Equal(t, "select", field.Type)
		case normalField.ID:
			normalFound = true
			require.False(t, field.Deprecated)
			require.Equal(t, "string", field.Type)
		}
	}
	require.True(t, legacyFound)
	require.True(t, normalFound)

	detail, err := service.GetField(legacyField.ID, owner.ID)
	require.NoError(t, err)
	require.True(t, detail.Deprecated)
	require.Equal(t, "select", detail.Type)
}

func TestFieldService_CreateAttachmentFieldWithConfig(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewFieldService(db)

	owner := createResourceUser(t, db, "field_owner_attachment_create")
	database := createResourceDatabase(t, db, owner.ID, "FieldAttachmentCreateDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	field, err := service.CreateField(CreateFieldRequest{
		TableID:     table.ID,
		Name:        "attachments",
		Type:        "attachment",
		Description: "附件字段",
		Config: FieldConfig{
			AllowedTypes:  []string{"image/*", ".pdf"},
			MaxFileSizeMB: 10,
			Multiple:      true,
		},
	}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "attachment", field.Type)

	detail, err := service.GetField(field.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "attachment", detail.Type)
	require.Equal(t, []string{"image/*", ".pdf"}, detail.Config.AllowedTypes)
	require.Equal(t, 10, detail.Config.MaxFileSizeMB)
	require.True(t, detail.Config.Multiple)
}
