package services

import (
	"strconv"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// StatsService 统计服务
type StatsService struct {
	db *gorm.DB
}

// NewStatsService 创建统计服务实例
func NewStatsService(db *gorm.DB) *StatsService {
	return &StatsService{db: db}
}

// StatsSummary 统计摘要
type StatsSummary struct {
	Users         int64 `json:"users"`
	Organizations int64 `json:"organizations"`
	Databases     int64 `json:"databases"`
	Plugins       int64 `json:"plugins"`
}

// Activity 活动
type Activity struct {
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
	Type    string    `json:"type"` // primary, success, warning, danger, info
}

// GetSummary 获取统计数据
func (s *StatsService) GetSummary(userID string) (*StatsSummary, error) {
	var stats StatsSummary

	// Count users
	s.db.Model(&models.User{}).Count(&stats.Users)

	// Count organizations where user is owner or member
	s.db.Table("organizations").
		Where("owner_id = ? OR id IN (SELECT organization_id FROM organization_members WHERE user_id = ?)", userID, userID).
		Count(&stats.Organizations)

	// Count databases accessible to user
	s.db.Table("databases").
		Where("owner_id = ? OR id IN (SELECT database_id FROM database_access WHERE user_id = ?)", userID, userID).
		Count(&stats.Databases)

	// Count plugins created by user
	s.db.Model(&models.Plugin{}).Where("created_by = ?", userID).Count(&stats.Plugins)

	return &stats, nil
}

// GetRecentActivities 获取最近活动
func (s *StatsService) GetRecentActivities(userID string, limitStr string) ([]Activity, error) {
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 10
	}

	activityService := NewActivityService(s.db)
	logs, err := activityService.GetRecentActivities(userID, limit)
	if err != nil {
		return nil, err
	}

	var activities []Activity
	for _, log := range logs {
		activities = append(activities, Activity{
			Content: log.Description,
			Time:    log.CreatedAt,
			Type:    getActivityType(log.Action),
		})
	}

	return activities, nil
}

// getActivityType 根据操作类型返回活动标签类型
func getActivityType(action string) string {
	types := map[string]string{
		"create": "success",
		"update": "warning",
		"delete": "danger",
	}
	if t, ok := types[action]; ok {
		return t
	}
	return "primary"
}
