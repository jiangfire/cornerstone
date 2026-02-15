package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// PluginService 插件服务
type PluginService struct {
	db *gorm.DB
}

// NewPluginService 创建插件服务实例
func NewPluginService(db *gorm.DB) *PluginService {
	return &PluginService{db: db}
}

// CreatePluginRequest 创建插件请求
type CreatePluginRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=255"`
	Description  string `json:"description" binding:"max=500"`
	Language     string `json:"language" binding:"required,oneof=go python bash"`
	EntryFile    string `json:"entry_file" binding:"required"`
	Timeout      int    `json:"timeout" binding:"min=1,max=300"`
	Config       string `json:"config" binding:"omitempty"`        // JSON config schema
	ConfigValues string `json:"config_values" binding:"omitempty"` // JSON config values
}

// UpdatePluginRequest 更新插件请求
type UpdatePluginRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=255"`
	Description  string `json:"description" binding:"max=500"`
	Timeout      int    `json:"timeout" binding:"min=1,max=300"`
	Config       string `json:"config" binding:"omitempty"`
	ConfigValues string `json:"config_values" binding:"omitempty"`
}

// CreatePlugin 创建插件
func (s *PluginService) CreatePlugin(req CreatePluginRequest, userID string) (*models.Plugin, error) {
	// 检查插件名称是否已存在
	var existing models.Plugin
	if err := s.db.Where("name = ? AND created_by = ?", req.Name, userID).First(&existing).Error; err == nil {
		return nil, errors.New("插件名称已存在")
	}

	plugin := models.Plugin{
		Name:         req.Name,
		Description:  req.Description,
		Language:     req.Language,
		EntryFile:    req.EntryFile,
		Timeout:      req.Timeout,
		Config:       req.Config,
		ConfigValues: req.ConfigValues,
		CreatedBy:    userID,
	}

	if err := s.db.Create(&plugin).Error; err != nil {
		return nil, fmt.Errorf("创建插件失败: %w", err)
	}

	return &plugin, nil
}

// ListPlugins 列出插件
func (s *PluginService) ListPlugins(userID string) ([]models.Plugin, error) {
	var plugins []models.Plugin
	if err := s.db.Where("created_by = ?", userID).Find(&plugins).Error; err != nil {
		return nil, fmt.Errorf("查询插件列表失败: %w", err)
	}
	return plugins, nil
}

// GetPlugin 获取插件详情
func (s *PluginService) GetPlugin(pluginID string) (*models.Plugin, error) {
	var plugin models.Plugin
	if err := s.db.Where("id = ?", pluginID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件不存在")
		}
		return nil, fmt.Errorf("查询插件失败: %w", err)
	}
	return &plugin, nil
}

// UpdatePlugin 更新插件
func (s *PluginService) UpdatePlugin(pluginID string, req UpdatePluginRequest) error {
	var plugin models.Plugin
	if err := s.db.Where("id = ?", pluginID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("插件不存在")
		}
		return fmt.Errorf("查询插件失败: %w", err)
	}

	plugin.Name = req.Name
	plugin.Description = req.Description
	plugin.Timeout = req.Timeout
	plugin.Config = req.Config
	plugin.ConfigValues = req.ConfigValues

	if err := s.db.Save(&plugin).Error; err != nil {
		return fmt.Errorf("更新插件失败: %w", err)
	}

	return nil
}

// DeletePlugin 删除插件
func (s *PluginService) DeletePlugin(pluginID string) error {
	result := s.db.Where("id = ?", pluginID).Delete(&models.Plugin{})
	if result.Error != nil {
		return fmt.Errorf("删除插件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("插件不存在")
	}
	return nil
}

// BindPlugin 绑定插件到表
func (s *PluginService) BindPlugin(pluginID, tableID, trigger string) error {
	// 验证插件是否存在
	var plugin models.Plugin
	if err := s.db.Where("id = ?", pluginID).First(&plugin).Error; err != nil {
		return errors.New("插件不存在")
	}

	// 验证表是否存在
	var table models.Table
	if err := s.db.Where("id = ?", tableID).First(&table).Error; err != nil {
		return errors.New("表不存在")
	}

	// 创建绑定
	binding := models.PluginBinding{
		PluginID: pluginID,
		TableID:  tableID,
		Trigger:  trigger,
	}

	if err := s.db.Create(&binding).Error; err != nil {
		return fmt.Errorf("绑定插件失败: %w", err)
	}

	return nil
}

// UnbindPlugin 解绑插件
func (s *PluginService) UnbindPlugin(pluginID, tableID string) error {
	result := s.db.Where("plugin_id = ? AND table_id = ?", pluginID, tableID).Delete(&models.PluginBinding{})
	if result.Error != nil {
		return fmt.Errorf("解绑插件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("绑定关系不存在")
	}
	return nil
}

// BindingDetail 绑定详情
type BindingDetail struct {
	ID           string    `json:"id"`
	TableID      string    `json:"table_id"`
	TableName    string    `json:"table_name"`
	DatabaseID   string    `json:"database_id"`
	DatabaseName string    `json:"database_name"`
	Trigger      string    `json:"trigger"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListBindings 列出插件的所有绑定
func (s *PluginService) ListBindings(pluginID string) ([]BindingDetail, error) {
	var bindings []models.PluginBinding
	if err := s.db.Where("plugin_id = ?", pluginID).Find(&bindings).Error; err != nil {
		return nil, fmt.Errorf("查询绑定失败: %w", err)
	}

	var details []BindingDetail
	for _, binding := range bindings {
		var table models.Table
		if err := s.db.Where("id = ?", binding.TableID).First(&table).Error; err != nil {
			continue
		}

		var database models.Database
		s.db.Where("id = ?", table.DatabaseID).First(&database)

		details = append(details, BindingDetail{
			ID:           binding.ID,
			TableID:      binding.TableID,
			TableName:    table.Name,
			DatabaseID:   table.DatabaseID,
			DatabaseName: database.Name,
			Trigger:      binding.Trigger,
			CreatedAt:    binding.CreatedAt,
		})
	}

	return details, nil
}
