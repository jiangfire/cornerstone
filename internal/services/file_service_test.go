package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
)

func createFileRecord(t *testing.T, db *gorm.DB, recordID, fieldID string) *models.File {
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

func createFullTestChain(t *testing.T, db *gorm.DB, userID string) (*models.Database, *models.Table, *models.Field, *models.Record) {
	t.Helper()
	dbModel := &models.Database{Name: "file_test_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "file_test_table"}
	require.NoError(t, db.Create(tbl).Error)
	fld := &models.Field{TableID: tbl.ID, Name: "attachment", Type: "file"}
	require.NoError(t, db.Create(fld).Error)
	rec := &models.Record{TableID: tbl.ID, Data: `{"attachment":""}`}
	require.NoError(t, db.Create(rec).Error)
	authz.ClearTokenCache()
	return dbModel, tbl, fld, rec
}

// --- Path security ---

func TestResolveSecureStoragePath_ValidPath(t *testing.T) {
	path, err := ResolveSecureStoragePath("./uploads/test.txt")
	require.NoError(t, err)

	rootAbs, _ := filepath.Abs("./uploads")
	expected, _ := filepath.Abs("./uploads/test.txt")
	assert.Equal(t, expected, path)
	assert.True(t, filepath.IsAbs(path))
	assert.Contains(t, path, rootAbs)
}

func TestResolveSecureStoragePath_EmptyPath(t *testing.T) {
	_, err := ResolveSecureStoragePath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "文件路径为空")
}

func TestResolveSecureStoragePath_EmptyPathWhitespace(t *testing.T) {
	_, err := ResolveSecureStoragePath("   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "文件路径为空")
}

func TestResolveSecureStoragePath_PathTraversal(t *testing.T) {
	_, err := ResolveSecureStoragePath("./uploads/../../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "非法的文件路径")
}

func TestResolveSecureStoragePath_AbsoluteOutsideUploads(t *testing.T) {
	_, err := ResolveSecureStoragePath("/etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "非法的文件路径")
}

// --- File type matching ---

func TestMatchAllowedFileType_ExtensionMatch(t *testing.T) {
	assert.True(t, matchAllowedFileType(".pdf", "report.pdf", "application/pdf"))
	assert.False(t, matchAllowedFileType(".pdf", "report.txt", "text/plain"))
}

func TestMatchAllowedFileType_WildcardMatch(t *testing.T) {
	assert.True(t, matchAllowedFileType("image/*", "photo.jpg", "image/jpeg"))
	assert.True(t, matchAllowedFileType("image/*", "icon.png", "image/png"))
	assert.False(t, matchAllowedFileType("image/*", "doc.pdf", "application/pdf"))
}

func TestMatchAllowedFileType_ExactMimeMatch(t *testing.T) {
	assert.True(t, matchAllowedFileType("text/plain", "readme.txt", "text/plain"))
	assert.False(t, matchAllowedFileType("text/plain", "readme.txt", "application/pdf"))
}

func TestMatchAllowedFileType_NoMatch(t *testing.T) {
	assert.False(t, matchAllowedFileType(".pdf", "photo.png", "image/png"))
	assert.False(t, matchAllowedFileType("application/json", "data.xml", "text/xml"))
}

func TestMatchAllowedFileType_EmptyAllowedType(t *testing.T) {
	assert.False(t, matchAllowedFileType("", "file.pdf", "application/pdf"))
	assert.False(t, matchAllowedFileType("  ", "file.pdf", "application/pdf"))
}

func TestFileMatchesAllowedTypes_EmptyListAllowsAll(t *testing.T) {
	assert.True(t, fileMatchesAllowedTypes("anything.dat", "application/octet-stream", nil))
	assert.True(t, fileMatchesAllowedTypes("anything.dat", "application/octet-stream", []string{}))
}

func TestFileMatchesAllowedTypes_MatchingAndNonMatching(t *testing.T) {
	types := []string{".pdf", "image/*", "text/plain"}
	assert.True(t, fileMatchesAllowedTypes("doc.pdf", "application/pdf", types))
	assert.True(t, fileMatchesAllowedTypes("photo.jpg", "image/jpeg", types))
	assert.True(t, fileMatchesAllowedTypes("readme.txt", "text/plain", types))
	assert.False(t, fileMatchesAllowedTypes("data.xml", "text/xml", types))
}

func TestNormalizeFileTypeToken(t *testing.T) {
	assert.Equal(t, "image/jpeg", normalizeFileTypeToken(" Image/JPEG "))
	assert.Equal(t, ".pdf", normalizeFileTypeToken(" .PDF "))
	assert.Equal(t, "text/plain", normalizeFileTypeToken("text/plain"))
	assert.Equal(t, "", normalizeFileTypeToken("   "))
}

// --- Attachment reference removal ---

func TestRemoveAttachmentReferenceValue_StringMatch(t *testing.T) {
	val, removed := removeAttachmentReferenceValue("fil_abc123", "fil_abc123")
	assert.True(t, removed)
	assert.Equal(t, "", val)
}

func TestRemoveAttachmentReferenceValue_StringNoMatch(t *testing.T) {
	val, removed := removeAttachmentReferenceValue("fil_abc123", "fil_other")
	assert.False(t, removed)
	assert.Equal(t, "fil_abc123", val)
}

func TestRemoveAttachmentReferenceValue_InterfaceSliceRemoval(t *testing.T) {
	input := []interface{}{"fil_a", "fil_b", "fil_c"}
	val, removed := removeAttachmentReferenceValue(input, "fil_b")
	assert.True(t, removed)
	assert.Equal(t, []interface{}{"fil_a", "fil_c"}, val)
}

func TestRemoveAttachmentReferenceValue_InterfaceSliceNoMatch(t *testing.T) {
	input := []interface{}{"fil_a", "fil_b"}
	val, removed := removeAttachmentReferenceValue(input, "fil_missing")
	assert.False(t, removed)
	assert.Equal(t, input, val)
}

func TestRemoveAttachmentReferenceValue_StringSliceRemoval(t *testing.T) {
	input := []string{"fil_a", "fil_b", "fil_c"}
	val, removed := removeAttachmentReferenceValue(input, "fil_b")
	assert.True(t, removed)
	assert.Equal(t, []string{"fil_a", "fil_c"}, val)
}

func TestRemoveAttachmentReferenceValue_StringSliceNoMatch(t *testing.T) {
	input := []string{"fil_a", "fil_b"}
	val, removed := removeAttachmentReferenceValue(input, "fil_missing")
	assert.False(t, removed)
	assert.Equal(t, input, val)
}

func TestRemoveAttachmentReferenceValue_OtherTypeReturnsFalse(t *testing.T) {
	val, removed := removeAttachmentReferenceValue(42, "fil_abc123")
	assert.False(t, removed)
	assert.Equal(t, 42, val)

	val2, removed2 := removeAttachmentReferenceValue(3.14, "fil_abc123")
	assert.False(t, removed2)
	assert.Equal(t, 3.14, val2)
}

// --- Field config parsing ---

func TestParseStoredFieldConfig_EmptyString(t *testing.T) {
	config := parseStoredFieldConfig("")
	assert.Equal(t, FieldConfig{}, config)
}

func TestParseStoredFieldConfig_ValidJSON(t *testing.T) {
	jsonStr := `{"allowed_types":[".pdf","image/*"],"max_file_size_mb":10,"multiple":true}`
	config := parseStoredFieldConfig(jsonStr)
	assert.Equal(t, []string{".pdf", "image/*"}, config.AllowedTypes)
	assert.Equal(t, 10, config.MaxFileSizeMB)
	assert.True(t, config.Multiple)
}

func TestParseStoredFieldConfig_InvalidJSON(t *testing.T) {
	config := parseStoredFieldConfig("{not valid json}")
	assert.Equal(t, FieldConfig{}, config)
}

// --- File service CRUD ---

func TestNewFileService(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)
	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
}

func TestGetFile_ExistsWithRecordAccess(t *testing.T) {
	db := setupTestDB(t)
	_, _, _, rec := createFullTestChain(t, db, "user1")
	svc := NewFileService(db)

	file := createFileRecord(t, db, rec.ID, "")

	found, err := svc.GetFile(file.ID, "user1")
	require.NoError(t, err)
	assert.Equal(t, file.ID, found.ID)
	assert.Equal(t, "test.txt", found.FileName)
}

func TestGetFile_NonexistentFile(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	_, err := svc.GetFile("fil_nonexistent", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "文件不存在")
}

func TestDeleteFile_NonexistentFile(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	err := svc.DeleteFile("fil_nonexistent", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "文件不存在")
}

func TestListRecordFiles_NonexistentRecord(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	_, err := svc.ListRecordFiles("rec_nonexistent", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "记录不存在")
}

func TestUploadFile_MissingBothRecordIDAndFieldID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	_, err := svc.UploadFile(UploadFileRequest{}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "记录ID或字段ID至少需要提供一个")
}

func TestGetAccessibleRecord_NonexistentRecord(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	_, err := svc.getAccessibleRecord("rec_nonexistent", "user1", []string{"owner"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "记录不存在")
}

func TestGetAccessibleField_NonexistentField(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	_, err := svc.getAccessibleField("fld_nonexistent", "user1", []string{"owner"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "字段不存在")
}

func TestGetAccessibleFile_NonexistentFile(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	_, _, err := svc.getAccessibleFile("fil_nonexistent", "user1", []string{"owner"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "文件不存在")
}

func TestGetAccessibleFile_FileWithoutRecordIDOrFieldID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	file := createFileRecord(t, db, "", "")
	_, _, err := svc.getAccessibleFile(file.ID, "user1", []string{"owner"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "文件缺少关联记录或字段")
}

func TestGetMaxUploadSizeBytes(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFileService(db)

	maxSize := svc.getMaxUploadSizeBytes()
	assert.Equal(t, int64(50*1024*1024), maxSize)
}
