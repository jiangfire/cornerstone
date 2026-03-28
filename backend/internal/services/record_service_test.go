package services

import (
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestRecordService_SelectAndMultiselectValidationClosedLoop(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	fieldService := NewFieldService(db)

	owner := createResourceUser(t, db, "record_owner_selects")
	database := createResourceDatabase(t, db, owner.ID, "RecordSelectDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	_, err := fieldService.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "status",
		Type:    "select",
		Config: FieldConfig{
			Options: []string{"draft", "approved"},
		},
	}, owner.ID)
	require.NoError(t, err)

	_, err = fieldService.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "tags",
		Type:    "multiselect",
		Config: FieldConfig{
			Options: []string{"urgent", "finance", "ops"},
		},
	}, owner.ID)
	require.NoError(t, err)

	_, err = recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			"status": "draft",
			"tags":   []interface{}{"urgent", "ops"},
		},
	}, owner.ID)
	require.NoError(t, err)

	_, err = recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			"status": "invalid_status",
		},
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的选项值")

	_, err = recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			"status": "approved",
			"tags":   []interface{}{"urgent", "invalid_tag"},
		},
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的选项值")
}

func TestRecordService_DeleteRecordSoftDeleteAndHidesRecord(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewRecordService(db)

	owner := createResourceUser(t, db, "record_owner_delete")
	database := createResourceDatabase(t, db, owner.ID, "RecordDeleteDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	createResourceField(t, db, table.ID, "title", "string", true, "")

	record, err := service.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			"title": "first",
		},
	}, owner.ID)
	require.NoError(t, err)

	require.NoError(t, service.DeleteRecord(record.ID, owner.ID))

	var stored models.Record
	require.NoError(t, db.Where("id = ?", record.ID).First(&stored).Error)
	require.NotNil(t, stored.DeletedAt)
	require.Equal(t, owner.ID, stored.UpdatedBy)
	require.Equal(t, 2, stored.Version)

	_, err = service.GetRecord(record.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "记录不存在")

	listed, err := service.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   20,
	}, owner.ID)
	require.NoError(t, err)
	require.Empty(t, listed.Records)
	require.Zero(t, listed.Total)
}

func TestRecordService_DeniesAccessWhenTableDeleted(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	tableService := NewTableService(db)

	owner := createResourceUser(t, db, "record_owner_deleted_table")
	database := createResourceDatabase(t, db, owner.ID, "RecordDeletedTableDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	createResourceField(t, db, table.ID, "title", "string", true, "")

	record := createResourceRecord(t, db, table.ID, owner.ID, `{"title":"legacy"}`)
	require.NoError(t, tableService.DeleteTable(table.ID, owner.ID))

	_, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   20,
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "表不存在")

	_, err = recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			"title": "new",
		},
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "表不存在")

	_, err = recordService.GetRecord(record.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "表不存在")
}

func TestRecordService_DeniesAccessWhenDatabaseDeleted(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	databaseService := NewDatabaseService(db)

	owner := createResourceUser(t, db, "record_owner_deleted_db")
	database := createResourceDatabase(t, db, owner.ID, "RecordDeletedDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	createResourceField(t, db, table.ID, "title", "string", true, "")

	require.NoError(t, databaseService.DeleteDatabase(database.ID, owner.ID))

	_, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   20,
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")

	_, err = recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			"title": "new",
		},
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")
}
