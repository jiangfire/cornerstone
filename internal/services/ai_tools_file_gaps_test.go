package services

import (
	"bytes"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func setupGapDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.SetupTestDBWithTokens(t, "user1")
}

func createGapMasterToken(t *testing.T, db *gorm.DB) *models.Token {
	t.Helper()
	authz.ClearTokenCache()
	tok := &models.Token{Name: "gap_master", Token: "cs_gap_master", IsMaster: true}
	require.NoError(t, db.Create(tok).Error)
	return tok
}

func createGapChain(t *testing.T, db *gorm.DB) (*models.Database, *models.Table, *models.Field, *models.Record) {
	t.Helper()
	dbModel := &models.Database{Name: "gap_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "gap_table"}
	require.NoError(t, db.Create(tbl).Error)
	fld := &models.Field{TableID: tbl.ID, Name: "name", Type: "string"}
	require.NoError(t, db.Create(fld).Error)
	rec := &models.Record{TableID: tbl.ID, Data: `{"name":"Alice"}`, Version: 1}
	require.NoError(t, db.Create(rec).Error)
	return dbModel, tbl, fld, rec
}

func createGapFileChain(t *testing.T, db *gorm.DB) (*models.Database, *models.Table, *models.Field, *models.Record) {
	t.Helper()
	dbModel := &models.Database{Name: "gap_file_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "gap_file_table"}
	require.NoError(t, db.Create(tbl).Error)
	fld := &models.Field{TableID: tbl.ID, Name: "attachment", Type: "file"}
	require.NoError(t, db.Create(fld).Error)
	rec := &models.Record{TableID: tbl.ID, Data: `{"attachment":""}`}
	require.NoError(t, db.Create(rec).Error)
	authz.ClearTokenCache()
	return dbModel, tbl, fld, rec
}

func TestGap_ExecuteGenerateTestData_Success(t *testing.T) {
	db := setupGapDB(t)
	_, tbl, _, _ := createGapChain(t, db)
	master := createGapMasterToken(t, db)

	result, err := ExecuteAIToolForToken(db, master.ID, "generate_test_data", map[string]any{
		"table_id": tbl.ID,
		"count":    float64(3),
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, tbl.ID, resMap["table_id"])
	assert.Equal(t, 3, resMap["inserted"])
}

func TestGap_ExecuteGenerateTestData_MissingTableID(t *testing.T) {
	db := setupGapDB(t)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "generate_test_data", map[string]any{
		"count": float64(1),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table_id required")
}

func TestGap_ExecuteGenerateTestData_MissingCount(t *testing.T) {
	db := setupGapDB(t)
	_, tbl, _, _ := createGapChain(t, db)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "generate_test_data", map[string]any{
		"table_id": tbl.ID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count required")
}

func TestGap_ExecuteDeleteRecord_MissingRecordID(t *testing.T) {
	db := setupGapDB(t)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "delete_record", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record_id required")
}

func TestGap_ExecuteDeleteRecord_NotFound(t *testing.T) {
	db := setupGapDB(t)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "delete_record", map[string]any{
		"record_id": "rec_nonexistent",
	})
	require.Error(t, err)
}

func TestGap_ExecuteUpdateRecord_MissingRecordID(t *testing.T) {
	db := setupGapDB(t)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "update_record", map[string]any{
		"data": map[string]any{"name": "Bob"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record_id required")
}

func TestGap_ExecuteUpdateRecord_MissingData(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapChain(t, db)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "update_record", map[string]any{
		"record_id": rec.ID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data required")
}

func TestGap_ExecuteCreateField_MissingTableID(t *testing.T) {
	db := setupGapDB(t)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "create_field", map[string]any{
		"name": "email",
		"type": "string",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table_id required")
}

func TestGap_ExecuteCreateField_MissingName(t *testing.T) {
	db := setupGapDB(t)
	_, tbl, _, _ := createGapChain(t, db)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "create_field", map[string]any{
		"table_id": tbl.ID,
		"type":     "string",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name required")
}

func TestGap_ExecuteCreateField_MissingType(t *testing.T) {
	db := setupGapDB(t)
	_, tbl, _, _ := createGapChain(t, db)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "create_field", map[string]any{
		"table_id": tbl.ID,
		"name":     "email",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type required")
}

func TestGap_ExecuteCreateField_SuccessWithDescription(t *testing.T) {
	db := setupGapDB(t)
	_, tbl, _, _ := createGapChain(t, db)
	master := createGapMasterToken(t, db)

	result, err := ExecuteAIToolForToken(db, master.ID, "create_field", map[string]any{
		"table_id":    tbl.ID,
		"name":        "email",
		"type":        "string",
		"description": "Email address",
		"required":    true,
	})
	require.NoError(t, err)

	resMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "email", resMap["name"])
	assert.Equal(t, "string", resMap["type"])
}

func TestGap_GenerateFieldValue_Text(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	val := generateFieldValue(rng, "text")
	assert.IsType(t, "", val)
}

func TestGap_GenerateFieldValue_JSON(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	val := generateFieldValue(rng, "json")
	_, ok := val.(map[string]any)
	assert.True(t, ok)
}

func TestGap_GenerateFieldValue_Datetime(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	val := generateFieldValue(rng, "datetime")
	s, ok := val.(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339, s)
	assert.NoError(t, err)
}

func TestGap_GenerateFieldValue_File(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	val := generateFieldValue(rng, "file")
	_, ok := val.([]string)
	assert.True(t, ok)
}

func TestGap_GenerateFieldValue_Default(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	val := generateFieldValue(rng, "unknown_type")
	assert.Equal(t, "sample_value", val)
}

func TestGap_ExecuteQuery_MissingFrom(t *testing.T) {
	db := setupGapDB(t)
	master := createGapMasterToken(t, db)

	_, err := ExecuteAIToolForToken(db, master.ID, "execute_query", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "from required")
}

func TestGap_ExecuteQuery_WithSelectWhereLimitOffset(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapChain(t, db)
	master := createGapMasterToken(t, db)

	result, err := ExecuteAIToolForToken(db, master.ID, "execute_query", map[string]any{
		"from":   "records",
		"select": []any{"id", "data"},
		"where":  map[string]any{"id": rec.ID},
		"limit":  float64(10),
		"offset": float64(0),
	})
	require.NoError(t, err)

	rows, ok := result.([]map[string]any)
	require.True(t, ok)
	assert.Len(t, rows, 1)
}

func makeMultipartFileHeader(t *testing.T, fieldName, fileName string, data []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	h, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = h.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest("POST", "/upload", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, fh, err := req.FormFile(fieldName)
	require.NoError(t, err)
	return fh
}

func TestGap_UploadFile_WithRecordID(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	fh := makeMultipartFileHeader(t, "file", "test.txt", []byte("hello world"))

	file, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		File:     fh,
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, "test.txt", file.FileName)
	assert.Equal(t, int64(11), file.FileSize)
	assert.Equal(t, rec.ID, file.RecordID)

	_ = os.Remove(file.StorageURL)
}

func TestGap_UploadFile_WithFieldID(t *testing.T) {
	db := setupGapDB(t)
	_, _, fld, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	fh := makeMultipartFileHeader(t, "file", "photo.jpg", []byte("fake jpeg data"))

	file, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		FieldID:  fld.ID,
		File:     fh,
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, "photo.jpg", file.FileName)
	assert.Equal(t, rec.ID, file.RecordID)
	assert.Equal(t, fld.ID, file.FieldID)

	_ = os.Remove(file.StorageURL)
}

func TestGap_UploadFile_FileTooLarge(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "text/plain")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		File: &multipart.FileHeader{
			Filename: "huge.txt",
			Header:   hdr,
			Size:     100 * 1024 * 1024,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file size exceeds limit")
}

func TestGap_UploadFile_EmptyFilename(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "text/plain")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		File: &multipart.FileHeader{
			Filename: "   ",
			Header:   hdr,
			Size:     10,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file name is required")
}

func TestGap_UploadFile_IllegalFilename(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "text/plain")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		File: &multipart.FileHeader{
			Filename: "../etc/passwd",
			Header:   hdr,
			Size:     10,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "illegal file name")
}

func TestGap_UploadFile_UnsupportedFileType(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "application/x-msdownload")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		File: &multipart.FileHeader{
			Filename: "malware.exe",
			Header:   hdr,
			Size:     100,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported file type")
}

func TestGap_UploadFile_FieldNotFileType(t *testing.T) {
	db := setupGapDB(t)
	dbModel := &models.Database{Name: "gap_nofile_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "gap_nofile_table"}
	require.NoError(t, db.Create(tbl).Error)
	fld := &models.Field{TableID: tbl.ID, Name: "notes", Type: "string"}
	require.NoError(t, db.Create(fld).Error)
	rec := &models.Record{TableID: tbl.ID, Data: `{"notes":"hello"}`}
	require.NoError(t, db.Create(rec).Error)
	authz.ClearTokenCache()

	svc := NewFileService(db)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "text/plain")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		FieldID:  fld.ID,
		File: &multipart.FileHeader{
			Filename: "test.txt",
			Header:   hdr,
			Size:     10,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only file type fields support file uploads")
}

func TestGap_UploadFile_FieldTableMismatch(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)

	dbModel2 := &models.Database{Name: "gap_other_db"}
	require.NoError(t, db.Create(dbModel2).Error)
	tbl2 := &models.Table{DatabaseID: dbModel2.ID, Name: "gap_other_table"}
	require.NoError(t, db.Create(tbl2).Error)
	fld2 := &models.Field{TableID: tbl2.ID, Name: "doc", Type: "file"}
	require.NoError(t, db.Create(fld2).Error)
	authz.ClearTokenCache()

	svc := NewFileService(db)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "text/plain")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		FieldID:  fld2.ID,
		File: &multipart.FileHeader{
			Filename: "test.txt",
			Header:   hdr,
			Size:     10,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field does not belong to the record's table")
}

func TestGap_UploadFile_FieldSizeExceeded(t *testing.T) {
	db := setupGapDB(t)
	_, tbl, _, rec := createGapFileChain(t, db)

	fld := &models.Field{
		TableID: tbl.ID,
		Name:    "doc",
		Type:    "file",
		Options: `{"max_file_size_mb":1,"allowed_types":[".txt"]}`,
	}
	require.NoError(t, db.Create(fld).Error)
	authz.ClearTokenCache()

	svc := NewFileService(db)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "text/plain")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		FieldID:  fld.ID,
		File: &multipart.FileHeader{
			Filename: "big.txt",
			Header:   hdr,
			Size:     5 * 1024 * 1024,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file size exceeds field limit")
}

func TestGap_UploadFile_FieldTypeNotAllowed(t *testing.T) {
	db := setupGapDB(t)
	_, tbl, _, rec := createGapFileChain(t, db)

	fld := &models.Field{
		TableID: tbl.ID,
		Name:    "doc",
		Type:    "file",
		Options: `{"max_file_size_mb":50,"allowed_types":[".pdf"]}`,
	}
	require.NoError(t, db.Create(fld).Error)
	authz.ClearTokenCache()

	svc := NewFileService(db)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Type", "text/plain")

	_, err := svc.UploadFile(UploadFileRequest{
		RecordID: rec.ID,
		FieldID:  fld.ID,
		File: &multipart.FileHeader{
			Filename: "notes.txt",
			Header:   hdr,
			Size:     100,
		},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file type does not match field restrictions")
}

func TestGap_ListRecordFiles_Success(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	createFileRecord(t, db, rec.ID, "")
	createFileRecord(t, db, rec.ID, "")

	files, err := svc.ListRecordFiles(rec.ID, "user1")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestGap_DeleteFile_WithPhysicalFile(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	dir := "./uploads"
	require.NoError(t, os.MkdirAll(dir, 0o750))
	tmpPath := filepath.Join(dir, "gap_delete_test.txt")
	require.NoError(t, os.WriteFile(tmpPath, []byte("hello"), 0o644))
	t.Cleanup(func() { _ = os.Remove(tmpPath) })

	file := &models.File{
		RecordID:   rec.ID,
		FileName:   "gap_delete_test.txt",
		FileSize:   5,
		FileType:   "text/plain",
		StorageURL: tmpPath,
	}
	require.NoError(t, db.Create(file).Error)

	err := svc.DeleteFile(file.ID, "user1")
	require.NoError(t, err)

	var count int64
	db.Model(&models.File{}).Where("id = ?", file.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	_, statErr := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestGap_RemoveFileRef_NilFile(t *testing.T) {
	db := setupGapDB(t)
	svc := NewFileService(db)
	err := svc.removeFileReferenceFromRecord(nil)
	assert.NoError(t, err)
}

func TestGap_RemoveFileRef_EmptyRecordID(t *testing.T) {
	db := setupGapDB(t)
	svc := NewFileService(db)
	err := svc.removeFileReferenceFromRecord(&models.File{RecordID: "", FieldID: "fld_abc"})
	assert.NoError(t, err)
}

func TestGap_RemoveFileRef_EmptyFieldID(t *testing.T) {
	db := setupGapDB(t)
	svc := NewFileService(db)
	err := svc.removeFileReferenceFromRecord(&models.File{RecordID: "rec_abc", FieldID: ""})
	assert.NoError(t, err)
}

func TestGap_RemoveFileRef_Success(t *testing.T) {
	db := setupGapDB(t)
	_, _, fld, rec := createGapFileChain(t, db)

	fileID := models.GenerateID("fil")
	rec.Data = models.JSONField(`{"attachment":"` + fileID + `"}`)
	require.NoError(t, db.Save(rec).Error)

	file := &models.File{
		RecordID:   rec.ID,
		FieldID:    fld.ID,
		FileName:   "test.txt",
		FileSize:   10,
		FileType:   "text/plain",
		StorageURL: "./uploads/test.txt",
	}
	require.NoError(t, db.Create(file).Error)
	file.ID = fileID
	require.NoError(t, db.Save(file).Error)

	svc := NewFileService(db)
	err := svc.removeFileReferenceFromRecord(file)
	require.NoError(t, err)

	var updated models.Record
	require.NoError(t, db.Where("id = ?", rec.ID).First(&updated).Error)
	assert.NotContains(t, updated.Data, fileID)
}

func TestGap_RemoveFileRef_DeletedRecord(t *testing.T) {
	db := setupGapDB(t)
	_, _, fld, _ := createGapFileChain(t, db)

	file := &models.File{
		RecordID:   "rec_nonexistent",
		FieldID:    fld.ID,
		FileName:   "test.txt",
		FileSize:   10,
		FileType:   "text/plain",
		StorageURL: "./uploads/test.txt",
	}
	require.NoError(t, db.Create(file).Error)

	svc := NewFileService(db)
	err := svc.removeFileReferenceFromRecord(file)
	assert.NoError(t, err)
}

func TestGap_GetAccessibleRecord_Found(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	found, err := svc.getAccessibleRecord(rec.ID, "user1", []string{"owner"})
	require.NoError(t, err)
	assert.Equal(t, rec.ID, found.ID)
}

func TestGap_GetAccessibleField_Found(t *testing.T) {
	db := setupGapDB(t)
	_, _, fld, _ := createGapFileChain(t, db)
	svc := NewFileService(db)

	found, err := svc.getAccessibleField(fld.ID, "user1", []string{"owner"})
	require.NoError(t, err)
	assert.Equal(t, fld.ID, found.ID)
}

func TestGap_GetAccessibleFile_WithRecordID(t *testing.T) {
	db := setupGapDB(t)
	_, _, _, rec := createGapFileChain(t, db)
	svc := NewFileService(db)

	file := createFileRecord(t, db, rec.ID, "")

	f, foundRec, err := svc.getAccessibleFile(file.ID, "user1", []string{"owner"})
	require.NoError(t, err)
	assert.Equal(t, file.ID, f.ID)
	assert.Equal(t, rec.ID, foundRec.ID)
}

func TestGap_GetAccessibleFile_WithFieldIDOnly(t *testing.T) {
	db := setupGapDB(t)
	_, _, fld, _ := createGapFileChain(t, db)
	svc := NewFileService(db)

	file := &models.File{
		FieldID:    fld.ID,
		FileName:   "test.txt",
		FileSize:   10,
		FileType:   "text/plain",
		StorageURL: "./uploads/test.txt",
	}
	require.NoError(t, db.Create(file).Error)

	f, foundRec, err := svc.getAccessibleFile(file.ID, "user1", []string{"owner"})
	require.NoError(t, err)
	assert.Equal(t, file.ID, f.ID)
	assert.Nil(t, foundRec)
}
