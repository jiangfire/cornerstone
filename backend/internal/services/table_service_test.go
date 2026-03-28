package services

import (
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestTableService_DeleteTableSoftDeleteAndAllowsRecreate(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewTableService(db)

	owner := createResourceUser(t, db, "table_owner_delete")
	admin := createResourceUser(t, db, "table_admin_delete")
	database := createResourceDatabase(t, db, owner.ID, "TableDeleteDB")
	grantResourceDatabaseAccess(t, db, database.ID, admin.ID, "admin")

	table, err := service.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "Orders",
	}, owner.ID)
	require.NoError(t, err)

	require.NoError(t, service.DeleteTable(table.ID, admin.ID))

	var stored models.Table
	require.NoError(t, db.Where("id = ?", table.ID).First(&stored).Error)
	require.NotNil(t, stored.DeletedAt)
	require.Contains(t, stored.Name, "__deleted__")

	_, err = service.GetTable(table.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "表不存在")

	recreated, err := service.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "Orders",
	}, owner.ID)
	require.NoError(t, err)
	require.NotEqual(t, table.ID, recreated.ID)
}

func TestTableService_DeniesAccessWhenDatabaseDeleted(t *testing.T) {
	db := setupResourceTestDB(t)
	tableService := NewTableService(db)
	databaseService := NewDatabaseService(db)

	owner := createResourceUser(t, db, "table_owner_deleted_db")
	database := createResourceDatabase(t, db, owner.ID, "DeletedDB")

	table := createResourceTable(t, db, database.ID, "ActiveOrders")
	require.NoError(t, databaseService.DeleteDatabase(database.ID, owner.ID))

	_, err := tableService.ListTables(database.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")

	_, err = tableService.CreateTable(CreateTableRequest{
		DatabaseID: database.ID,
		Name:       "NewOrders",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")

	_, err = tableService.GetTable(table.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")
}

func TestTableService_UpdateTableRejectsDuplicateActiveName(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewTableService(db)

	owner := createResourceUser(t, db, "table_owner_duplicate")
	database := createResourceDatabase(t, db, owner.ID, "DuplicateDB")

	first := createResourceTable(t, db, database.ID, "Orders")
	second := createResourceTable(t, db, database.ID, "Customers")

	_, err := service.UpdateTable(second.ID, UpdateTableRequest{
		Name: "Orders",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "该数据库中已存在同名表")

	current, err := service.GetTable(second.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "Customers", current.Name)
	require.Equal(t, first.DatabaseID, current.DatabaseID)
}
