package services

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// FileService 文件管理服务
type FileService struct {
	db *gorm.DB
}

// NewFileService 创建文件服务实例
func NewFileService(db *gorm.DB) *FileService {
	return &FileService{db: db}
}

// UploadFileRequest 文件上传请求
type UploadFileRequest struct {
	RecordID string
	File     *multipart.FileHeader
}

// FileResponse 文件响应
type FileResponse struct {
	ID         string `json:"id"`
	RecordID   string `json:"record_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileType   string `json:"file_type"`
	StorageURL string `json:"storage_url"`
	UploadedBy string `json:"uploaded_by"`
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

func (s *FileService) getAccessibleFile(fileID, userID string, requiredRoles []string) (*models.File, *models.Record, error) {
	var file models.File
	if err := s.db.Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("文件不存在")
		}
		return nil, nil, fmt.Errorf("查询文件失败: %w", err)
	}

	record, err := s.getAccessibleRecord(file.RecordID, userID, requiredRoles)
	if err != nil {
		return nil, nil, err
	}

	return &file, record, nil
}

func (s *FileService) getMaxUploadSizeBytes() int64 {
	settings, err := NewSettingsService(s.db).GetSettings()
	if err != nil || settings.MaxFileSize <= 0 {
		return int64(50 * 1024 * 1024)
	}
	return int64(settings.MaxFileSize) * 1024 * 1024
}

// UploadFile 上传文件
func (s *FileService) UploadFile(req UploadFileRequest, userID string) (*models.File, error) {
	// 1. 验证记录存在且当前用户有写入权限
	if _, err := s.getAccessibleRecord(req.RecordID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
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

	// 4. 创建上传目录
	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0750); err != nil {
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
		FileName:   fileName,
		FileSize:   req.File.Size,
		FileType:   ext,
		StorageURL: targetFilepath,
		UploadedBy: userID,
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
