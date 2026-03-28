package services

import (
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
	require.NoError(t, db.Where("id = ?", field.ID).First(&stored).Error)
	require.NotNil(t, stored.DeletedAt)
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
