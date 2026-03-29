package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestRecordService_EnforcesFieldPermissionsAcrossReadWriteAndExport(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	fieldService := NewFieldService(db)

	owner := createResourceUser(t, db, "record_owner_permissions")
	viewer := createResourceUser(t, db, "record_viewer_permissions")
	editor := createResourceUser(t, db, "record_editor_permissions")

	database := createResourceDatabase(t, db, owner.ID, "RecordPermissionDB")
	grantResourceDatabaseAccess(t, db, database.ID, viewer.ID, "viewer")
	grantResourceDatabaseAccess(t, db, database.ID, editor.ID, "editor")

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
	require.NoError(t, fieldService.SetFieldPermission(table.ID, FieldPermissionRequest{
		FieldID:   secretField.ID,
		Role:      "editor",
		CanRead:   true,
		CanWrite:  false,
		CanDelete: false,
	}, owner.ID))

	record, err := recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			publicField.Name: "公开标题",
			secretField.Name: "机密内容",
		},
	}, owner.ID)
	require.NoError(t, err)

	var createPayload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(record.Data), &createPayload))
	require.Equal(t, "机密内容", createPayload[secretField.Name])

	listResult, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   20,
	}, viewer.ID)
	require.NoError(t, err)
	require.Len(t, listResult.Records, 1)
	listPayload, ok := listResult.Records[0].Data.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "公开标题", listPayload[publicField.Name])
	require.NotContains(t, listPayload, secretField.Name)

	getResult, err := recordService.GetRecord(record.ID, viewer.ID)
	require.NoError(t, err)
	getPayload, ok := getResult.Data.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "公开标题", getPayload[publicField.Name])
	require.NotContains(t, getPayload, secretField.Name)

	csvData, _, _, err := recordService.ExportRecords(table.ID, viewer.ID, "csv", "")
	require.NoError(t, err)
	require.Contains(t, string(csvData), publicField.Name)
	require.NotContains(t, string(csvData), secretField.Name)

	jsonData, _, _, err := recordService.ExportRecords(table.ID, viewer.ID, "json", "")
	require.NoError(t, err)
	require.NotContains(t, string(jsonData), secretField.Name)

	_, err = recordService.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]interface{}{
			secretField.Name: "被拒绝的修改",
		},
	}, editor.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无写入权限")

	updatedRecord, err := recordService.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]interface{}{
			publicField.Name: "公开标题-已更新",
		},
	}, editor.ID)
	require.NoError(t, err)

	var updatedPayload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(updatedRecord.Data), &updatedPayload))
	require.Equal(t, "公开标题-已更新", updatedPayload[publicField.Name])
	require.Contains(t, updatedPayload, secretField.Name)

	var stored models.Record
	require.NoError(t, db.Where("id = ?", record.ID).First(&stored).Error)

	var storedPayload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stored.Data), &storedPayload))
	require.Equal(t, "公开标题-已更新", storedPayload[publicField.Name])
	require.Equal(t, "机密内容", storedPayload[secretField.Name])
}

func TestRecordService_FilterCannotProbeHiddenFieldValues(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	fieldService := NewFieldService(db)

	owner := createResourceUser(t, db, "record_owner_filter_probe")
	viewer := createResourceUser(t, db, "record_viewer_filter_probe")

	database := createResourceDatabase(t, db, owner.ID, "RecordFilterPermissionDB")
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

	_, err := recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			publicField.Name: "公开标题-A",
			secretField.Name: "机密-A",
		},
	}, owner.ID)
	require.NoError(t, err)

	_, err = recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			publicField.Name: "公开标题-B",
			secretField.Name: "机密-B",
		},
	}, owner.ID)
	require.NoError(t, err)

	visibleFilterResult, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   20,
		Filter:  fmt.Sprintf(`{"%s":"公开标题-A"}`, publicField.ID),
	}, viewer.ID)
	require.NoError(t, err)
	require.Len(t, visibleFilterResult.Records, 1)
	require.Equal(t, int64(1), visibleFilterResult.Total)

	hiddenStructuredFilterResult, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   20,
		Filter:  fmt.Sprintf(`{"%s":"机密-A"}`, secretField.ID),
	}, viewer.ID)
	require.NoError(t, err)
	require.Empty(t, hiddenStructuredFilterResult.Records)
	require.Zero(t, hiddenStructuredFilterResult.Total)

	hiddenKeywordFilterResult, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   20,
		Filter:  "机密-A",
	}, viewer.ID)
	require.NoError(t, err)
	require.Empty(t, hiddenKeywordFilterResult.Records)
	require.Zero(t, hiddenKeywordFilterResult.Total)

	jsonData, _, _, err := recordService.ExportRecords(table.ID, viewer.ID, "json", fmt.Sprintf(`{"%s":"机密-A"}`, secretField.ID))
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(jsonData))
}
