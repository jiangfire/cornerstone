package services

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func createTestFileHeader(t *testing.T, fieldName, fileName string, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/files/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(body.Len())+1024))

	files := req.MultipartForm.File[fieldName]
	require.Len(t, files, 1)
	return files[0]
}

func TestFileService_RejectsUnauthorizedAccessAndUsesConfiguredUploadLimit(t *testing.T) {
	db := setupResourceTestDB(t)
	fileService := NewFileService(db)

	owner := createResourceUser(t, db, "file_owner")
	viewer := createResourceUser(t, db, "file_viewer")
	outsider := createResourceUser(t, db, "file_outsider")

	database := createResourceDatabase(t, db, owner.ID, "FilePermissionDB")
	grantResourceDatabaseAccess(t, db, database.ID, viewer.ID, "viewer")
	table := createResourceTable(t, db, database.ID, "Files")
	createResourceField(t, db, table.ID, "title", "string", true, "")

	record := createResourceRecord(t, db, table.ID, owner.ID, `{"title":"with-file"}`)

	uploadDir := t.TempDir()
	storedPath := filepath.Join(uploadDir, "keep.txt")
	require.NoError(t, os.WriteFile(storedPath, []byte("secret"), 0600))

	file := models.File{
		RecordID:   record.ID,
		FileName:   "keep.txt",
		FileSize:   int64(len("secret")),
		FileType:   ".txt",
		StorageURL: storedPath,
		UploadedBy: owner.ID,
	}
	require.NoError(t, db.Create(&file).Error)

	_, err := fileService.GetFile(file.ID, outsider.ID)
	require.Error(t, err)

	_, err = fileService.ListRecordFiles(record.ID, outsider.ID)
	require.Error(t, err)

	err = fileService.DeleteFile(file.ID, outsider.ID)
	require.Error(t, err)
	_, statErr := os.Stat(storedPath)
	require.NoError(t, statErr)

	header := createTestFileHeader(t, "file", "upload.txt", []byte("payload"))
	_, err = fileService.UploadFile(UploadFileRequest{
		RecordID: record.ID,
		File:     header,
	}, viewer.ID)
	require.Error(t, err)

	settingsService := NewSettingsService(db)
	_, err = settingsService.UpdateSettings(UpdateSettingsRequest{
		SystemName:        "Cornerstone",
		SystemDescription: "test",
		AllowRegistration: true,
		MaxFileSize:       1,
		DBType:            "sqlite",
		DBPoolSize:        10,
		DBTimeout:         30,
		PluginTimeout:     300,
		PluginWorkDir:     "./plugins",
		PluginAutoUpdate:  false,
	}, owner.ID)
	require.NoError(t, err)

	largeHeader := createTestFileHeader(t, "file", "large.txt", bytes.Repeat([]byte("a"), 2*1024*1024))
	_, err = fileService.UploadFile(UploadFileRequest{
		RecordID: record.ID,
		File:     largeHeader,
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "最大1MB")

	files, err := fileService.ListRecordFiles(record.ID, viewer.ID)
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestFileService_RejectsPathTraversalFilename(t *testing.T) {
	db := setupResourceTestDB(t)
	fileService := NewFileService(db)

	owner := createResourceUser(t, db, "file_owner_traversal")
	database := createResourceDatabase(t, db, owner.ID, "FileTraversalDB")
	table := createResourceTable(t, db, database.ID, "Files")
	createResourceField(t, db, table.ID, "title", "string", true, "")
	record := createResourceRecord(t, db, table.ID, owner.ID, `{"title":"secure"}`)

	header := createTestFileHeader(t, "file", "safe.txt", []byte("payload"))
	header.Filename = "..\\..\\escape.txt"

	_, err := fileService.UploadFile(UploadFileRequest{
		RecordID: record.ID,
		File:     header,
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "非法的文件名")

	var count int64
	require.NoError(t, db.Model(&models.File{}).Count(&count).Error)
	require.Zero(t, count)
}
