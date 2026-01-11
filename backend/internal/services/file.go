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

// UploadFile 上传文件
func (s *FileService) UploadFile(req UploadFileRequest, userID string) (*models.File, error) {
	// 1. 验证记录是否存在
	var record models.Record
	if err := s.db.Where("id = ?", req.RecordID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("记录不存在")
		}
		return nil, fmt.Errorf("查询记录失败: %w", err)
	}

	// 2. 验证文件大小（限制为50MB）
	maxSize := int64(50 * 1024 * 1024)
	if req.File.Size > maxSize {
		return nil, errors.New("文件大小超过限制（最大50MB）")
	}

	// 3. 验证文件类型
	allowedTypes := []string{".jpg", ".jpeg", ".png", ".gif", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".txt", ".zip"}
	ext := strings.ToLower(filepath.Ext(req.File.Filename))
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
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	// 5. 生成唯一文件名
	filename := fmt.Sprintf("%s_%s", models.GenerateID("file"), req.File.Filename)
	filepath := filepath.Join(uploadDir, filename)

	// 6. 保存文件
	src, err := req.File.Open()
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	// 7. 创建文件记录
	file := models.File{
		RecordID:   req.RecordID,
		FileName:   req.File.Filename,
		FileSize:   req.File.Size,
		FileType:   ext,
		StorageURL: filepath,
		UploadedBy: userID,
	}

	if err := s.db.Create(&file).Error; err != nil {
		os.Remove(filepath) // 删除已保存的文件
		return nil, fmt.Errorf("创建文件记录失败: %w", err)
	}

	return &file, nil
}

// GetFile 获取文件信息
func (s *FileService) GetFile(fileID string) (*models.File, error) {
	var file models.File
	if err := s.db.Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("文件不存在")
		}
		return nil, fmt.Errorf("查询文件失败: %w", err)
	}
	return &file, nil
}

// DeleteFile 删除文件
func (s *FileService) DeleteFile(fileID string) error {
	var file models.File
	if err := s.db.Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("文件不存在")
		}
		return fmt.Errorf("查询文件失败: %w", err)
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
func (s *FileService) ListRecordFiles(recordID string) ([]models.File, error) {
	var files []models.File
	if err := s.db.Where("record_id = ?", recordID).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("查询文件列表失败: %w", err)
	}
	return files, nil
}

