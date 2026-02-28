package services

import (
	"errors"
	"fmt"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SettingsService 系统设置服务
type SettingsService struct {
	db *gorm.DB
}

// NewSettingsService 创建系统设置服务实例
func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{db: db}
}

// UpdateSettingsRequest 更新系统设置请求
type UpdateSettingsRequest struct {
	SystemName        string `json:"system_name" binding:"required,min=1,max=255"`
	SystemDescription string `json:"system_description" binding:"max=2000"`
	AllowRegistration bool   `json:"allow_registration"`
	MaxFileSize       int    `json:"max_file_size" binding:"min=1,max=1024"`
	DBType            string `json:"db_type" binding:"required,oneof=postgresql mysql sqlite"`
	DBPoolSize        int    `json:"db_pool_size" binding:"min=1,max=200"`
	DBTimeout         int    `json:"db_timeout" binding:"min=1,max=300"`
	PluginTimeout     int    `json:"plugin_timeout" binding:"min=1,max=600"`
	PluginWorkDir     string `json:"plugin_work_dir" binding:"required,max=1000"`
	PluginAutoUpdate  bool   `json:"plugin_auto_update"`
}

func defaultSettings() models.AppSettings {
	return models.AppSettings{
		ID:                1,
		SystemName:        "Cornerstone",
		SystemDescription: "数据管理平台",
		AllowRegistration: true,
		MaxFileSize:       50,
		DBType:            "postgresql",
		DBPoolSize:        10,
		DBTimeout:         30,
		PluginTimeout:     300,
		PluginWorkDir:     "./plugins",
		PluginAutoUpdate:  false,
	}
}

func (s *SettingsService) ensureSettings() (*models.AppSettings, error) {
	var settings models.AppSettings
	if err := s.db.Where("id = 1").First(&settings).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询系统设置失败: %w", err)
		}

		settings = defaultSettings()
		if err := s.db.Create(&settings).Error; err != nil {
			return nil, fmt.Errorf("初始化系统设置失败: %w", err)
		}
	}

	return &settings, nil
}

// GetSettings 获取系统设置
func (s *SettingsService) GetSettings() (*models.AppSettings, error) {
	return s.ensureSettings()
}

// UpdateSettings 更新系统设置
func (s *SettingsService) UpdateSettings(req UpdateSettingsRequest, userID string) (*models.AppSettings, error) {
	current, err := s.ensureSettings()
	if err != nil {
		return nil, err
	}

	current.SystemName = req.SystemName
	current.SystemDescription = req.SystemDescription
	current.AllowRegistration = req.AllowRegistration
	current.MaxFileSize = req.MaxFileSize
	current.DBType = req.DBType
	current.DBPoolSize = req.DBPoolSize
	current.DBTimeout = req.DBTimeout
	current.PluginTimeout = req.PluginTimeout
	current.PluginWorkDir = req.PluginWorkDir
	current.PluginAutoUpdate = req.PluginAutoUpdate
	current.UpdatedBy = userID

	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(current).Error; err != nil {
		return nil, fmt.Errorf("更新系统设置失败: %w", err)
	}

	return current, nil
}

// GetPluginRuntimeConfig 获取插件运行时默认配置
func (s *SettingsService) GetPluginRuntimeConfig() (timeoutSec int, workDir string, err error) {
	settings, err := s.ensureSettings()
	if err != nil {
		return 300, "./plugins", err
	}

	timeoutSec = settings.PluginTimeout
	if timeoutSec <= 0 {
		timeoutSec = 300
	}

	workDir = settings.PluginWorkDir
	if workDir == "" {
		workDir = "./plugins"
	}

	return timeoutSec, workDir, nil
}
