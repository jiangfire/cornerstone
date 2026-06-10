package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/jiangfire/cornerstone/internal/models"
	"gorm.io/gorm"
)

// FileService manages file operations
type FileService struct {
	db *gorm.DB
}

// fileUploadDir is the root directory for file storage (relative to process working directory).
// All download/delete paths must be validated by ResolveSecureStoragePath to be within this directory,
// preventing path traversal if StorageURL in DB is tampered with.
const fileUploadDir = "./uploads"

// ResolveSecureStoragePath resolves storageURL to an absolute path,
// validates it is within fileUploadDir, and returns the safe absolute path or an error.
func ResolveSecureStoragePath(storageURL string) (string, error) {
	if strings.TrimSpace(storageURL) == "" {
		return "", errors.New("file path is empty")
	}
	rootAbs, err := filepath.Abs(fileUploadDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve upload directory: %w", err)
	}
	targetAbs, err := filepath.Abs(storageURL)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("illegal file path")
	}
	return targetAbs, nil
}

// NewFileService creates a new FileService instance
func NewFileService(db *gorm.DB) *FileService {
	return &FileService{db: db}
}

// UploadFileRequest is the file upload request
type UploadFileRequest struct {
	RecordID string
	FieldID  string
	File     *multipart.FileHeader
}

// FileResponse is the file API response
type FileResponse struct {
	ID         string `json:"id"`
	RecordID   string `json:"record_id"`
	FieldID    string `json:"field_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileType   string `json:"file_type"`
	StorageURL string `json:"storage_url"`
}

func (s *FileService) getAccessibleRecord(recordID, userID string, requiredRoles []string) (*models.Record, error) {
	var record models.Record
	if err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("record not found")
		}
		return nil, fmt.Errorf("failed to query record: %w", err)
	}

	if err := NewRecordService(s.db).checkTableAccess(record.TableID, userID, requiredRoles); err != nil {
		return nil, err
	}

	return &record, nil
}

func (s *FileService) getAccessibleField(fieldID, userID string, requiredRoles []string) (*models.Field, error) {
	var field models.Field
	if err := s.db.Where("id = ? AND deleted_at IS NULL", fieldID).First(&field).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("field not found")
		}
		return nil, fmt.Errorf("failed to query field: %w", err)
	}

	if err := NewFieldService(s.db).checkTableAccess(field.TableID, userID, requiredRoles); err != nil {
		return nil, err
	}

	return &field, nil
}

func (s *FileService) getAccessibleFile(fileID, userID string, requiredRoles []string) (*models.File, *models.Record, error) {
	var file models.File
	if err := s.db.Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("file not found")
		}
		return nil, nil, fmt.Errorf("failed to query file: %w", err)
	}

	if file.RecordID != "" {
		record, err := s.getAccessibleRecord(file.RecordID, userID, requiredRoles)
		if err != nil {
			return nil, nil, err
		}
		return &file, record, nil
	}

	if file.FieldID != "" {
		if _, err := s.getAccessibleField(file.FieldID, userID, requiredRoles); err != nil {
			return nil, nil, err
		}
		return &file, nil, nil
	}

	return nil, nil, errors.New("file has no associated record or field, cannot access")
}

func (s *FileService) getMaxUploadSizeBytes() int64 {
	return int64(50 * 1024 * 1024)
}

func parseStoredFieldConfig(options string) FieldConfig {
	if options == "" {
		return FieldConfig{}
	}

	var config FieldConfig
	_ = json.Unmarshal([]byte(options), &config)
	return config
}

func normalizeFileTypeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func matchAllowedFileType(allowedType, fileName, fileType string) bool {
	normalizedAllowed := normalizeFileTypeToken(allowedType)
	if normalizedAllowed == "" {
		return false
	}

	extension := strings.ToLower(filepath.Ext(fileName))
	normalizedFileType := normalizeFileTypeToken(fileType)

	if strings.HasPrefix(normalizedAllowed, ".") {
		return extension == normalizedAllowed
	}

	if strings.HasSuffix(normalizedAllowed, "/*") {
		prefix := strings.TrimSuffix(normalizedAllowed, "*")
		return strings.HasPrefix(normalizedFileType, prefix)
	}

	return normalizedFileType == normalizedAllowed
}

func fileMatchesAllowedTypes(fileName, fileType string, allowedTypes []string) bool {
	if len(allowedTypes) == 0 {
		return true
	}

	for _, allowedType := range allowedTypes {
		if matchAllowedFileType(allowedType, fileName, fileType) {
			return true
		}
	}

	return false
}

// UploadFile uploads a file
func (s *FileService) UploadFile(req UploadFileRequest, userID string) (*models.File, error) {
	if req.RecordID == "" && req.FieldID == "" {
		return nil, errors.New("at least one of record ID or field ID is required")
	}

	var record *models.Record
	if req.RecordID != "" {
		var err error
		record, err = s.getAccessibleRecord(req.RecordID, userID, []string{"owner", "admin", "editor"})
		if err != nil {
			return nil, err
		}
	}

	var field *models.Field
	if req.FieldID != "" {
		var err error
		field, err = s.getAccessibleField(req.FieldID, userID, []string{"owner", "admin", "editor"})
		if err != nil {
			return nil, err
		}
		if !isAttachmentFieldType(field.Type) {
			return nil, errors.New("only file type fields support file uploads")
		}
		if record != nil && field.TableID != record.TableID {
			return nil, errors.New("field does not belong to the record's table")
		}
	}

	// Validate file size (from system settings)
	maxSize := s.getMaxUploadSizeBytes()
	if req.File.Size > maxSize {
		return nil, fmt.Errorf("file size exceeds limit (max %dMB)", maxSize/1024/1024)
	}

	// Validate file name and file type
	fileName := strings.TrimSpace(req.File.Filename)
	if fileName == "" {
		return nil, errors.New("file name is required")
	}
	if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") || filepath.Base(fileName) != fileName {
		return nil, errors.New("illegal file name")
	}

	allowedTypes := []string{".jpg", ".jpeg", ".png", ".gif", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".txt", ".zip"}
	ext := strings.ToLower(filepath.Ext(fileName))
	contentType := strings.ToLower(strings.TrimSpace(req.File.Header.Get("Content-Type")))
	if contentType == "" {
		contentType = ext
	}
	allowed := false
	for _, t := range allowedTypes {
		if ext == t {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errors.New("unsupported file type")
	}

	if field != nil {
		config := parseStoredFieldConfig(field.Options)
		if config.MaxFileSizeMB > 0 && req.File.Size > int64(config.MaxFileSizeMB)*1024*1024 {
			return nil, fmt.Errorf("file size exceeds field limit (max %dMB)", config.MaxFileSizeMB)
		}
		if !fileMatchesAllowedTypes(fileName, contentType, config.AllowedTypes) {
			return nil, errors.New("file type does not match field restrictions")
		}
	}

	// Create upload directory
	uploadDir := fileUploadDir
	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Generate unique file name
	filename := fmt.Sprintf("%s_%s", models.GenerateID("file"), fileName)
	targetFilepath := filepath.Join(uploadDir, filename)

	// Validate file path safety (prevent directory traversal)
	uploadDirAbs, err := filepath.Abs(uploadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload directory absolute path: %w", err)
	}
	targetFilepathAbs, err := filepath.Abs(targetFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file absolute path: %w", err)
	}
	// Check file path is within upload directory
	if !strings.HasPrefix(targetFilepathAbs, uploadDirAbs) {
		return nil, errors.New("illegal file path")
	}

	// Save file
	src, err := req.File.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer func() {
		//nolint:staticcheck // SA9003 - Close errors are informational in defer
		if err := src.Close(); err != nil {
			// Log close error
		}
	}()

	// #nosec G304 - path validated by filepath.Abs() to be within allowed directory
	dst, err := os.Create(targetFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		//nolint:staticcheck // SA9003 - Close errors are informational in defer
		if err := dst.Close(); err != nil {
			// Log close error
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Create file record
	file := models.File{
		RecordID:   req.RecordID,
		FieldID:    req.FieldID,
		FileName:   fileName,
		FileSize:   req.File.Size,
		FileType:   contentType,
		StorageURL: targetFilepath,
	}

	if err := s.db.Create(&file).Error; err != nil {
		// Delete saved file
		//nolint:staticcheck // SA9003 - Cleanup error is informational
		if rmErr := os.Remove(targetFilepath); rmErr != nil {
			// Log deletion failure but return primary error
		}
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	return &file, nil
}

// GetFile gets file information
func (s *FileService) GetFile(fileID, userID string) (*models.File, error) {
	file, _, err := s.getAccessibleFile(fileID, userID, []string{"owner", "admin", "editor", "viewer"})
	if err != nil {
		return nil, err
	}
	return file, nil
}

// DeleteFile deletes a file
func (s *FileService) DeleteFile(fileID, userID string) error {
	file, _, err := s.getAccessibleFile(fileID, userID, []string{"owner", "admin", "editor"})
	if err != nil {
		return err
	}

	if err := s.removeFileReferenceFromRecord(file); err != nil {
		return err
	}

	// Delete physical file
	if err := os.Remove(file.StorageURL); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete physical file: %w", err)
	}

	// Delete database record
	if err := s.db.Delete(&file).Error; err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}

	return nil
}

func removeAttachmentReferenceValue(currentValue interface{}, fileID string) (interface{}, bool) {
	switch value := currentValue.(type) {
	case string:
		if value == fileID {
			return "", true
		}
		return currentValue, false
	case []interface{}:
		filtered := make([]interface{}, 0, len(value))
		removed := false
		for _, item := range value {
			if itemStr, ok := item.(string); ok && itemStr == fileID {
				removed = true
				continue
			}
			filtered = append(filtered, item)
		}
		return filtered, removed
	case []string:
		filtered := make([]string, 0, len(value))
		removed := false
		for _, item := range value {
			if item == fileID {
				removed = true
				continue
			}
			filtered = append(filtered, item)
		}
		return filtered, removed
	default:
		return currentValue, false
	}
}

func (s *FileService) removeFileReferenceFromRecord(file *models.File) error {
	if file == nil || file.RecordID == "" || file.FieldID == "" {
		return nil
	}

	var record models.Record
	if err := s.db.Where("id = ? AND deleted_at IS NULL", file.RecordID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("failed to query associated record: %w", err)
	}

	var field models.Field
	if err := s.db.Where("id = ? AND deleted_at IS NULL", file.FieldID).First(&field).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("failed to query attachment field: %w", err)
	}

	payload := parseRecordPayload(record.Data)
	currentValue, exists := payload[field.Name]
	if !exists {
		return nil
	}

	updatedValue, removed := removeAttachmentReferenceValue(currentValue, file.ID)
	if !removed {
		return nil
	}
	payload[field.Name] = updatedValue

	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to update record attachment reference: %w", err)
	}

	if err := s.db.Model(&models.Record{}).Where("id = ?", record.ID).Updates(map[string]interface{}{
		"data":       string(dataJSON),
		"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		"version":    gorm.Expr("version + 1"),
	}).Error; err != nil {
		return fmt.Errorf("failed to save record attachment reference: %w", err)
	}

	return nil
}

// ListRecordFiles lists all files for a record
func (s *FileService) ListRecordFiles(recordID, userID string) ([]models.File, error) {
	if _, err := s.getAccessibleRecord(recordID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	var files []models.File
	if err := s.db.Where("record_id = ?", recordID).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to query file list: %w", err)
	}
	return files, nil
}
