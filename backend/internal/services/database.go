package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// DatabaseService 数据库管理服务
type DatabaseService struct {
	db *gorm.DB
}

// NewDatabaseService 创建数据库服务实例
func NewDatabaseService(db *gorm.DB) *DatabaseService {
	return &DatabaseService{db: db}
}

// CreateDBRequest 创建数据库请求
type CreateDBRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
	IsPublic    bool   `json:"is_public"`
	IsPersonal  bool   `json:"is_personal"`
}

// UpdateDBRequest 更新数据库请求
type UpdateDBRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
	IsPublic    bool   `json:"is_public"`
}

// DBResponse 数据库响应
type DBResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
	IsPublic    bool   `json:"is_public"`
	IsPersonal  bool   `json:"is_personal"`
	Role        string `json:"role"` // 当前用户权限
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ShareDBRequest 数据库分享请求
type ShareDBRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=owner admin editor viewer"`
}

// validateDatabaseName 验证数据库名称
func validateDatabaseName(name string) error {
	name = strings.TrimSpace(name)

	if len(name) < 2 || len(name) > 255 {
		return errors.New("数据库名称长度必须在2-255个字符之间")
	}

	// 支持字母（包括中文）、数字、下划线、连字符和空格
	// \p{L} 匹配所有语言的字母（包括中文）
	// \p{N} 匹配所有语言的数字
	// \s 匹配空白字符
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_\-\s]+$`, name)
	if !matched {
		return errors.New("数据库名称只能包含字母、数字、下划线、连字符和空格")
	}

	return nil
}

// validateDescription 验证描述
func validateDescription(desc string) error {
	if len(desc) > 500 {
		return errors.New("描述长度不能超过500个字符")
	}
	return nil
}

// sanitizeDatabaseInput 清理数据库输入
func sanitizeDatabaseInput(name, description string) (string, string) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	// 防止XSS攻击 - 移除危险字符
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

// CreateDatabase 创建数据库
func (s *DatabaseService) CreateDatabase(req CreateDBRequest, ownerID string) (*models.Database, error) {
	// 1. 输入验证和清理
	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("数据库名称验证失败: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("描述验证失败: %w", err)
	}

	// 2. 检查是否已存在同名数据库（同一用户）
	var existingDB models.Database
	err := s.db.Where("name = ? AND owner_id = ?", req.Name, ownerID).First(&existingDB).Error
	if err == nil {
		return nil, errors.New("您已创建过同名数据库")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 3. 创建数据库
	database := models.Database{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     ownerID,
		IsPublic:    req.IsPublic,
		IsPersonal:  req.IsPersonal,
	}

	if err := s.db.Create(&database).Error; err != nil {
		return nil, fmt.Errorf("创建数据库失败: %w", err)
	}

	// 4. 自动为所有者添加权限
	access := models.DatabaseAccess{
		UserID:     ownerID,
		DatabaseID: database.ID,
		Role:       "owner",
	}

	if err := s.db.Create(&access).Error; err != nil {
		return nil, fmt.Errorf("添加所有者权限失败: %w", err)
	}

	return &database, nil
}

// ListDatabases 获取用户可访问的数据库列表
func (s *DatabaseService) ListDatabases(userID string) ([]DBResponse, error) {
	// 查询用户有权限的所有数据库
	var results []struct {
		DatabaseID   string
		Name         string
		Description  string
		OwnerID      string
		IsPublic     bool
		IsPersonal   bool
		DatabaseRole string
		CreatedAt    string
		UpdatedAt    string
	}

	err := s.db.Raw(`
		SELECT
			d.id as database_id,
			d.name,
			d.description,
			d.owner_id,
			d.is_public,
			d.is_personal,
			da.role as database_role,
			d.created_at,
			d.updated_at
		FROM databases d
		INNER JOIN database_access da ON d.id = da.database_id
		WHERE da.user_id = ? AND d.deleted_at IS NULL
		ORDER BY d.created_at DESC
	`, userID).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 转换为响应格式
	dbs := make([]DBResponse, len(results))
	for i, r := range results {
		dbs[i] = DBResponse{
			ID:          r.DatabaseID,
			Name:        r.Name,
			Description: r.Description,
			OwnerID:     r.OwnerID,
			IsPublic:    r.IsPublic,
			IsPersonal:  r.IsPersonal,
			Role:        r.DatabaseRole,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		}
	}

	return dbs, nil
}

// GetDatabase 获取数据库详情
func (s *DatabaseService) GetDatabase(dbID, userID string) (*DBResponse, error) {
	// 检查用户是否有权限访问该数据库
	var access models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ?", dbID, userID).First(&access).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("无权访问该数据库")
		}
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 获取数据库信息
	var database models.Database
	err = s.db.Where("id = ?", dbID).First(&database).Error
	if err != nil {
		return nil, fmt.Errorf("数据库不存在: %w", err)
	}

	return &DBResponse{
		ID:          database.ID,
		Name:        database.Name,
		Description: database.Description,
		OwnerID:     database.OwnerID,
		IsPublic:    database.IsPublic,
		IsPersonal:  database.IsPersonal,
		Role:        access.Role,
		CreatedAt:   database.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   database.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateDatabase 更新数据库信息
func (s *DatabaseService) UpdateDatabase(dbID string, req UpdateDBRequest, userID string) (*models.Database, error) {
	// 1. 检查用户权限
	var access models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ?", dbID, userID).First(&access).Error
	if err != nil {
		return nil, errors.New("无权访问该数据库")
	}

	// 2. 检查权限（只有owner和admin可以修改）
	if access.Role != "owner" && access.Role != "admin" {
		return nil, errors.New("只有所有者和管理员可以修改数据库信息")
	}

	// 3. 输入验证和清理
	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("数据库名称验证失败: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("描述验证失败: %w", err)
	}

	// 4. 获取数据库并更新
	var database models.Database
	err = s.db.Where("id = ?", dbID).First(&database).Error
	if err != nil {
		return nil, fmt.Errorf("数据库不存在: %w", err)
	}

	database.Name = req.Name
	database.Description = req.Description
	database.IsPublic = req.IsPublic

	if err := s.db.Save(&database).Error; err != nil {
		return nil, fmt.Errorf("更新数据库失败: %w", err)
	}

	return &database, nil
}

// DeleteDatabase 删除数据库（软删除）
func (s *DatabaseService) DeleteDatabase(dbID, userID string) error {
	// 1. 检查是否是所有者
	var access models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ? AND role = ?", dbID, userID, "owner").First(&access).Error
	if err != nil {
		return errors.New("只有所有者可以删除数据库")
	}

	// 2. 软删除数据库
	if err := s.db.Delete(&models.Database{}, dbID).Error; err != nil {
		return fmt.Errorf("删除数据库失败: %w", err)
	}

	return nil
}

// ShareDatabase 分享数据库给其他用户
func (s *DatabaseService) ShareDatabase(dbID string, req ShareDBRequest, operatorID string) error {
	// 1. 检查操作者权限
	var operatorAccess models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ?", dbID, operatorID).First(&operatorAccess).Error
	if err != nil {
		return errors.New("无权访问该数据库")
	}

	// 2. 检查权限（只有owner和admin可以分享）
	if operatorAccess.Role != "owner" && operatorAccess.Role != "admin" {
		return errors.New("只有所有者和管理员可以分享数据库")
	}

	// 3. 检查被分享用户是否存在
	var user models.User
	err = s.db.Where("id = ?", req.UserID).First(&user).Error
	if err != nil {
		return errors.New("用户不存在")
	}

	// 4. 检查是否已分享
	var existingAccess models.DatabaseAccess
	err = s.db.Where("database_id = ? AND user_id = ?", dbID, req.UserID).First(&existingAccess).Error
	if err == nil {
		return errors.New("该用户已有访问权限")
	}

	// 5. 权限验证：owner只能分享给owner，admin可以分享给admin/editor/viewer
	if operatorAccess.Role == "owner" && req.Role != "owner" {
		return errors.New("所有者只能将数据库分享给其他所有者")
	}

	// 6. 添加权限
	access := models.DatabaseAccess{
		UserID:     req.UserID,
		DatabaseID: dbID,
		Role:       req.Role,
	}

	if err := s.db.Create(&access).Error; err != nil {
		return fmt.Errorf("分享数据库失败: %w", err)
	}

	return nil
}

// ListDatabaseUsers 获取数据库用户列表
func (s *DatabaseService) ListDatabaseUsers(dbID, userID string) ([]interface{}, error) {
	// 1. 检查用户是否有权限访问该数据库
	var access models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ?", dbID, userID).First(&access).Error
	if err != nil {
		return nil, errors.New("无权访问该数据库")
	}

	// 2. 查询数据库用户列表
	var users []struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		JoinedAt string `json:"joined_at"`
	}

	err = s.db.Raw(`
		SELECT
			da.user_id,
			u.username,
			u.email,
			da.role,
			da.created_at as joined_at
		FROM database_access da
		INNER JOIN users u ON da.user_id = u.id
		WHERE da.database_id = ?
		ORDER BY da.created_at ASC
	`, dbID).Scan(&users).Error

	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 转换为 interface{} 切片
	result := make([]interface{}, len(users))
	for i, u := range users {
		result[i] = u
	}

	return result, nil
}

// RemoveDatabaseUser 移除数据库用户
func (s *DatabaseService) RemoveDatabaseUser(dbID, removeUserID, operatorID string) error {
	// 1. 检查操作者权限
	var operatorAccess models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ?", dbID, operatorID).First(&operatorAccess).Error
	if err != nil {
		return errors.New("无权访问该数据库")
	}

	// 2. 不能移除所有者
	var targetAccess models.DatabaseAccess
	err = s.db.Where("database_id = ? AND user_id = ?", dbID, removeUserID).First(&targetAccess).Error
	if err != nil {
		return errors.New("用户不存在或无权限")
	}

	if targetAccess.Role == "owner" {
		return errors.New("不能移除数据库所有者")
	}

	// 3. 检查权限（只有owner和admin可以移除用户）
	if operatorAccess.Role != "owner" && operatorAccess.Role != "admin" {
		return errors.New("只有所有者和管理员可以移除用户")
	}

	// 4. 移除用户权限
	if err := s.db.Delete(&targetAccess).Error; err != nil {
		return fmt.Errorf("移除用户失败: %w", err)
	}

	return nil
}

// UpdateDatabaseUserRole 更新数据库用户角色
func (s *DatabaseService) UpdateDatabaseUserRole(dbID, updateUserID string, req ShareDBRequest, operatorID string) error {
	// 1. 检查操作者权限（只有owner可以修改角色）
	var operatorAccess models.DatabaseAccess
	err := s.db.Where("database_id = ? AND user_id = ? AND role = ?", dbID, operatorID, "owner").First(&operatorAccess).Error
	if err != nil {
		return errors.New("只有数据库所有者可以修改用户角色")
	}

	// 2. 获取目标用户权限
	var targetAccess models.DatabaseAccess
	err = s.db.Where("database_id = ? AND user_id = ?", dbID, updateUserID).First(&targetAccess).Error
	if err != nil {
		return errors.New("用户不存在或无权限")
	}

	// 3. 不能修改所有者角色
	if targetAccess.Role == "owner" {
		return errors.New("不能修改数据库所有者的角色")
	}

	// 4. 更新角色
	targetAccess.Role = req.Role
	if err := s.db.Save(&targetAccess).Error; err != nil {
		return fmt.Errorf("更新角色失败: %w", err)
	}

	return nil
}
