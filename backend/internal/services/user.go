package services

import (
	"errors"
	"fmt"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// UserService 用户服务
type UserService struct {
	db *gorm.DB
}

// NewUserService 创建用户服务实例
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// UserResponse 用户响应（不含敏感信息）
type UserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// ListAvailableUsers 获取可用用户列表（用于选择成员/共享用户）
func (s *UserService) ListAvailableUsers(operatorID, orgID, dbID string) ([]UserResponse, error) {
	query := s.db.Model(&models.User{}).
		Select("id, username, email").
		Where("id != ?", operatorID)

	// 如果提供了org_id，排除已存在的成员
	if orgID != "" {
		subQuery := s.db.Model(&models.OrganizationMember{}).
			Where("organization_id = ?", orgID).
			Select("user_id")
		query = query.Where("id NOT IN (?)", subQuery)
	}

	// 如果提供了db_id，排除已有权限的用户
	if dbID != "" {
		subQuery := s.db.Model(&models.DatabaseAccess{}).
			Where("database_id = ?", dbID).
			Select("user_id")
		query = query.Where("id NOT IN (?)", subQuery)
	}

	var users []UserResponse
	err := query.Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return users, nil
}

// SearchUsers 搜索用户
func (s *UserService) SearchUsers(operatorID, query string) ([]UserResponse, error) {
	var users []UserResponse

	err := s.db.Model(&models.User{}).
		Select("id, username, email").
		Where("id != ?", operatorID).
		Where("username LIKE ? OR email LIKE ?", "%"+query+"%", "%"+query+"%").
		Limit(50).
		Find(&users).Error

	if err != nil {
		return nil, fmt.Errorf("搜索用户失败: %w", err)
	}

	return users, nil
}

// GetUserByID 根据ID获取用户信息
func (s *UserService) GetUserByID(userID string) (*UserResponse, error) {
	var user UserResponse
	err := s.db.Model(&models.User{}).
		Select("id, username, email").
		Where("id = ?", userID).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return &user, nil
}
