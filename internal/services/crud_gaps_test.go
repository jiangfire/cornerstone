package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
)

func createTestField(t *testing.T, db *gorm.DB, tableID, name, fieldType string) *models.Field {
	t.Helper()
	f := &models.Field{TableID: tableID, Name: name, Type: fieldType}
	require.NoError(t, db.Create(f).Error)
	return f
}

func createTestFile(t *testing.T, db *gorm.DB, recordID, fieldID string) *models.File {
	t.Helper()
	f := &models.File{
		RecordID:   recordID,
		FieldID:    fieldID,
		FileName:   "test.txt",
		FileSize:   100,
		FileType:   "text/plain",
		StorageURL: "./uploads/test.txt",
	}
	require.NoError(t, db.Create(f).Error)
	return f
}

func setupCrudTestEnv(t *testing.T) (*gorm.DB, *models.Database, *models.Table, *models.Token) {
	t.Helper()
	db := setupTestDB(t)
	database := &models.Database{Name: "CrudGapDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "gap_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_crud_gap", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)
	return db, database, table, master
}

// ============================================================
// Field: GetField with config
// ============================================================

func TestGetField_WithConfig(t *testing.T) {
	db, _, table, master := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	maxLen := 50
	config := FieldConfig{
		Required:  true,
		MaxLength: &maxLen,
		Options:   []string{"a", "b"},
	}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	field := &models.Field{
		TableID:  table.ID,
		Name:     "email",
		Type:     "string",
		Required: true,
		Options:  string(configJSON),
	}
	require.NoError(t, db.Create(field).Error)

	resp, err := svc.GetField(field.ID, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "email", resp.Name)
	assert.Equal(t, "string", resp.Type)
	assert.True(t, resp.Required)
	assert.Equal(t, "a, b", resp.Options)
	assert.NotNil(t, resp.Config.MaxLength)
	assert.Equal(t, 50, *resp.Config.MaxLength)
	assert.True(t, resp.Config.Required)
}

// ============================================================
// Field: GetField no access
// ============================================================

func TestGetField_NoAccess(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	field := createTestField(t, db, table.ID, "secret", "string")

	viewer := &models.Token{
		Name: "viewer_noaccess", Token: "cs_viewer_getfield", IsMaster: false,
		Scopes: `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	_, err := svc.GetField(field.ID, viewer.ID)
	assert.Error(t, err)
}

// ============================================================
// Field: UpdateField success with name+type change
// ============================================================

func TestUpdateField_NameAndTypeChange(t *testing.T) {
	db, _, table, master := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	field := createTestField(t, db, table.ID, "old_name", "string")

	updated, err := svc.UpdateField(field.ID, UpdateFieldRequest{
		Name: "new_name",
		Type: "text",
	}, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "new_name", updated.Name)
	assert.Equal(t, "text", updated.Type)
}

// ============================================================
// Field: UpdateField with config changes
// ============================================================

func TestUpdateField_ConfigChanges(t *testing.T) {
	db, _, table, master := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	field := createTestField(t, db, table.ID, "score", "number")

	cfgMin := 0.0
	cfgMax := 100.0
	updated, err := svc.UpdateField(field.ID, UpdateFieldRequest{
		Name: "score",
		Type: "number",
		Config: FieldConfig{
			Min: &cfgMin,
			Max: &cfgMax,
		},
	}, master.ID)
	require.NoError(t, err)

	var config FieldConfig
	require.NoError(t, json.Unmarshal([]byte(updated.Options), &config))
	assert.Equal(t, 0.0, *config.Min)
	assert.Equal(t, 100.0, *config.Max)
}

// ============================================================
// Field: UpdateField invalid name
// ============================================================

func TestUpdateField_InvalidName(t *testing.T) {
	db, _, table, master := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	field := createTestField(t, db, table.ID, "valid", "string")

	_, err := svc.UpdateField(field.ID, UpdateFieldRequest{
		Name: "",
		Type: "string",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段名称验证失败")
}

// ============================================================
// Field: UpdateField invalid type
// ============================================================

func TestUpdateField_InvalidType(t *testing.T) {
	db, _, table, master := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	field := createTestField(t, db, table.ID, "valid", "string")

	_, err := svc.UpdateField(field.ID, UpdateFieldRequest{
		Name: "valid",
		Type: "notatype",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段类型验证失败")
}

// ============================================================
// Field: getActiveField
// ============================================================

func TestGetActiveField_Existing(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	field := createTestField(t, db, table.ID, "active_field", "string")

	found, err := svc.getActiveField(field.ID)
	require.NoError(t, err)
	assert.Equal(t, "active_field", found.Name)
}

func TestGetActiveField_SoftDeleted(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	field := createTestField(t, db, table.ID, "deleted_field", "string")
	require.NoError(t, db.Delete(field).Error)

	_, err := svc.getActiveField(field.ID)
	assert.Error(t, err)
}

// ============================================================
// Table: UpdateTable no access
// ============================================================

func TestUpdateTable_NoAccess(t *testing.T) {
	db, database, _, _ := setupCrudTestEnv(t)
	svc := NewTableService(db)

	table := &models.Table{DatabaseID: database.ID, Name: "upd_noaccess"}
	require.NoError(t, db.Create(table).Error)

	viewer := &models.Token{
		Name: "viewer_upd", Token: "cs_viewer_upd_tbl", IsMaster: false,
		Scopes: `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	_, err := svc.UpdateTable(table.ID, UpdateTableRequest{
		Name: "new_name",
	}, viewer.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权修改该表")
}

// ============================================================
// Record: validateAttachmentFieldValue - valid single attachment
// ============================================================

func TestValidateAttachmentFieldValue_SingleFile(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "avatar", "file")
	file := createTestFile(t, db, "", fld.ID)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "avatar", Type: "file"},
		FieldConfig{},
		file.ID,
		"",
		"user1",
	)
	assert.NoError(t, err)
}

// ============================================================
// Record: validateAttachmentFieldValue - multiple files when not allowed
// ============================================================

func TestValidateAttachmentFieldValue_MultipleNotAllowed(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "avatar", "file")
	f1 := createTestFile(t, db, "", fld.ID)
	f2 := createTestFile(t, db, "", fld.ID)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "avatar", Type: "file"},
		FieldConfig{Multiple: false},
		[]string{f1.ID, f2.ID},
		"",
		"user1",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "单个文件")
}

// ============================================================
// Record: validateAttachmentFieldValue - duplicate file ID
// ============================================================

func TestValidateAttachmentFieldValue_DuplicateFileID(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")
	file := createTestFile(t, db, "", fld.ID)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{Multiple: true},
		[]string{file.ID, file.ID},
		"",
		"user1",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "附件ID重复")
}

// ============================================================
// Record: validateAttachmentFieldValue - file not accessible
// ============================================================

func TestValidateAttachmentFieldValue_FileNotAccessible(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{Multiple: true},
		[]string{"fil_nonexistent"},
		"",
		"user1",
	)
	assert.Error(t, err)
}

// ============================================================
// Record: validateAttachmentFieldValue - file bound to wrong field
// ============================================================

func TestValidateAttachmentFieldValue_WrongField(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	otherFld := createTestField(t, db, table.ID, "other_field", "string")
	fld := createTestField(t, db, table.ID, "doc", "file")

	file := createTestFile(t, db, "", otherFld.ID)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{Multiple: true},
		[]string{file.ID},
		"",
		"user1",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不属于当前字段")
}

// ============================================================
// Record: validateAttachmentFieldValue - file already bound to another record
// ============================================================

func TestValidateAttachmentFieldValue_BoundToOtherRecord(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	existingRecord := &models.Record{TableID: table.ID, Data: `{"doc":""}`}
	require.NoError(t, db.Create(existingRecord).Error)

	file := createTestFile(t, db, existingRecord.ID, fld.ID)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{Multiple: true},
		[]string{file.ID},
		"rec_other_record_id",
		"user1",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已绑定到其他记录")
}

// ============================================================
// Record: syncAttachmentBindings - binds new files
// ============================================================

func TestSyncAttachmentBindings_BindsNewFiles(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")
	file := createTestFile(t, db, "", fld.ID)

	record := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(record).Error)

	fields := []models.Field{{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"}}
	data := map[string]interface{}{"doc": []string{file.ID}}

	err := s.syncAttachmentBindings(db, record.ID, fields, data)
	require.NoError(t, err)

	var updated models.File
	require.NoError(t, db.Where("id = ?", file.ID).First(&updated).Error)
	assert.Equal(t, record.ID, updated.RecordID)
	assert.Equal(t, fld.ID, updated.FieldID)
}

// ============================================================
// Record: syncAttachmentBindings - unbinds removed files
// ============================================================

func TestSyncAttachmentBindings_UnbindsRemovedFiles(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	record := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(record).Error)

	file := createTestFile(t, db, record.ID, fld.ID)

	fields := []models.Field{{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"}}
	data := map[string]interface{}{"doc": []string{}}

	err := s.syncAttachmentBindings(db, record.ID, fields, data)
	require.NoError(t, err)

	var updated models.File
	require.NoError(t, db.Where("id = ?", file.ID).First(&updated).Error)
	assert.Equal(t, "", updated.RecordID)
}

// ============================================================
// Record: syncAttachmentBindings - skips non-attachment fields
// ============================================================

func TestSyncAttachmentBindings_SkipsNonAttachmentFields(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "name", "string")

	record := &models.Record{TableID: table.ID, Data: `{"name":"test"}`}
	require.NoError(t, db.Create(record).Error)

	fields := []models.Field{{ID: fld.ID, TableID: table.ID, Name: "name", Type: "string"}}
	data := map[string]interface{}{"name": "test"}

	err := s.syncAttachmentBindings(db, record.ID, fields, data)
	assert.NoError(t, err)
}

// ============================================================
// Record: attachmentFieldIDsFromData - field present
// ============================================================

func TestAttachmentFieldIDsFromData_FieldPresent(t *testing.T) {
	field := models.Field{Name: "avatar", Type: "file"}
	ids, err := attachmentFieldIDsFromData(field, map[string]interface{}{
		"avatar": []string{"fil_1", "fil_2"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"fil_1", "fil_2"}, ids)
}

// ============================================================
// Record: attachmentFieldIDsFromData - field absent
// ============================================================

func TestAttachmentFieldIDsFromData_FieldAbsent(t *testing.T) {
	field := models.Field{Name: "avatar", Type: "file"}
	ids, err := attachmentFieldIDsFromData(field, map[string]interface{}{
		"name": "test",
	})
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// ============================================================
// Shared: validateFieldDescription - too long
// ============================================================

func TestValidateFieldDescription_TooLong(t *testing.T) {
	err := validateFieldDescription(strings.Repeat("x", 1001))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "1000")
}

// ============================================================
// Shared: validateFieldConfig MaxFileSizeMB validation
// ============================================================

func TestValidateFieldConfig_MaxFileSizeMB_Negative(t *testing.T) {
	err := validateFieldConfig(FieldConfig{MaxFileSizeMB: -5})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "附件大小限制不能小于0")
}

func TestValidateFieldConfig_MaxFileSizeMB_Zero(t *testing.T) {
	err := validateFieldConfig(FieldConfig{MaxFileSizeMB: 0})
	assert.NoError(t, err)
}

func TestValidateFieldConfig_MaxFileSizeMB_Positive(t *testing.T) {
	err := validateFieldConfig(FieldConfig{MaxFileSizeMB: 50})
	assert.NoError(t, err)
}

// ============================================================
// Record: validateAttachmentFieldValue - file size exceeds limit
// ============================================================

func TestValidateAttachmentFieldValue_FileSizeExceedsLimit(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	largeFile := &models.File{
		FieldID:    fld.ID,
		FileName:   "big.pdf",
		FileSize:   int64(200 * 1024 * 1024),
		FileType:   "application/pdf",
		StorageURL: "./uploads/big.pdf",
	}
	require.NoError(t, db.Create(largeFile).Error)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{MaxFileSizeMB: 1, Multiple: true},
		[]string{largeFile.ID},
		"",
		"user1",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "超过字段限制")
}

// ============================================================
// Record: validateAttachmentFieldValue - file type not allowed
// ============================================================

func TestValidateAttachmentFieldValue_FileTypeNotAllowed(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	file := &models.File{
		FieldID:    fld.ID,
		FileName:   "script.exe",
		FileSize:   100,
		FileType:   "application/x-msdownload",
		StorageURL: "./uploads/script.exe",
	}
	require.NoError(t, db.Create(file).Error)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{AllowedTypes: []string{".pdf", "image/*"}, Multiple: true},
		[]string{file.ID},
		"",
		"user1",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "类型不符合")
}

// ============================================================
// Record: validateAttachmentFieldValue - create with already bound file
// ============================================================

func TestValidateAttachmentFieldValue_CreateWithBoundFile(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	existingRecord := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(existingRecord).Error)

	file := createTestFile(t, db, existingRecord.ID, fld.ID)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{Multiple: true},
		[]string{file.ID},
		"",
		"user1",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未绑定记录")
}

// ============================================================
// Record: validateAttachmentFieldValue - same record update allowed
// ============================================================

func TestValidateAttachmentFieldValue_SameRecordUpdate(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	record := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(record).Error)

	file := createTestFile(t, db, record.ID, fld.ID)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{Multiple: true},
		[]string{file.ID},
		record.ID,
		"user1",
	)
	assert.NoError(t, err)
}

// ============================================================
// Record: validateAttachmentFieldValue - invalid value type
// ============================================================

func TestValidateAttachmentFieldValue_InvalidValueType(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{},
		42,
		"",
		"user1",
	)
	assert.Error(t, err)
}

// ============================================================
// Field: buildDeletedFieldName edge case
// ============================================================

func TestBuildDeletedFieldName_ExactBoundary(t *testing.T) {
	suffix := "__deleted__fld_123"
	name := strings.Repeat("y", 255-len(suffix))
	result := buildDeletedFieldName(name, "fld_123")
	assert.Len(t, result, 255)
}

func TestBuildDeletedFieldName_VeryLongID(t *testing.T) {
	longID := strings.Repeat("z", 300)
	result := buildDeletedFieldName("x", longID)
	assert.Contains(t, result, "__deleted__")
}

// ============================================================
// Table: buildDeletedTableName edge cases
// ============================================================

func TestBuildDeletedTableName_ExactBoundary(t *testing.T) {
	suffix := "__deleted__tbl_123"
	name := strings.Repeat("y", 255-len(suffix))
	result := buildDeletedTableName(name, "tbl_123")
	assert.Len(t, result, 255)
}

func TestBuildDeletedTableName_VeryLongID(t *testing.T) {
	longID := strings.Repeat("z", 300)
	result := buildDeletedTableName("x", longID)
	assert.Contains(t, result, "__deleted__")
}

// ============================================================
// Field: DeleteField with non-master viewer
// ============================================================

func TestDeleteField_NonMasterViewer(t *testing.T) {
	db, database, _, _ := setupCrudTestEnv(t)
	svc := NewFieldService(db)

	table := &models.Table{DatabaseID: database.ID, Name: "del_viewer_table"}
	require.NoError(t, db.Create(table).Error)

	field := createTestField(t, db, table.ID, "to_del", "string")

	viewer := &models.Token{
		Name: "viewer_del_fld", Token: "cs_viewer_del_fld", IsMaster: false,
		Scopes: fmt.Sprintf(`{"databases":{"%s":"viewer"},"tables":{}}`, database.ID),
	}
	require.NoError(t, db.Create(viewer).Error)

	err := svc.DeleteField(field.ID, viewer.ID)
	assert.Error(t, err)
}

// ============================================================
// Table: UpdateTable no access (viewer)
// ============================================================

func TestUpdateTable_ViewerNoManage(t *testing.T) {
	db, database, _, _ := setupCrudTestEnv(t)
	svc := NewTableService(db)

	table := &models.Table{DatabaseID: database.ID, Name: "upd_viewer_tbl"}
	require.NoError(t, db.Create(table).Error)

	viewer := &models.Token{
		Name: "viewer_tbl_upd", Token: "cs_viewer_tbl_upd", IsMaster: false,
		Scopes: fmt.Sprintf(`{"databases":{"%s":"viewer"},"tables":{}}`, database.ID),
	}
	require.NoError(t, db.Create(viewer).Error)

	_, err := svc.UpdateTable(table.ID, UpdateTableRequest{Name: "new_name"}, viewer.ID)
	assert.Error(t, err)
}

// ============================================================
// Record: attachmentFieldIDsFromData - single string value
// ============================================================

func TestAttachmentFieldIDsFromData_SingleString(t *testing.T) {
	field := models.Field{Name: "avatar", Type: "file"}
	ids, err := attachmentFieldIDsFromData(field, map[string]interface{}{
		"avatar": "fil_single",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"fil_single"}, ids)
}

// ============================================================
// Record: syncAttachmentBindings - rebinds existing file
// ============================================================

func TestSyncAttachmentBindings_RebindsFile(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	oldRecord := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(oldRecord).Error)

	file := createTestFile(t, db, oldRecord.ID, fld.ID)

	newRecord := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(newRecord).Error)

	fields := []models.Field{{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"}}
	data := map[string]interface{}{"doc": []string{file.ID}}

	err := s.syncAttachmentBindings(db, newRecord.ID, fields, data)
	require.NoError(t, err)

	var updated models.File
	require.NoError(t, db.Where("id = ?", file.ID).First(&updated).Error)
	assert.Equal(t, newRecord.ID, updated.RecordID)
}

// ============================================================
// Record: syncAttachmentBindings - keep existing binding
// ============================================================

func TestSyncAttachmentBindings_KeepsExistingBinding(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	record := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(record).Error)

	file := createTestFile(t, db, record.ID, fld.ID)

	fields := []models.Field{{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"}}
	data := map[string]interface{}{"doc": []string{file.ID}}

	err := s.syncAttachmentBindings(db, record.ID, fields, data)
	require.NoError(t, err)

	var updated models.File
	require.NoError(t, db.Where("id = ?", file.ID).First(&updated).Error)
	assert.Equal(t, record.ID, updated.RecordID)
	assert.Equal(t, fld.ID, updated.FieldID)
}

// ============================================================
// Record: validateAttachmentFieldValue - file size within limit
// ============================================================

func TestValidateAttachmentFieldValue_FileSizeWithinLimit(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	smallFile := &models.File{
		FieldID:    fld.ID,
		FileName:   "small.txt",
		FileSize:   50,
		FileType:   "text/plain",
		StorageURL: "./uploads/small.txt",
	}
	require.NoError(t, db.Create(smallFile).Error)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{MaxFileSizeMB: 1, Multiple: true},
		[]string{smallFile.ID},
		"",
		"user1",
	)
	assert.NoError(t, err)
}

// ============================================================
// Record: validateAttachmentFieldValue - allowed file type
// ============================================================

func TestValidateAttachmentFieldValue_AllowedFileType(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	pdfFile := &models.File{
		FieldID:    fld.ID,
		FileName:   "report.pdf",
		FileSize:   100,
		FileType:   "application/pdf",
		StorageURL: "./uploads/report.pdf",
	}
	require.NoError(t, db.Create(pdfFile).Error)

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{AllowedTypes: []string{".pdf"}, Multiple: true},
		[]string{pdfFile.ID},
		"",
		"user1",
	)
	assert.NoError(t, err)
}

// ============================================================
// Record: validateAttachmentFieldValue - empty file list
// ============================================================

func TestValidateAttachmentFieldValue_EmptyFileList(t *testing.T) {
	db, _, table, _ := setupCrudTestEnv(t)
	s := NewRecordService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")

	err := s.validateAttachmentFieldValue(
		models.Field{ID: fld.ID, TableID: table.ID, Name: "doc", Type: "file"},
		FieldConfig{},
		[]string{},
		"",
		"user1",
	)
	assert.NoError(t, err)
}
