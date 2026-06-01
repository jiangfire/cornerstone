package services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
)

func TestParseAttachmentValue_InterfaceSliceNonString(t *testing.T) {
	_, err := parseAttachmentValue([]interface{}{"ok", 123})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "附件值必须是文件ID")
}

func TestParseAttachmentValue_StringSliceEmptyTrimming(t *testing.T) {
	result, err := parseAttachmentValue([]string{"  ", "f1", "  f2  ", ""})
	require.NoError(t, err)
	assert.Equal(t, []string{"f1", "f2"}, result)
}

func TestParseAttachmentValue_InterfaceSliceEmptyTrimming(t *testing.T) {
	result, err := parseAttachmentValue([]interface{}{"  ", "f1", "  f2  "})
	require.NoError(t, err)
	assert.Equal(t, []string{"f1", "f2"}, result)
}

func TestParseAttachmentValue_InterfaceSliceAllEmpty(t *testing.T) {
	result, err := parseAttachmentValue([]interface{}{"  ", ""})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseAttachmentValue_StringSliceAllEmpty(t *testing.T) {
	result, err := parseAttachmentValue([]string{"  ", ""})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestCheckTableAccess_NonMasterTokenDenied(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "access_test_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "access_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "title", Type: "string"}).Error)

	viewer := &models.Token{Name: "viewer_noperm", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	err := s.checkTableAccess(tbl.ID, viewer.ID, []string{"owner", "admin", "editor"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该表")
}

func TestCheckTableAccess_DeletedDatabase(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "deleted_db_test"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "orphan_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "title", Type: "string"}).Error)

	require.NoError(t, db.Delete(dbModel).Error)
	authz.ClearTokenCache()

	err := s.checkTableAccess(tbl.ID, "user1", []string{"owner", "admin"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "数据库不存在")
}

func TestValidateRecordData_AttachmentFieldValidation(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "attach_val_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "attach_val_table"}
	require.NoError(t, db.Create(tbl).Error)

	fileField := &models.Field{
		TableID: tbl.ID,
		Name:    "doc",
		Type:    "file",
		Options: "{}",
	}
	require.NoError(t, db.Create(fileField).Error)

	err := s.validateRecordData(tbl.ID, map[string]interface{}{"doc": 12345}, "", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "doc")
}

func TestValidateRecordData_NilValueOnOptionalField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "nil_val_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "nil_val_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "optional_field",
		Type:    "string",
		Options: `{"max_length":5}`,
	}).Error)

	err := s.validateRecordData(tbl.ID, map[string]interface{}{"optional_field": nil}, "", "user1")
	assert.NoError(t, err)
}

func TestValidateRecordData_EmptyStringOnOptionalField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "empty_str_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "empty_str_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "opt_str",
		Type:    "string",
		Options: `{"max_length":3}`,
	}).Error)

	err := s.validateRecordData(tbl.ID, map[string]interface{}{"opt_str": ""}, "", "user1")
	assert.NoError(t, err)
}

func TestCreateRecord_NonMasterDeniedWritePermission(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "write_perm_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "write_perm_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	viewer := &models.Token{Name: "viewer_write_denied", IsMaster: false, Scopes: `{"databases":{},"tables":{}}`}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"name": "test"},
	}, viewer.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该表")
}

func TestCreateRecord_AttachmentFieldWithInvalidValue(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "attach_create_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "attach_create_table"}
	require.NoError(t, db.Create(tbl).Error)

	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "attachment",
		Type:    "file",
		Options: "{}",
	}).Error)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"attachment": []interface{}{"fil_nonexistent"}},
	}, "user1")
	require.Error(t, err)
}

func TestCreateRecord_AttachmentFieldEmptyOK(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "attach_empty_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "attach_empty_table"}
	require.NoError(t, db.Create(tbl).Error)

	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "attachment",
		Type:    "file",
		Options: "{}",
	}).Error)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"attachment": []string{}},
	}, "user1")
	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
}

func TestUpdateRecord_OptimisticLockSuccess(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "LockDB", "lock_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"name": "original"},
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, 1, record.Version)

	updated, err := s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data:    map[string]interface{}{"name": "updated_with_lock"},
		Version: 1,
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)

	var stored models.Record
	require.NoError(t, db.Where("id = ?", record.ID).First(&stored).Error)
	assert.Equal(t, 2, stored.Version)

	var data map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stored.Data), &data))
	assert.Equal(t, "updated_with_lock", data["name"])
}

func TestUpdateRecord_NonWritableField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "writedenied_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "writedenied_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	viewer := &models.Token{
		Name:     "viewer_update",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	record := &models.Record{TableID: tbl.ID, Data: `{"name":"original"}`, Version: 1}
	require.NoError(t, db.Create(record).Error)

	_, err := s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]interface{}{"name": "hacked"},
	}, viewer.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该表")
}

func TestListRecords_KeywordFilterOffsetBeyondResults(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "KwOffsetDB", "kw_offset_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]interface{}{"name": "apple"},
	}, "user1")
	require.NoError(t, err)
	_, err = s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]interface{}{"name": "banana"},
	}, "user1")
	require.NoError(t, err)

	result, err := s.ListRecords(QueryRequest{
		TableID: tbl.ID,
		Limit:   10,
		Offset:  100,
		Filter:  "apple",
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Empty(t, result.Records)
	assert.False(t, result.HasMore)
}

func TestBatchCreateRecords_AttachmentFieldWithFileIDs(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "batch_attach_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "batch_attach_table"}
	require.NoError(t, db.Create(tbl).Error)

	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "doc",
		Type:    "file",
		Options: "{}",
	}).Error)

	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"doc": []string{"fil_123"}},
	}, "user1", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "批量创建暂不支持 file 字段")
}

func TestBatchCreateRecords_RequiredFieldMissing(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "batch_req_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "batch_req_table"}
	require.NoError(t, db.Create(tbl).Error)

	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "title",
		Type:    "string",
		Required: true,
	}).Error)

	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{},
	}, "user1", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必填")
}

func TestBatchCreateRecords_NonWritableField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "batch_write_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "batch_write_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	viewer := &models.Token{
		Name:     "viewer_batch",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"name": "test"},
	}, viewer.ID, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该表")
}

func TestDeleteRecord_PermissionDenied(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "delete_perm_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "delete_perm_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	record := &models.Record{TableID: tbl.ID, Data: `{"name":"sensitive"}`, Version: 1}
	require.NoError(t, db.Create(record).Error)

	viewer := &models.Token{
		Name:     "viewer_delete",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	err := s.DeleteRecord(record.ID, viewer.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该表")
}

func TestExportRecords_NoRecords(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "ExportEmptyDB", "export_empty_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	data, contentType, filename, err := s.ExportRecords(tbl.ID, "user1", "csv", "")
	require.NoError(t, err)
	assert.Contains(t, contentType, "text/csv")
	assert.Contains(t, filename, ".csv")
	assert.Contains(t, string(data), "name")

	lines := 0
	for _, ch := range string(data) {
		if ch == '\n' {
			lines++
		}
	}
	assert.Equal(t, 1, lines)
}

func TestExportRecords_NoRecordsJSON(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "ExportEmptyJSONDB", "export_empty_json_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	data, contentType, _, err := s.ExportRecords(tbl.ID, "user1", "json", "")
	require.NoError(t, err)
	assert.Contains(t, contentType, "application/json")

	var rows []interface{}
	require.NoError(t, json.Unmarshal(data, &rows))
	assert.Empty(t, rows)
}

func TestBuildStructuredFilterClauses_MalformedJSON(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{{Name: "status", Type: "string"}}
	readable := map[string]models.Field{"status": {Name: "status"}}

	_, _, err := s.buildStructuredFilterClauses(fields, readable, map[string]interface{}{
		"status": json.Number("not-a-number"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "过滤值序列化失败")
}
