package services

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
)

func setupGapTestEnv(t *testing.T) (*gorm.DB, *models.Database, *models.Table, *models.Token) {
	t.Helper()
	db := setupTestDB(t)
	database := &models.Database{Name: "GapTestDB"}
	require.NoError(t, db.Create(database).Error)
	table := &models.Table{DatabaseID: database.ID, Name: "gap_table"}
	require.NoError(t, db.Create(table).Error)
	master := &models.Token{Name: "master", Token: "cs_master_gap", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)
	return db, database, table, master
}

// ============================================================
// Database service
// ============================================================

func TestNewDatabaseService(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)
	assert.NotNil(t, svc)
}

func TestSanitizeDatabaseInput_TrimsName(t *testing.T) {
	name, desc := sanitizeDatabaseInput("  hello  ", "  desc  ")
	assert.Equal(t, "hello", name)
	assert.Equal(t, "desc", desc)
}

func TestSanitizeDatabaseInput_RemovesSpecialChars(t *testing.T) {
	name, desc := sanitizeDatabaseInput(`<script>"test"'`, `<b>"desc"'`)
	assert.Equal(t, "scripttest", name)
	assert.Equal(t, "bdesc", desc)
}

func TestCreateDatabase_Success(t *testing.T) {
	db, _, _, master := setupGapTestEnv(t)
	svc := NewDatabaseService(db)

	result, err := svc.CreateDatabase(CreateDBRequest{
		Name:        "new_db",
		Description: "test description",
	}, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "new_db", result.Name)
	assert.Equal(t, "test description", result.Description)
	assert.NotEmpty(t, result.ID)
}

func TestCreateDatabase_DuplicateName(t *testing.T) {
	db, _, _, master := setupGapTestEnv(t)
	svc := NewDatabaseService(db)

	_, err := svc.CreateDatabase(CreateDBRequest{Name: "dup_db"}, master.ID)
	require.NoError(t, err)

	_, err = svc.CreateDatabase(CreateDBRequest{Name: "dup_db"}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "同名数据库")
}

func TestCreateDatabase_EmptyName(t *testing.T) {
	db, _, _, master := setupGapTestEnv(t)
	svc := NewDatabaseService(db)

	_, err := svc.CreateDatabase(CreateDBRequest{Name: ""}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称验证失败")
}

func TestCreateDatabase_NameTooLong(t *testing.T) {
	db, _, _, master := setupGapTestEnv(t)
	svc := NewDatabaseService(db)

	longName := strings.Repeat("x", 256)
	_, err := svc.CreateDatabase(CreateDBRequest{Name: longName}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称验证失败")
}

// ============================================================
// File service
// ============================================================

func TestNewFileService_Gap(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)
	assert.NotNil(t, svc)
}

func TestGetMaxUploadSizeBytes_Gap(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)
	size := svc.getMaxUploadSizeBytes()
	assert.Equal(t, int64(50*1024*1024), size)
}

func TestNormalizeFileTypeToken_Gap(t *testing.T) {
	assert.Equal(t, "image/png", normalizeFileTypeToken(" Image/PNG "))
	assert.Equal(t, "text/plain", normalizeFileTypeToken("text/plain"))
	assert.Equal(t, "", normalizeFileTypeToken("  "))
}

func TestFileMatchesAllowedTypes_ExactMatch(t *testing.T) {
	assert.True(t, fileMatchesAllowedTypes("photo.png", "image/png", []string{"image/png"}))
}

func TestFileMatchesAllowedTypes_WildcardMatch(t *testing.T) {
	assert.True(t, fileMatchesAllowedTypes("photo.jpg", "image/jpeg", []string{"image/*"}))
}

func TestFileMatchesAllowedTypes_NoMatch(t *testing.T) {
	assert.False(t, fileMatchesAllowedTypes("doc.pdf", "application/pdf", []string{"image/*"}))
}

func TestFileMatchesAllowedTypes_EmptyAllowedTypes(t *testing.T) {
	assert.True(t, fileMatchesAllowedTypes("any.txt", "text/plain", []string{}))
	assert.True(t, fileMatchesAllowedTypes("any.txt", "text/plain", nil))
}

func TestGetFile_Success(t *testing.T) {
	db, _, table, _ := setupGapTestEnv(t)
	svc := NewFileService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")
	record := &models.Record{TableID: table.ID, Data: `{"doc":""}`}
	require.NoError(t, db.Create(record).Error)
	file := createTestFile(t, db, record.ID, fld.ID)

	result, err := svc.GetFile(file.ID, "user1")
	require.NoError(t, err)
	assert.Equal(t, file.ID, result.ID)
	assert.Equal(t, "test.txt", result.FileName)
}

func TestGetFile_NotFound(t *testing.T) {
	db, _, _, _ := setupGapTestEnv(t)
	svc := NewFileService(db)

	_, err := svc.GetFile("fil_nonexistent", "user1")
	assert.Error(t, err)
}

func TestDeleteFile_Success(t *testing.T) {
	db, _, table, _ := setupGapTestEnv(t)
	svc := NewFileService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")
	data := map[string]interface{}{"doc": []string{}}
	dataJSON, _ := json.Marshal(data)
	record := &models.Record{TableID: table.ID, Data: string(dataJSON)}
	require.NoError(t, db.Create(record).Error)

	tmpFile := "./uploads/test_delete_gap.txt"
	require.NoError(t, os.MkdirAll("./uploads", 0o750))
	require.NoError(t, os.WriteFile(tmpFile, []byte("test"), 0o644))

	file := &models.File{
		RecordID:   record.ID,
		FieldID:    fld.ID,
		FileName:   "test_delete_gap.txt",
		FileSize:   4,
		FileType:   "text/plain",
		StorageURL: tmpFile,
	}
	require.NoError(t, db.Create(file).Error)

	err := svc.DeleteFile(file.ID, "user1")
	require.NoError(t, err)

	var count int64
	db.Model(&models.File{}).Where("id = ?", file.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	_, statErr := os.Stat(tmpFile)
	assert.True(t, os.IsNotExist(statErr))
}

func TestDeleteFile_NotFound(t *testing.T) {
	db, _, _, _ := setupGapTestEnv(t)
	svc := NewFileService(db)

	err := svc.DeleteFile("fil_nonexistent", "user1")
	assert.Error(t, err)
}

func TestRemoveFileReferenceFromRecord(t *testing.T) {
	db, _, table, _ := setupGapTestEnv(t)
	svc := NewFileService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")
	file := createTestFile(t, db, "", fld.ID)

	data := map[string]interface{}{"doc": []string{file.ID}}
	dataJSON, _ := json.Marshal(data)
	record := &models.Record{TableID: table.ID, Data: string(dataJSON)}
	require.NoError(t, db.Create(record).Error)

	file.RecordID = record.ID
	require.NoError(t, db.Save(file).Error)

	err := svc.removeFileReferenceFromRecord(&models.File{
		ID:       file.ID,
		RecordID: record.ID,
		FieldID:  fld.ID,
	})
	require.NoError(t, err)

	var updated models.Record
	require.NoError(t, db.Where("id = ?", record.ID).First(&updated).Error)
	payload := parseRecordPayload(updated.Data)
	arr, ok := payload["doc"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, arr)
}

func TestGetAccessibleFile_Found(t *testing.T) {
	db, _, table, _ := setupGapTestEnv(t)
	svc := NewFileService(db)

	fld := createTestField(t, db, table.ID, "doc", "file")
	record := &models.Record{TableID: table.ID, Data: `{}`}
	require.NoError(t, db.Create(record).Error)
	file := createTestFile(t, db, record.ID, fld.ID)

	f, rec, err := svc.getAccessibleFile(file.ID, "user1", []string{"owner", "admin", "editor", "viewer"})
	require.NoError(t, err)
	assert.Equal(t, file.ID, f.ID)
	assert.NotNil(t, rec)
}

func TestGetAccessibleFile_NotFound(t *testing.T) {
	db, _, _, _ := setupGapTestEnv(t)
	svc := NewFileService(db)

	_, _, err := svc.getAccessibleFile("fil_nonexistent", "user1", []string{"owner"})
	assert.Error(t, err)
}

// ============================================================
// Record helpers
// ============================================================

func TestNewRecordService(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)
	assert.NotNil(t, svc)
}

func TestParseStringListValue_CommaSeparated(t *testing.T) {
	result, err := parseStringListValue([]string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestParseStringListValue_SingleValue(t *testing.T) {
	result, err := parseStringListValue([]string{"only"})
	require.NoError(t, err)
	assert.Equal(t, []string{"only"}, result)
}

func TestParseStringListValue_InterfaceSlice(t *testing.T) {
	result, err := parseStringListValue([]interface{}{"x", "y"})
	require.NoError(t, err)
	assert.Equal(t, []string{"x", "y"}, result)
}

func TestParseStringListValue_InvalidType(t *testing.T) {
	_, err := parseStringListValue("not_a_list")
	assert.Error(t, err)
}

func TestValidateFieldValue_StringType(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "string"}, "hello")
	assert.NoError(t, err)
}

func TestValidateFieldValue_StringType_Invalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "string"}, 123)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字符串类型")
}

func TestValidateFieldValue_NumberType(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "number"}, float64(42.5))
	assert.NoError(t, err)
}

func TestValidateFieldValue_NumberType_Invalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "number"}, "not_a_number")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数字类型")
}

func TestValidateFieldValue_BooleanType_Gap(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "boolean"}, true)
	assert.NoError(t, err)
}

func TestValidateFieldValue_BooleanType_Invalid_Gap(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "boolean"}, "true")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "布尔类型")
}

func TestValidateFieldValue_DateType(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "date"}, "2024-01-15")
	assert.NoError(t, err)
}

func TestValidateFieldValue_DateType_Invalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "date"}, "not-a-date")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "日期格式")
}

func TestValidateFieldValue_DateType_NotString(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "date"}, 123)
	assert.Error(t, err)
}

func TestValidateFieldValue_DateTimeType(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "datetime"}, "2024-01-15T10:30:00Z")
	assert.NoError(t, err)
}

func TestValidateFieldValue_DateTimeType_Invalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "datetime"}, "not-datetime")
	assert.Error(t, err)
}

func TestValidateFieldValue_ListType_Gap(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "list"}, []string{"a", "b"})
	assert.NoError(t, err)
}

func TestValidateFieldValue_ListType_Invalid_Gap(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "list"}, "single")
	assert.Error(t, err)
}

func TestValidateFieldValue_AttachmentType(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "file"}, "fil_123")
	assert.NoError(t, err)
}

func TestValidateFieldValue_NilValue(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{Type: "string"}, nil)
	assert.NoError(t, err)
}

func TestValidateFieldValue_StringWithMaxLength(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	maxLen := 5
	err := svc.validateFieldValue(models.Field{
		Type:    "string",
		Options: fmt.Sprintf(`{"max_length":%d}`, maxLen),
	}, "toolong")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "长度不能超过")
}

func TestValidateFieldValue_StringWithRegex(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{
		Type:    "string",
		Options: `{"validation":"^\\d+$"}`,
	}, "abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "格式不匹配")
}

func TestValidateFieldValue_StringRegexMatch(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	err := svc.validateFieldValue(models.Field{
		Type:    "string",
		Options: `{"validation":"^\\d+$"}`,
	}, "123")
	assert.NoError(t, err)
}

func TestValidateFieldValue_EmptyString(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	maxLen := 5
	err := svc.validateFieldValue(models.Field{
		Type:    "string",
		Options: fmt.Sprintf(`{"max_length":%d}`, maxLen),
	}, "")
	assert.NoError(t, err)
}

func TestExtractKnownRecordData_FiltersUnknownFields(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	fields := []models.Field{
		{ID: "fld_1", Name: "name"},
		{ID: "fld_2", Name: "age"},
	}
	data := map[string]interface{}{
		"name":   "Alice",
		"fld_2":  30,
		"unknown": "value",
	}

	normalized, matched := svc.extractKnownRecordData(fields, data)
	assert.Equal(t, "Alice", normalized["name"])
	assert.Equal(t, 30, normalized["age"])
	_, hasUnknown := normalized["unknown"]
	assert.False(t, hasUnknown)
	_, hasName := matched["name"]
	assert.True(t, hasName)
	_, hasFld2 := matched["fld_2"]
	assert.True(t, hasFld2)
	_, hasUnknownKey := matched["unknown"]
	assert.False(t, hasUnknownKey)
}

func TestNormalizeRecordData_TrimsStrings(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	fields := []models.Field{
		{ID: "fld_1", Name: "name"},
	}
	data := map[string]interface{}{
		"name": "Alice",
	}

	result, err := svc.normalizeRecordData(fields, data)
	require.NoError(t, err)
	assert.Equal(t, "Alice", result["name"])
}

func TestNormalizeRecordData_UnknownField(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	fields := []models.Field{
		{ID: "fld_1", Name: "name"},
	}
	data := map[string]interface{}{
		"name":    "Alice",
		"unknown": "value",
	}

	_, err := svc.normalizeRecordData(fields, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestEnsureWritableFields_BlocksReadOnlyFields(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	writable := map[string]models.Field{
		"name": {Name: "name"},
	}
	data := map[string]interface{}{
		"name":  "Alice",
		"admin": "should fail",
	}

	err := svc.ensureWritableFields(data, writable)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无写入权限")
}

func TestEnsureWritableFields_AllWritable(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	writable := map[string]models.Field{
		"name": {Name: "name"},
		"age":  {Name: "age"},
	}
	data := map[string]interface{}{
		"name": "Alice",
		"age":  30,
	}

	err := svc.ensureWritableFields(data, writable)
	assert.NoError(t, err)
}

func TestParseRecordPayload_Gap(t *testing.T) {
	payload := parseRecordPayload(`{"name":"Alice","age":30}`)
	assert.Equal(t, "Alice", payload["name"])
	assert.Equal(t, float64(30), payload["age"])
}

func TestParseRecordPayload_Empty_Gap(t *testing.T) {
	payload := parseRecordPayload("")
	assert.Empty(t, payload)
}

func TestParseRecordPayload_InvalidJSON_Gap(t *testing.T) {
	payload := parseRecordPayload("not json")
	assert.Empty(t, payload)
}

func TestFilterReadableData_RespectsPermissions(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	fields := []models.Field{
		{Name: "name"},
		{Name: "secret"},
	}
	readable := map[string]models.Field{
		"name": {Name: "name"},
	}
	payload := map[string]interface{}{
		"name":   "Alice",
		"secret": "hidden",
	}

	filtered := svc.filterReadableData(fields, readable, payload)
	assert.Equal(t, "Alice", filtered["name"])
	_, hasSecret := filtered["secret"]
	assert.False(t, hasSecret)
}

func TestFilterReadableData_ByFieldID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRecordService(db)

	fld := models.Field{ID: "fld_1", Name: "name"}
	fields := []models.Field{fld}
	readable := map[string]models.Field{"name": fld}
	payload := map[string]interface{}{
		"fld_1": "Alice",
	}

	filtered := svc.filterReadableData(fields, readable, payload)
	assert.Equal(t, "Alice", filtered["name"])
}

func TestTryParseStructuredFilter_JSONObject(t *testing.T) {
	result, ok := tryParseStructuredFilter(`{"name":"Alice"}`)
	assert.True(t, ok)
	assert.Equal(t, "Alice", result["name"])
}

func TestTryParseStructuredFilter_PlainString(t *testing.T) {
	result, ok := tryParseStructuredFilter("just a keyword")
	assert.Nil(t, result)
	assert.False(t, ok)
}

func TestTryParseStructuredFilter_Empty(t *testing.T) {
	result, ok := tryParseStructuredFilter("")
	assert.Nil(t, result)
	assert.False(t, ok)
}

func TestTryParseStructuredFilter_EmptyObject(t *testing.T) {
	result, ok := tryParseStructuredFilter("{}")
	assert.Nil(t, result)
	assert.False(t, ok)
}

// ============================================================
// Token service
// ============================================================

func TestNewTokenService(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTokenService(db)
	assert.NotNil(t, svc)
}

// ============================================================
// Cache
// ============================================================

func TestInvalidateFieldCache(t *testing.T) {
	SharedFieldCache.Set("test_table_id", []models.Field{{Name: "test"}})
	_, ok := SharedFieldCache.Get("test_table_id")
	assert.True(t, ok)

	InvalidateFieldCache("test_table_id")

	_, ok = SharedFieldCache.Get("test_table_id")
	assert.False(t, ok)
}
