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

// FileService 文件管理服务
type FileService struct {
	db *gorm.DB
}

// fileUploadDir 文件存储根目录（相对于进程工作目录）。
// 所有下载/删除路径都必须通过 ResolveSecureStoragePath 校验落在该目录下，
// 防止 DB 中的 StorageURL 被恶意篡改后导致路径穿越。
const fileUploadDir = "./uploads"

// ResolveSecureStoragePath 把 storageURL 解析为绝对路径，
// 并校验其位于 fileUploadDir 之内。返回安全可读的绝对路径或错误。
func ResolveSecureStoragePath(storageURL string) (string, error) {
	if strings.TrimSpace(storageURL) == "" {
		return "", errors.New("文件路径为空")
	}
	rootAbs, err := filepath.Abs(fileUploadDir)
	if err != nil {
		return "", fmt.Errorf("解析上传目录失败: %w", err)
	}
	targetAbs, err := filepath.Abs(storageURL)
	if err != nil {
		return "", fmt.Errorf("解析文件路径失败: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("非法的文件路径")
	}
	return targetAbs, nil
}

// NewFileService 创建文件服务实例
func NewFileService(db *gorm.DB) *FileService {
	return &FileService{db: db}
}

// UploadFileRequest 文件上传请求
type UploadFileRequest struct {
	RecordID string
	FieldID  string
	File     *multipart.FileHeader
}

// FileResponse 文件响应
type FileResponse struct {
	ID         string `json:"id"`
	RecordID   string `json:"record_id"`
	FieldID    string `json:"field_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileType   string `json:"file_type"`
	StorageURL string `json:"storage_url"`
	CreatedAt  string `json:"created_at"`
}

func (s *FileService) getAccessibleRecord(recordID, userID string, requiredRoles []string) (*models.Record, error) {
	var record models.Record
	if err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("记录不存在")
		}
		return nil, fmt.Errorf("查询记录失败: %w", err)
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
			return nil, errors.New("字段不存在")
		}
		return nil, fmt.Errorf("查询字段失败: %w", err)
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
			return nil, nil, errors.New("文件不存在")
		}
		return nil, nil, fmt.Errorf("查询文件失败: %w", err)
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

	return nil, nil, errors.New("文件缺少关联记录或字段，无法访问")
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

// UploadFile 上传文件
func (s *FileService) UploadFile(req UploadFileRequest, userID string) (*models.File, error) {
	if req.RecordID == "" && req.FieldID == "" {
		return nil, errors.New("记录ID或字段ID至少需要提供一个")
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
			return nil, errors.New("仅 file 类型字段支持绑定上传文件")
		}
		if record != nil && field.TableID != record.TableID {
			return nil, errors.New("字段不属于当前记录所在表")
		}
	}

	// 2. 验证文件大小（读取系统设置）
	maxSize := s.getMaxUploadSizeBytes()
	if req.File.Size > maxSize {
		return nil, fmt.Errorf("文件大小超过限制（最大%dMB）", maxSize/1024/1024)
	}

	// 3. 验证文件名和文件类型
	fileName := strings.TrimSpace(req.File.Filename)
	if fileName == "" {
		return nil, errors.New("文件名不能为空")
	}
	if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") || filepath.Base(fileName) != fileName {
		return nil, errors.New("非法的文件名")
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
		return nil, errors.New("不支持的文件类型")
	}

	if field != nil {
		config := parseStoredFieldConfig(field.Options)
		if config.MaxFileSizeMB > 0 && req.File.Size > int64(config.MaxFileSizeMB)*1024*1024 {
			return nil, fmt.Errorf("文件大小超过字段限制（最大%dMB）", config.MaxFileSizeMB)
		}
		if !fileMatchesAllowedTypes(fileName, contentType, config.AllowedTypes) {
			return nil, errors.New("文件类型不符合字段限制")
		}
	}

	// 4. 创建上传目录
	uploadDir := fileUploadDir
	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	// 5. 生成唯一文件名
	filename := fmt.Sprintf("%s_%s", models.GenerateID("file"), fileName)
	targetFilepath := filepath.Join(uploadDir, filename)

	// 5.1 验证文件路径安全（防止目录遍历攻击）
	uploadDirAbs, err := filepath.Abs(uploadDir)
	if err != nil {
		return nil, fmt.Errorf("获取上传目录绝对路径失败: %w", err)
	}
	targetFilepathAbs, err := filepath.Abs(targetFilepath)
	if err != nil {
		return nil, fmt.Errorf("获取文件绝对路径失败: %w", err)
	}
	// 检查文件路径是否在上传目录内
	if !strings.HasPrefix(targetFilepathAbs, uploadDirAbs) {
		return nil, errors.New("非法的文件路径")
	}

	// 6. 保存文件
	src, err := req.File.Open()
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer func() {
		//nolint:staticcheck // SA9003 - Close errors are informational in defer
		if err := src.Close(); err != nil {
			// 记录关闭错误
		}
	}()

	// #nosec G304 - 已通过filepath.Abs()验证路径在允许的目录内
	dst, err := os.Create(targetFilepath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() {
		//nolint:staticcheck // SA9003 - Close errors are informational in defer
		if err := dst.Close(); err != nil {
			// 记录关闭错误
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	// 7. 创建文件记录
	file := models.File{
		RecordID:   req.RecordID,
		FieldID:    req.FieldID,
		FileName:   fileName,
		FileSize:   req.File.Size,
		FileType:   contentType,
		StorageURL: targetFilepath,
	}

	if err := s.db.Create(&file).Error; err != nil {
		// 删除已保存的文件
		//nolint:staticcheck // SA9003 - Cleanup error is informational
		if rmErr := os.Remove(targetFilepath); rmErr != nil {
			// 记录删除失败错误，但返回主要错误
		}
		return nil, fmt.Errorf("创建文件记录失败: %w", err)
	}

	return &file, nil
}

// GetFile 获取文件信息
func (s *FileService) GetFile(fileID, userID string) (*models.File, error) {
	file, _, err := s.getAccessibleFile(fileID, userID, []string{"owner", "admin", "editor", "viewer"})
	if err != nil {
		return nil, err
	}
	return file, nil
}

// DeleteFile 删除文件
func (s *FileService) DeleteFile(fileID, userID string) error {
	file, _, err := s.getAccessibleFile(fileID, userID, []string{"owner", "admin", "editor"})
	if err != nil {
		return err
	}

	if err := s.removeFileReferenceFromRecord(file); err != nil {
		return err
	}

	// 删除物理文件
	if err := os.Remove(file.StorageURL); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除物理文件失败: %w", err)
	}

	// 删除数据库记录
	if err := s.db.Delete(&file).Error; err != nil {
		return fmt.Errorf("删除文件记录失败: %w", err)
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
		return fmt.Errorf("查询关联记录失败: %w", err)
	}

	var field models.Field
	if err := s.db.Where("id = ? AND deleted_at IS NULL", file.FieldID).First(&field).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("查询附件字段失败: %w", err)
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
		return fmt.Errorf("更新记录附件引用失败: %w", err)
	}

	if err := s.db.Model(&models.Record{}).Where("id = ?", record.ID).Updates(map[string]interface{}{
		"data":       string(dataJSON),
		"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		"version":    gorm.Expr("version + 1"),
	}).Error; err != nil {
		return fmt.Errorf("保存记录附件引用失败: %w", err)
	}

	return nil
}

// ListRecordFiles 列出记录的所有文件
func (s *FileService) ListRecordFiles(recordID, userID string) ([]models.File, error) {
	if _, err := s.getAccessibleRecord(recordID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	var files []models.File
	if err := s.db.Where("record_id = ?", recordID).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("查询文件列表失败: %w", err)
	}
	return files, nil
}
