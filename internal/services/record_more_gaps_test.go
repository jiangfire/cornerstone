package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
)

func TestUpdateRecord_UnknownField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "UpdUnkDB", "upd_unk_table",
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

	_, err = s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]interface{}{"nonexistent_field": "value"},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUpdateRecord_InvalidFieldType(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "UpdTypeDB", "upd_type_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"count", "number", false},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"count": float64(42)},
	}, "user1")
	require.NoError(t, err)

	_, err = s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]interface{}{"count": "not-a-number"},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected number type")
}

func TestGetRecord_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "GetDenyDB", "get_deny_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"name": "secret"},
	}, "user1")
	require.NoError(t, err)

	viewer := &models.Token{Name: "viewer_no_access", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	_, err = s.GetRecord(record.ID, viewer.ID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied: cannot access this table")
}

func TestListRecords_NonMasterDenied(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "ListDenyDB", "list_deny_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"name", "string", false},
	)

	viewer := &models.Token{Name: "viewer_list_denied", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	_, err := s.ListRecords(QueryRequest{
		TableID: tbl.ID,
		Limit:   10,
	}, viewer.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied: cannot access this table")
}

func TestExportRecords_ListFieldValues(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "ExpListDB", "exp_list_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"tags", "list", false},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"tags": []string{"a", "b", "c"}},
	}, "user1")
	require.NoError(t, err)

	data, contentType, filename, err := s.ExportRecords(tbl.ID, "user1", "csv", "")
	require.NoError(t, err)
	assert.Contains(t, contentType, "text/csv")
	assert.Contains(t, filename, ".csv")

	csvStr := string(data)
	assert.True(t, strings.Contains(csvStr, "a"), "CSV should contain list values, got: %s", csvStr)
}

func TestExportRecords_JSONFieldValues(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "ExpJsonDB", "exp_json_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"meta", "json", false},
	)

	_, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"meta": map[string]interface{}{"key": "val", "num": float64(1)}},
	}, "user1")
	require.NoError(t, err)

	data, contentType, _, err := s.ExportRecords(tbl.ID, "user1", "csv", "")
	require.NoError(t, err)
	assert.Contains(t, contentType, "text/csv")
	assert.Contains(t, string(data), "key")
	assert.Contains(t, string(data), "val")
}

func TestCreateRecord_WithFileFieldNonEmpty(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, fields := createTestTableWithFields(t, db, "user1", "FileBindDB", "file_bind_table",
		struct {
			Name     string
			Type     string
			Required bool
		}{"doc", "file", false},
	)

	fileField := fields[0]
	fileRecord := &models.File{
		FieldID:  fileField.ID,
		FileName: "test.pdf",
		FileSize: 1024,
		FileType: "application/pdf",
	}
	require.NoError(t, db.Create(fileRecord).Error)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"doc": []string{fileRecord.ID}},
	}, "user1")
	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)

	var updatedFile models.File
	require.NoError(t, db.Where("id = ?", fileRecord.ID).First(&updatedFile).Error)
	assert.Equal(t, record.ID, updatedFile.RecordID, "file should be bound to the new record")
}
