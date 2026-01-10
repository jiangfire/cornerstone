package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// TableService 表管理服务
type TableService struct {
	db *gorm.DB
}

// NewTableService 创建表服务实例
func NewTableService(db *gorm.DB) *TableService {
	return &TableService{db: db}
}

// CreateTableRequest 创建表请求
type CreateTableRequest struct {
	DatabaseID  string `json:"database_id" binding:"required"`
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateTableRequest 更新表请求
type UpdateTableRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
}

// TableResponse 表响应
type TableResponse struct {
	ID          string `json:"id"`
	DatabaseID  string `json:"database_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// checkDatabaseAccess 检查用户是否有数据库访问权限
func (s *TableService) checkDatabaseAccess(dbID, userID string, requiredRoles []string) error {
	var access models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ?", dbID, userID).First(&access).Error
	if err != nil {
		return errors.New("无权访问该数据库")
	}

	// 检查角色权限
	roleAllowed := false
	for _, role := range requiredRoles {
		if access.Role == role {
			roleAllowed = true
			break
		}
	}

	if !roleAllowed {
		return fmt.Errorf("需要权限：%v，当前角色：%s", requiredRoles, access.Role)
	}

	return nil
}

// validateTableName 验证表名称
func validateTableName(name string) error {
	name = strings.TrimSpace(name)

	if len(name) < 2 || len(name) > 255 {
		return errors.New("表名称长度必须在2-255个字符之间")
	}

	// 支持字母（包括中文）、数字、下划线
	// \p{L} 匹配所有语言的字母（包括中文）
	// \p{N} 匹配所有语言的数字
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_]+$`, name)
	if !matched {
		return errors.New("表名称只能包含字母、数字和下划线")
	}

	// 不能以ASCII数字开头（0-9）
	if matched, _ := regexp.MatchString(`^[0-9]`, name); matched {
		return errors.New("表名称不能以数字开头")
	}

	return nil
}

// sanitizeTableInput 清理表输入
func sanitizeTableInput(name, description string) (string, string) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	// 移除危险字符
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "'", "")

	description = strings.ReplaceAll(description, "<", "")
	description = strings.ReplaceAll(description, ">", "")
	description = strings.ReplaceAll(description, "\"", "")
	description = strings.ReplaceAll(description, "'", "")

	return name, description
}

// CreateTable 创建表
func (s *TableService) CreateTable(req CreateTableRequest, userID string) (*models.Table, error) {
	// 1. 检查数据库访问权限（owner, admin, editor可以创建表）
	if err := s.checkDatabaseAccess(req.DatabaseID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 2. 输入验证和清理
	req.Name, req.Description = sanitizeTableInput(req.Name, req.Description)

	if err := validateTableName(req.Name); err != nil {
		return nil, fmt.Errorf("表名称验证失败: %w", err)
	}

	if len(req.Description) > 500 {
		return nil, errors.New("描述长度不能超过500个字符")
	}

	// 3. 检查是否已存在同名表
	var existingTable models.Table
	err := s.db.Where("database_id = ? AND name = ?", req.DatabaseID, req.Name).First(&existingTable).Error
	if err == nil {
		return nil, errors.New("该数据库中已存在同名表")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 4. 创建表
	table := models.Table{
		DatabaseID:  req.DatabaseID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.db.Create(&table).Error; err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	return &table, nil
}

// ListTables 获取数据库表列表
func (s *TableService) ListTables(dbID, userID string) ([]TableResponse, error) {
	// 1. 检查数据库访问权限
	if err := s.checkDatabaseAccess(dbID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	// 2. 查询表列表
	var tables []models.Table
	err := s.db.Where("database_id = ? AND deleted_at IS NULL", dbID).Order("created_at ASC").Find(&tables).Error
	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 3. 转换为响应格式
	result := make([]TableResponse, len(tables))
	for i, t := range tables {
		result[i] = TableResponse{
			ID:          t.ID,
			DatabaseID:  t.DatabaseID,
			Name:        t.Name,
			Description: t.Description,
			CreatedAt:   t.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   t.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return result, nil
}

// GetTable 获取表详情
func (s *TableService) GetTable(tableID, userID string) (*TableResponse, error) {
	// 1. 获取表信息
	var table models.Table
	err := s.db.Where("id = ?", tableID).First(&table).Error
	if err != nil {
		return nil, fmt.Errorf("表不存在: %w", err)
	}

	// 2. 检查数据库访问权限
	if err := s.checkDatabaseAccess(table.DatabaseID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	return &TableResponse{
		ID:          table.ID,
		DatabaseID:  table.DatabaseID,
		Name:        table.Name,
		Description: table.Description,
		CreatedAt:   table.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   table.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateTable 更新表信息
func (s *TableService) UpdateTable(tableID string, req UpdateTableRequest, userID string) (*models.Table, error) {
	// 1. 获取表信息
	var table models.Table
	err := s.db.Where("id = ?", tableID).First(&table).Error
	if err != nil {
		return nil, fmt.Errorf("表不存在: %w", err)
	}

	// 2. 检查数据库访问权限（只有owner, admin, editor可以修改）
	if err := s.checkDatabaseAccess(table.DatabaseID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 3. 输入验证和清理
	req.Name, req.Description = sanitizeTableInput(req.Name, req.Description)

	if err := validateTableName(req.Name); err != nil {
		return nil, fmt.Errorf("表名称验证失败: %w", err)
	}

	if len(req.Description) > 500 {
		return nil, errors.New("描述长度不能超过500个字符")
	}

	// 4. 检查是否已存在同名表（排除当前表）
	var existingTable models.Table
	err = s.db.Where("database_id = ? AND name = ? AND id != ?", table.DatabaseID, req.Name, tableID).First(&existingTable).Error
	if err == nil {
		return nil, errors.New("该数据库中已存在同名表")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 5. 更新表信息
	table.Name = req.Name
	table.Description = req.Description

	if err := s.db.Save(&table).Error; err != nil {
		return nil, fmt.Errorf("更新表失败: %w", err)
	}

	return &table, nil
}

// DeleteTable 删除表（软删除）
func (s *TableService) DeleteTable(tableID, userID string) error {
	// 1. 获取表信息
	var table models.Table
	err := s.db.Where("id = ?", tableID).First(&table).Error
	if err != nil {
		return fmt.Errorf("表不存在: %w", err)
	}

	// 2. 检查数据库访问权限（只有owner, admin可以删除）
	if err := s.checkDatabaseAccess(table.DatabaseID, userID, []string{"owner", "admin"}); err != nil {
		return err
	}

	// 3. 软删除表
	if err := s.db.Delete(&table).Error; err != nil {
		return fmt.Errorf("删除表失败: %w", err)
	}

	return nil
}
