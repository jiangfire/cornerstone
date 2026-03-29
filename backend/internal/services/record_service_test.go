package services

import (
	"encoding/json"
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

func TestRecordService_BatchCreateHonorsFieldPermissions(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	fieldService := NewFieldService(db)

	owner := createResourceUser(t, db, "record_owner_batch_permissions")
	editor := createResourceUser(t, db, "record_editor_batch_permissions")

	database := createResourceDatabase(t, db, owner.ID, "RecordBatchPermissionDB")
	grantResourceDatabaseAccess(t, db, database.ID, editor.ID, "editor")

	table := createResourceTable(t, db, database.ID, "Orders")
	publicField := createResourceField(t, db, table.ID, "title", "string", true, "")
	secretField := createResourceField(t, db, table.ID, "secret", "string", false, "")

	require.NoError(t, fieldService.SetFieldPermission(table.ID, FieldPermissionRequest{
		FieldID:   secretField.ID,
		Role:      "editor",
		CanRead:   true,
		CanWrite:  false,
		CanDelete: false,
	}, owner.ID))

	_, err := recordService.BatchCreateRecords(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			publicField.Name: "公开标题",
			secretField.Name: "机密内容",
		},
	}, editor.ID, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无写入权限")

	records, err := recordService.BatchCreateRecords(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			publicField.Name: "公开标题",
			secretField.Name: "机密内容",
		},
	}, owner.ID, 2)
	require.NoError(t, err)
	require.Len(t, records, 2)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(records[0].Data), &payload))
	require.Equal(t, "机密内容", payload[secretField.Name])

	var stored []models.Record
	require.NoError(t, db.Where("table_id = ?", table.ID).Find(&stored).Error)
	require.Len(t, stored, 2)
}

func TestRecordService_ListRecordsAppliesPaginationAfterPermissionAwareFiltering(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	fieldService := NewFieldService(db)

	owner := createResourceUser(t, db, "record_owner_filtered_paging")
	viewer := createResourceUser(t, db, "record_viewer_filtered_paging")

	database := createResourceDatabase(t, db, owner.ID, "RecordFilterPagingDB")
	grantResourceDatabaseAccess(t, db, database.ID, viewer.ID, "viewer")

	table := createResourceTable(t, db, database.ID, "Orders")
	publicField := createResourceField(t, db, table.ID, "title", "string", true, "")
	secretField := createResourceField(t, db, table.ID, "secret", "string", false, "")

	require.NoError(t, fieldService.SetFieldPermission(table.ID, FieldPermissionRequest{
		FieldID:   secretField.ID,
		Role:      "viewer",
		CanRead:   false,
		CanWrite:  false,
		CanDelete: false,
	}, owner.ID))

	for _, title := range []string{"订单-A", "订单-A", "订单-B"} {
		_, err := recordService.CreateRecord(CreateRecordRequest{
			TableID: table.ID,
			Data: map[string]interface{}{
				publicField.Name: title,
				secretField.Name: "隐藏值-" + title,
			},
		}, owner.ID)
		require.NoError(t, err)
	}

	result, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   1,
		Offset:  1,
		Filter:  `{"title":"订单-A"}`,
	}, viewer.ID)
	require.NoError(t, err)
	require.Len(t, result.Records, 1)
	require.Equal(t, int64(2), result.Total)
	require.False(t, result.HasMore)

	payload, ok := result.Records[0].Data.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "订单-A", payload["title"])
	require.NotContains(t, payload, "secret")
}
