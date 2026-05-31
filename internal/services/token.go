package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/jiangfire/cornerstone/internal/models"
	"gorm.io/gorm"
)

// TokenService Token 管理服务
type TokenService struct {
	db *gorm.DB
}

// NewTokenService 创建 Token 服务实例
func NewTokenService(db *gorm.DB) *TokenService {
	return &TokenService{db: db}
}

// CreateTokenRequest 创建 Token 请求
type CreateTokenRequest struct {
	Name      string     `json:"name" binding:"required,min=1,max=255"`
	Scopes    string     `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// CreateToken 创建 Token（需 Master Token）
func (s *TokenService) CreateToken(req CreateTokenRequest) (*models.Token, error) {
	token := &models.Token{
		Name:      req.Name,
		IsMaster:  false,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
	}

	if err := s.db.Create(token).Error; err != nil {
		return nil, fmt.Errorf("创建 Token 失败: %w", err)
	}
	return token, nil
}

// ListTokens 列出 Token
// Master Token 看全部，普通 Token 只看自己
func (s *TokenService) ListTokens(tokenID string, isMaster bool) ([]models.Token, error) {
	var tokens []models.Token
	query := s.db.Where("is_master = ?", false).Order("created_at DESC")

	if !isMaster {
		query = query.Where("id = ?", tokenID)
	}

	if err := query.Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("查询 Token 列表失败: %w", err)
	}
	return tokens, nil
}

// DeleteToken 删除 Token
// Master Token 可删除任意，普通 Token 只能删除自己
func (s *TokenService) DeleteToken(tokenID string, targetID string, isMaster bool) error {
	if !isMaster && tokenID != targetID {
		return errors.New("无权删除其他 Token")
	}

	var t models.Token
	if err := s.db.Where("id = ?", targetID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Token 不存在")
		}
		return fmt.Errorf("查询 Token 失败: %w", err)
	}

	if t.IsMaster && !isMaster {
		return errors.New("无权删除 Master Token")
	}

	if err := s.db.Delete(&t).Error; err != nil {
		return fmt.Errorf("删除 Token 失败: %w", err)
	}
	return nil
}

// UpdateToken 更新 Token 权限（需 Master Token）
func (s *TokenService) UpdateToken(targetID string, scopes string, expiresAt *time.Time) (*models.Token, error) {
	var t models.Token
	if err := s.db.Where("id = ?", targetID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Token 不存在")
		}
		return nil, fmt.Errorf("查询 Token 失败: %w", err)
	}

	if t.IsMaster {
		return nil, errors.New("不能修改 Master Token 权限")
	}

	updates := map[string]interface{}{
		"scopes":     scopes,
		"expires_at": expiresAt,
	}
	if err := s.db.Model(&t).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新 Token 失败: %w", err)
	}

	// 重新查询返回最新数据
	if err := s.db.Where("id = ?", targetID).First(&t).Error; err != nil {
		return nil, fmt.Errorf("查询最新 Token 失败: %w", err)
	}
	return &t, nil
}
