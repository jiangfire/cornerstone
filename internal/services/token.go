package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/dto"
	"gorm.io/gorm"
)

// TokenService manages token operations
type TokenService struct {
	db *gorm.DB
}

// NewTokenService creates a new TokenService instance
func NewTokenService(db *gorm.DB) *TokenService {
	return &TokenService{db: db}
}

// CreateToken creates a new token (requires master token)
func (s *TokenService) CreateToken(req dto.TokenCreateRequest) (*models.Token, error) {
	token := &models.Token{
		Name:      req.Name,
		IsMaster:  false,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
	}

	if err := s.db.Create(token).Error; err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}
	return token, nil
}

// ListTokens lists tokens
// Master token sees all; regular tokens see only themselves
func (s *TokenService) ListTokens(tokenID string, isMaster bool) ([]dto.TokenObject, error) {
	var tokens []models.Token
	query := s.db.Where("is_master = ?", false).Order("created_at DESC")

	if !isMaster {
		query = query.Where("id = ?", tokenID)
	}

	if err := query.Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}

	result := make([]dto.TokenObject, len(tokens))
	for i := range tokens {
		result[i] = dto.TokenObject{
			ID:        tokens[i].ID,
			Name:      tokens[i].Name,
			IsMaster:  tokens[i].IsMaster,
			Scopes:    tokens[i].Scopes,
			ExpiresAt: tokens[i].ExpiresAt,
		}
	}
	return result, nil
}

// DeleteToken deletes a token
// Master token can delete any; regular tokens can only delete themselves
func (s *TokenService) DeleteToken(tokenID string, targetID string, isMaster bool) error {
	if !isMaster && tokenID != targetID {
		return errors.New("permission denied: cannot delete other tokens")
	}

	var t models.Token
	if err := s.db.Where("id = ?", targetID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("token not found")
		}
		return fmt.Errorf("failed to query token: %w", err)
	}

	if t.IsMaster && !isMaster {
		return errors.New("permission denied: cannot delete master token")
	}

	if err := s.db.Delete(&t).Error; err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	authz.InvalidateTokenCache(targetID)
	return nil
}

// UpdateToken updates token permissions (requires master token)
func (s *TokenService) UpdateToken(targetID string, scopes string, expiresAt *time.Time) (*dto.TokenObject, error) {
	var t models.Token
	if err := s.db.Where("id = ?", targetID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("token not found")
		}
		return nil, fmt.Errorf("failed to query token: %w", err)
	}

	if t.IsMaster {
		return nil, errors.New("cannot modify master token permissions")
	}

	updates := map[string]interface{}{
		"scopes":     scopes,
		"expires_at": expiresAt,
	}
	if err := s.db.Model(&t).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update token: %w", err)
	}

	// Re-query to return latest data
	if err := s.db.Where("id = ?", targetID).First(&t).Error; err != nil {
		return nil, fmt.Errorf("failed to query updated token: %w", err)
	}
	authz.InvalidateTokenCache(targetID)
	resp := dto.TokenObject{
		ID:        t.ID,
		Name:      t.Name,
		IsMaster:  t.IsMaster,
		Scopes:    t.Scopes,
		ExpiresAt: t.ExpiresAt,
	}
	return &resp, nil
}
