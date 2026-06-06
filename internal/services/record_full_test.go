package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
)

func createTestTableWithFields(t *testing.T, db_ *gorm.DB, userID, dbName, tableName string, fieldDefs ...struct {
	Name     string
	Type     string
	Required bool
}) (*models.Database, *models.Table, []*models.Field) {
	t.Helper()
	dbModel := &models.Database{Name: dbName}
	require.NoError(t, db_.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: tableName}
	require.NoError(t, db_.Create(tbl).Error)

	var fields []*models.Field
	for _, fd := range fieldDefs {
		f := &models.Field{TableID: tbl.ID, Name: fd.Name, Type: fd.Type, Required: fd.Required}
		require.NoError(t, db_.Create(f).Error)
		fields = append(fields, f)
	}
	authz.ClearTokenCache()
	return dbModel, tbl, fields
}

func TestCreateRecord_Success(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", true},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "hello"},
	}, "user1")

	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
	assert.Equal(t, tbl.ID, record.TableID)
	assert.Equal(t, 1, record.Version)

	var stored models.Record
	require.NoError(t, db.Where("id = ?", record.ID).First(&stored).Error)
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(stored.Data), &data))
	assert.Equal(t, "hello", data["name"])
}

func TestCreateRecord_TableNotExist(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: "tbl_nonexistent",
		Data:    map[string]any{"name": "test"},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "表不存在")
}

func TestCreateRecord_UnknownField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"title", "string", false},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"nonexistent_field": "value"},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestCreateRecord_RequiredFieldMissing(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"title", "string", true},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必填")
}

func TestCreateRecord_FieldByID(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, fields := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"title", "string", false},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{fields[0].ID: "via-id"},
	}, "user1")
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(record.Data), &data))
	assert.Equal(t, "via-id", data["title"])
}

func TestUpdateRecord_Success(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "original"},
	}, "user1")
	require.NoError(t, err)

	updated, err := s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]any{"name": "updated"},
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)

	var stored models.Record
	require.NoError(t, db.Where("id = ?", record.ID).First(&stored).Error)
	assert.Equal(t, 2, stored.Version)
}

func TestUpdateRecord_ReadOnlyFieldReject(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
		struct {
			Name     string
			Type     string
			Required bool
		}{"age", "number", false},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "test", "age": float64(10)},
	}, "user1")
	require.NoError(t, err)

	updated, err := s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]any{"age": float64(20)},
	}, "user1")
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(updated.Data), &data))
	assert.Equal(t, float64(20), data["age"])
}

func TestUpdateRecord_OptimisticLock(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	record, _ := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "original"},
	}, "user1")

	_, err := s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data:    map[string]any{"name": "v2"},
		Version: 999,
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "版本")
}

func TestUpdateRecord_MergePartial(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
		struct {
			Name     string
			Type     string
			Required bool
		}{"status", "string", false},
	)

	record, _ := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "alice", "status": "active"},
	}, "user1")

	updated, err := s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]any{"status": "inactive"},
	}, "user1")
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(updated.Data), &data))
	assert.Equal(t, "alice", data["name"])
	assert.Equal(t, "inactive", data["status"])
}

func TestListRecords_NoFilter(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	for i := 0; i < 5; i++ {
		_, err := s.CreateRecord(CreateRecordRequest{
			TableID: tbl.ID,
			Data:    map[string]any{"name": "item"},
		}, "user1")
		require.NoError(t, err)
	}

	result, err := s.ListRecords(QueryRequest{TableID: tbl.ID, Limit: 3}, "user1")
	require.NoError(t, err)
	assert.Equal(t, int64(5), result.Total)
	assert.Len(t, result.Records, 3)
	assert.True(t, result.HasMore)
}

func TestListRecords_StructuredFilter(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"status", "string", false},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"status": "active"},
	}, "user1")
	require.NoError(t, err)
	_, err = s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"status": "inactive"},
	}, "user1")
	require.NoError(t, err)

	filter := `{"status":"active"}`
	result, err := s.ListRecords(QueryRequest{
		TableID: tbl.ID, Limit: 10, Filter: filter,
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Records, 1)
}

func TestListRecords_StructuredFilterHiddenField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "test"},
	}, "user1")
	require.NoError(t, err)

	filter := `{"secret_field":"value"}`
	result, err := s.ListRecords(QueryRequest{
		TableID: tbl.ID, Limit: 10, Filter: filter,
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Empty(t, result.Records)
}

func TestListRecords_KeywordFilter(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "apple"},
	}, "user1")
	require.NoError(t, err)
	_, err = s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "banana"},
	}, "user1")
	require.NoError(t, err)

	result, err := s.ListRecords(QueryRequest{
		TableID: tbl.ID, Limit: 10, Filter: "apple",
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
}

func TestListRecords_Pagination(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	for i := 0; i < 25; i++ {
		_, err := s.CreateRecord(CreateRecordRequest{
			TableID: tbl.ID, Data: map[string]any{"name": "item"},
		}, "user1")
		require.NoError(t, err)
	}

	page1, err := s.ListRecords(QueryRequest{TableID: tbl.ID, Limit: 10, Offset: 0}, "user1")
	require.NoError(t, err)
	assert.Len(t, page1.Records, 10)
	assert.True(t, page1.HasMore)

	page2, err := s.ListRecords(QueryRequest{TableID: tbl.ID, Limit: 10, Offset: 10}, "user1")
	require.NoError(t, err)
	assert.Len(t, page2.Records, 10)
	assert.True(t, page2.HasMore)

	page3, err := s.ListRecords(QueryRequest{TableID: tbl.ID, Limit: 10, Offset: 20}, "user1")
	require.NoError(t, err)
	assert.Len(t, page3.Records, 5)
	assert.False(t, page3.HasMore)
}

func TestGetRecord_Success(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	created, _ := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "test"},
	}, "user1")

	found, err := s.GetRecord(created.ID, "user1")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	data, ok := found.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test", data["name"])
}

func TestGetRecord_NotFound(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, err := s.GetRecord("rec_nonexistent", "user1")
	require.Error(t, err)
}

func TestDeleteRecord_Success(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	record, _ := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "to-delete"},
	}, "user1")

	err := s.DeleteRecord(record.ID, "user1")
	require.NoError(t, err)

	var count int64
	db.Model(&models.Record{}).Where("id = ? AND deleted_at IS NULL", record.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDeleteRecord_SoftDelete(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	record, _ := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "soft"},
	}, "user1")

	err := s.DeleteRecord(record.ID, "user1")
	require.NoError(t, err)

	var deleted models.Record
	err = db.Unscoped().Where("id = ?", record.ID).First(&deleted).Error
	require.NoError(t, err)
	assert.NotNil(t, deleted.DeletedAt)
}

func TestDeleteRecord_NotFound(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	err := s.DeleteRecord("rec_nonexistent", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "记录不存在")
}

func TestExportRecords_CSV(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "alice"},
	}, "user1")
	s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "bob"},
	}, "user1")

	data, contentType, filename, err := s.ExportRecords(tbl.ID, "user1", "csv", "")
	require.NoError(t, err)
	assert.Contains(t, contentType, "text/csv")
	assert.Contains(t, filename, ".csv")
	assert.Contains(t, string(data), "name")
	assert.Contains(t, string(data), "alice")
	assert.Contains(t, string(data), "bob")
}

func TestExportRecords_JSON(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"name": "alice"},
	}, "user1")

	data, contentType, filename, err := s.ExportRecords(tbl.ID, "user1", "json", "")
	require.NoError(t, err)
	assert.Contains(t, contentType, "application/json")
	assert.Contains(t, filename, ".json")
	assert.Contains(t, string(data), "alice")
}

func TestExportRecords_UnsupportedFormat(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	_, _, _, err := s.ExportRecords(tbl.ID, "user1", "xml", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的导出格式")
}

func TestExportRecords_WithFilter(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"status", "string", false},
	)

	s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"status": "active"},
	}, "user1")
	s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID, Data: map[string]any{"status": "inactive"},
	}, "user1")

	data, _, _, err := s.ExportRecords(tbl.ID, "user1", "csv", `{"status":"active"}`)
	require.NoError(t, err)
	assert.Contains(t, string(data), "active")
	assert.NotContains(t, string(data), "inactive")
}

func TestExportRecords_TableNotExist(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, _, _, err := s.ExportRecords("tbl_nonexistent", "user1", "csv", "")
	require.Error(t, err)
}

func TestBatchCreateRecords_ParameterBounds(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	records, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "batch"},
	}, "user1", 1)
	require.NoError(t, err)
	assert.Len(t, records, 1)

	records, err = s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "batch"},
	}, "user1", 100)
	require.NoError(t, err)
	assert.Len(t, records, 100)
}

func TestBatchCreateRecords_TableNotExist(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: "tbl_nonexistent",
		Data:    map[string]any{"name": "x"},
	}, "user1", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "表不存在")
}

func TestValidateFieldValue_BooleanType(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "boolean"}
	assert.NoError(t, s.validateFieldValue(field, true))
	assert.NoError(t, s.validateFieldValue(field, false))
	assert.Error(t, s.validateFieldValue(field, "yes"))
}

func TestValidateFieldValue_ListType(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "list"}
	assert.NoError(t, s.validateFieldValue(field, []string{"a", "b"}))
	assert.NoError(t, s.validateFieldValue(field, []interface{}{"a", "b"}))
	assert.Error(t, s.validateFieldValue(field, "not-a-list"))
}

func TestValidateFieldValue_StringMaxLength(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	config := FieldConfig{MaxLength: intPtr(5)}
	field := models.Field{Type: "string", Options: marshalConfig(t, config)}

	assert.NoError(t, s.validateFieldValue(field, "hi"))
	assert.Error(t, s.validateFieldValue(field, "hello world"))
}

func TestValidateFieldValue_StringValidation(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	config := FieldConfig{Validation: `^\d+$`}
	field := models.Field{Type: "string", Options: marshalConfig(t, config)}

	assert.NoError(t, s.validateFieldValue(field, "123"))
	assert.Error(t, s.validateFieldValue(field, "abc"))
}

func TestParseStringListValue(t *testing.T) {
	result, err := parseStringListValue([]string{"a", "b"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, result)

	result, err = parseStringListValue([]interface{}{"x", "y"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"x", "y"}, result)

	_, err = parseStringListValue(42)
	assert.Error(t, err)
}

func TestParseAttachmentValue(t *testing.T) {
	result, err := parseAttachmentValue("file1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"file1"}, result)

	result, err = parseAttachmentValue([]string{"f1", "f2"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"f1", "f2"}, result)

	result, err = parseAttachmentValue("")
	assert.NoError(t, err)
	assert.Empty(t, result)

	_, err = parseAttachmentValue(42)
	assert.Error(t, err)
}

func TestTryParseStructuredFilter(t *testing.T) {
	m, ok := tryParseStructuredFilter(`{"name":"test"}`)
	assert.True(t, ok)
	assert.Equal(t, "test", m["name"])

	_, ok = tryParseStructuredFilter("just a keyword")
	assert.False(t, ok)

	_, ok = tryParseStructuredFilter("")
	assert.False(t, ok)

	_, ok = tryParseStructuredFilter("{}")
	assert.False(t, ok)
}

func TestNormalizeRecordData_UnknownKey(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{{Name: "a", Type: "string"}}
	_, err := s.normalizeRecordData(fields, map[string]any{"b": "val"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestNormalizeRecordData_KnownKeys(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{{Name: "a", Type: "string"}, {Name: "b", Type: "number"}}
	normalized, err := s.normalizeRecordData(fields, map[string]any{"a": "val", "b": float64(1)})
	require.NoError(t, err)
	assert.Equal(t, "val", normalized["a"])
	assert.Equal(t, float64(1), normalized["b"])
}

func TestEnsureWritableFields(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	writable := map[string]models.Field{"name": {Name: "name"}}
	assert.NoError(t, s.ensureWritableFields(map[string]any{"name": "x"}, writable))
	assert.Error(t, s.ensureWritableFields(map[string]any{"readonly_field": "x"}, writable))
}

func TestFilterReadableData(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{{Name: "a"}, {Name: "b"}}
	readable := map[string]models.Field{"a": {Name: "a"}}
	payload := map[string]any{"a": 1, "b": 2}

	filtered := s.filterReadableData(fields, readable, payload)
	assert.Equal(t, 1, filtered["a"])
	_, hasB := filtered["b"]
	assert.False(t, hasB)
}

func TestListRecords_DefaultLimit(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	for i := 0; i < 25; i++ {
		s.CreateRecord(CreateRecordRequest{
			TableID: tbl.ID, Data: map[string]any{"name": "item"},
		}, "user1")
	}

	result, err := s.ListRecords(QueryRequest{TableID: tbl.ID}, "user1")
	require.NoError(t, err)
	assert.Len(t, result.Records, 20)
	assert.Equal(t, int64(25), result.Total)
}

func TestGenerateTestData(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
		struct {
			Name     string
			Type     string
			Required bool
		}{"score", "number", false},
	)

	records, err := s.GenerateTestData(tbl.ID, "user1", 3)
	require.NoError(t, err)
	assert.Len(t, records, 3)

	records, err = s.GenerateTestData(tbl.ID, "user1", 0)
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestStringifyExportValue(t *testing.T) {
	assert.Equal(t, "hello", stringifyExportValue("hello"))
	assert.Equal(t, "42", stringifyExportValue(42))
	assert.Equal(t, "3.14", stringifyExportValue(3.14))
	assert.Equal(t, "", stringifyExportValue(nil))
	assert.Equal(t, "true", stringifyExportValue(true))
}

func TestJsonValuesEqual(t *testing.T) {
	assert.True(t, jsonValuesEqual("abc", "abc"))
	assert.True(t, jsonValuesEqual(float64(1), float64(1)))
	assert.True(t, jsonValuesEqual(map[string]any{"k": "v"}, map[string]any{"k": "v"}))
	assert.False(t, jsonValuesEqual("a", "b"))
}

func TestParseRecordPayload(t *testing.T) {
	assert.Equal(t, map[string]any{}, parseRecordPayload(""))
	assert.Equal(t, map[string]any{}, parseRecordPayload("not json"))

	result := parseRecordPayload(`{"name":"test"}`)
	assert.Equal(t, "test", result["name"])
}

func TestResolveReadableFilterField(t *testing.T) {
	fields := []models.Field{
		{ID: "fld_1", Name: "name"},
		{ID: "fld_2", Name: "status"},
	}
	readable := map[string]models.Field{"name": {Name: "name"}, "status": {Name: "status"}}

	name, ok := resolveReadableFilterField(fields, readable, "name")
	assert.True(t, ok)
	assert.Equal(t, "name", name)

	name, ok = resolveReadableFilterField(fields, readable, "fld_1")
	assert.True(t, ok)
	assert.Equal(t, "name", name)

	_, ok = resolveReadableFilterField(fields, readable, "hidden")
	assert.False(t, ok)
}

func TestListRecords_KeywordFilter_ExceedsLimit(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "TestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	origMax := maxKeywordScanRecords
	maxKeywordScanRecords = 5
	defer func() { maxKeywordScanRecords = origMax }()

	for i := 0; i < 6; i++ {
		s.CreateRecord(CreateRecordRequest{
			TableID: tbl.ID, Data: map[string]any{"name": strings.Repeat("test", 10)},
		}, "user1")
	}

	_, err := s.ListRecords(QueryRequest{
		TableID: tbl.ID, Limit: 10, Filter: "test",
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "关键字过滤匹配过多记录")
}

func marshalConfig(t *testing.T, config FieldConfig) string {
	t.Helper()
	data, err := json.Marshal(config)
	require.NoError(t, err)
	return string(data)
}

// ============================================================
// 从 record_gaps_test.go 合并：附件解析边界
// ============================================================

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

// ============================================================
// 从 record_gaps_test.go 合并：表访问权限检查
// ============================================================

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

func TestCheckTableAccess_ViewerAllowedForRead(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "viewer_access_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "viewer_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "title", Type: "string"}).Error)

	viewer := &models.Token{
		Name:     "viewer_can_read",
		IsMaster: false,
		Scopes:   fmt.Sprintf(`{"databases":{"%s":"viewer"},"tables":{"%s":{"role":"viewer"}}}`, dbModel.ID, tbl.ID),
	}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	err := s.checkTableAccess(tbl.ID, viewer.ID, []string{"owner", "admin", "editor", "viewer"})
	require.NoError(t, err)
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

// ============================================================
// 从 record_gaps_test.go 合并：创建记录权限与附件边界
// ============================================================

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

// ============================================================
// 从 record_gaps_test.go 合并：更新记录边界
// ============================================================

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

// ============================================================
// 从 record_gaps_test.go 合并：列表查询边界
// ============================================================

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

// ============================================================
// 从 record_gaps_test.go 合并：删除记录权限
// ============================================================

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

// ============================================================
// 从 record_gaps_test.go 合并：导出空记录边界
// ============================================================

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
