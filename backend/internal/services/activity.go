package services

import (
	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// ActivityService 活动日志服务
type ActivityService struct {
	db *gorm.DB
}

// NewActivityService 创建活动日志服务实例
func NewActivityService(db *gorm.DB) *ActivityService {
	return &ActivityService{db: db}
}

// LogActivity 记录活动日志
func (s *ActivityService) LogActivity(userID, action, resourceType, resourceID, description string) error {
	log := models.ActivityLog{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Description:  description,
	}
	return s.db.Create(&log).Error
}

// GetRecentActivities 获取最近活动
func (s *ActivityService) GetRecentActivities(userID string, limit int) ([]models.ActivityLog, error) {
	var logs []models.ActivityLog
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
