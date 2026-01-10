package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// FieldService 字段管理服务
type FieldService struct {
	db *gorm.DB
}

// NewFieldService 创建字段服务实例
func NewFieldService(db *gorm.DB) *FieldService {
	return &FieldService{db: db}
}

// validateFieldName 验证字段名称
func validateFieldName(name string) error {
	name = strings.TrimSpace(name)

	if len(name) < 1 || len(name) > 255 {
		return errors.New("字段名称长度必须在1-255个字符之间")
	}

	// 只允许字母、数字、下划线
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, name)
	if !matched {
		return errors.New("字段名称只能包含字母、数字和下划线")
	}

	// 不能以数字开头
	if matched, _ := regexp.MatchString(`^[0-9]`, name); matched {
		return errors.New("字段名称不能以数字开头")
	}

	return nil
}

// validateFieldType 验证字段类型
func validateFieldType(fieldType string) error {
	validTypes := []string{"string", "number", "boolean", "date", "datetime", "single_select", "multi_select"}
	for _, validType := range validTypes {
		if fieldType == validType {
			return nil
		}
	}
	return fmt.Errorf("无效的字段类型: %s", fieldType)
}

// validateFieldConfig 验证字段配置
func validateFieldConfig(config FieldConfig) error {
	// 验证选项数量限制
	if len(config.Options) > 100 {
		return errors.New("选项数量不能超过100个")
	}

	// 验证选项值长度
	for _, option := range config.Options {
		if len(option) > 255 {
			return errors.New("选项值长度不能超过255个字符")
		}
	}

	// 验证数值范围
	if config.Min != nil && config.Max != nil && *config.Min > *config.Max {
		return errors.New("最小值不能大于最大值")
	}

	// 验证最大长度
	if config.MaxLength != nil && *config.MaxLength < 1 {
		return errors.New("最大长度必须大于0")
	}

	// 验证正则表达式
	if config.Validation != "" {
		_, err := regexp.Compile(config.Validation)
		if err != nil {
			return fmt.Errorf("无效的正则表达式: %w", err)
		}
	}

	return nil
}

// sanitizeFieldName 清理字段名称
func sanitizeFieldName(name string) string {
	name = strings.TrimSpace(name)
	// 移除危险字符
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "'", "")
	return name
}

// sanitizeFieldConfig 清理字段配置
func sanitizeFieldConfig(config FieldConfig) FieldConfig {
	// 清理选项
	cleanedOptions := make([]string, 0, len(config.Options))
	for _, option := range config.Options {
		cleanedOption := strings.TrimSpace(option)
		if cleanedOption != "" {
			// 移除危险字符
			cleanedOption = strings.ReplaceAll(cleanedOption, "<", "")
			cleanedOption = strings.ReplaceAll(cleanedOption, ">", "")
			cleanedOption = strings.ReplaceAll(cleanedOption, "\"", "")
			cleanedOption = strings.ReplaceAll(cleanedOption, "'", "")
			cleanedOptions = append(cleanedOptions, cleanedOption)
		}
	}
	config.Options = cleanedOptions

	// 清理正则表达式
	config.Validation = strings.TrimSpace(config.Validation)

	return config
}

// FieldConfig 字段配置
type FieldConfig struct {
	Options    []string `json:"options,omitempty"`    // 单选/多选选项
	Required   bool     `json:"required,omitempty"`   // 是否必填
	Min        *float64 `json:"min,omitempty"`        // 最小值
	Max        *float64 `json:"max,omitempty"`        // 最大值
	Format     string   `json:"format,omitempty"`     // 日期格式等
	MaxLength  *int     `json:"max_length,omitempty"` // 最大长度
	Validation string   `json:"validation,omitempty"` // 正则验证
}

// CreateFieldRequest 创建字段请求
type CreateFieldRequest struct {
	TableID   string      `json:"table_id" binding:"required"`
	Name      string      `json:"name" binding:"required,min=1,max=255"`
	Type      string      `json:"type" binding:"required,oneof=string number boolean date datetime single_select multi_select"`
	Required  bool        `json:"required"`
	Config    FieldConfig `json:"config"`
}

// UpdateFieldRequest 更新字段请求
type UpdateFieldRequest struct {
	Name      string      `json:"name" binding:"required,min=1,max=255"`
	Type      string      `json:"type" binding:"required,oneof=string number boolean date datetime single_select multi_select"`
	Required  bool        `json:"required"`
	Config    FieldConfig `json:"config"`
}

// FieldResponse 字段响应
type FieldResponse struct {
	ID        string      `json:"id"`
	TableID   string      `json:"table_id"`
	Name      string      `json:"name"`
	Type      string      `json:"type"`
	Required  bool        `json:"required"`
	Config    FieldConfig `json:"config"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

// checkTableAccess 检查表访问权限
func (s *FieldService) checkTableAccess(tableID, userID string, requiredRoles []string) error {
	var table models.Table
	err := s.db.Where("id = ?", tableID).First(&table).Error
	if err != nil {
		return errors.New("表不存在")
	}

	var access models.DatabaseAccess
	err = s.db.Where("database_id = ? AND user_id = ?", table.DatabaseID, userID).First(&access).Error
	if err != nil {
		return errors.New("无权访问该数据库")
	}

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

// CreateField 创建字段
func (s *FieldService) CreateField(req CreateFieldRequest, userID string) (*models.Field, error) {
	// 1. 检查表访问权限（owner, admin, editor可以创建字段）
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 2. 输入验证和清理
	req.Name = sanitizeFieldName(req.Name)
	req.Config = sanitizeFieldConfig(req.Config)

	if err := validateFieldName(req.Name); err != nil {
		return nil, fmt.Errorf("字段名称验证失败: %w", err)
	}

	if err := validateFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("字段类型验证失败: %w", err)
	}

	if err := validateFieldConfig(req.Config); err != nil {
		return nil, fmt.Errorf("字段配置验证失败: %w", err)
	}

	// 3. 检查是否已存在同名字段
	var existingField models.Field
	err := s.db.Where("table_id = ? AND name = ?", req.TableID, req.Name).First(&existingField).Error
	if err == nil {
		return nil, errors.New("该表中已存在同名字段")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 4. 序列化配置
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("配置序列化失败: %w", err)
	}

	// 5. 创建字段
	field := models.Field{
		TableID:   req.TableID,
		Name:      req.Name,
		Type:      req.Type,
		Required:  req.Required,
		Options:   string(configJSON),
	}

	if err := s.db.Create(&field).Error; err != nil {
		return nil, fmt.Errorf("创建字段失败: %w", err)
	}

	return &field, nil
}

// ListFields 获取字段列表
func (s *FieldService) ListFields(tableID, userID string) ([]FieldResponse, error) {
	// 1. 检查表访问权限
	if err := s.checkTableAccess(tableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	// 2. 查询字段列表
	var fields []models.Field
	err := s.db.Where("table_id = ? AND deleted_at IS NULL", tableID).Order("created_at ASC").Find(&fields).Error
	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 3. 转换为响应格式
	result := make([]FieldResponse, len(fields))
	for i, f := range fields {
		var config FieldConfig
		if f.Options != "" {
			json.Unmarshal([]byte(f.Options), &config)
		}

		result[i] = FieldResponse{
			ID:        f.ID,
			TableID:   f.TableID,
			Name:      f.Name,
			Type:      f.Type,
			Required:  f.Required,
			Config:    config,
			CreatedAt: f.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: f.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return result, nil
}

// GetField 获取字段详情
func (s *FieldService) GetField(fieldID, userID string) (*FieldResponse, error) {
	// 1. 获取字段信息
	var field models.Field
	err := s.db.Where("id = ?", fieldID).First(&field).Error
	if err != nil {
		return nil, fmt.Errorf("字段不存在: %w", err)
	}

	// 2. 检查表访问权限
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	// 3. 解析配置
	var config FieldConfig
	if field.Options != "" {
		json.Unmarshal([]byte(field.Options), &config)
	}

	return &FieldResponse{
		ID:        field.ID,
		TableID:   field.TableID,
		Name:      field.Name,
		Type:      field.Type,
		Required:  field.Required,
		Config:    config,
		CreatedAt: field.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: field.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateField 更新字段信息
func (s *FieldService) UpdateField(fieldID string, req UpdateFieldRequest, userID string) (*models.Field, error) {
	// 1. 获取字段信息
	var field models.Field
	err := s.db.Where("id = ?", fieldID).First(&field).Error
	if err != nil {
		return nil, fmt.Errorf("字段不存在: %w", err)
	}

	// 2. 检查表访问权限（只有owner, admin, editor可以修改）
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 3. 输入验证和清理
	req.Name = sanitizeFieldName(req.Name)
	req.Config = sanitizeFieldConfig(req.Config)

	if err := validateFieldName(req.Name); err != nil {
		return nil, fmt.Errorf("字段名称验证失败: %w", err)
	}

	if err := validateFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("字段类型验证失败: %w", err)
	}

	if err := validateFieldConfig(req.Config); err != nil {
		return nil, fmt.Errorf("字段配置验证失败: %w", err)
	}

	// 4. 检查是否已存在同名字段（排除当前字段）
	var existingField models.Field
	err = s.db.Where("table_id = ? AND name = ? AND id != ?", field.TableID, req.Name, fieldID).First(&existingField).Error
	if err == nil {
		return nil, errors.New("该表中已存在同名字段")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 5. 序列化配置
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("配置序列化失败: %w", err)
	}

	// 6. 更新字段信息
	field.Name = req.Name
	field.Type = req.Type
	field.Required = req.Required
	field.Options = string(configJSON)

	if err := s.db.Save(&field).Error; err != nil {
		return nil, fmt.Errorf("更新字段失败: %w", err)
	}

	return &field, nil
}

// DeleteField 删除字段（软删除）
func (s *FieldService) DeleteField(fieldID, userID string) error {
	// 1. 获取字段信息
	var field models.Field
	err := s.db.Where("id = ?", fieldID).First(&field).Error
	if err != nil {
		return fmt.Errorf("字段不存在: %w", err)
	}

	// 2. 检查表访问权限（只有owner, admin可以删除）
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin"}); err != nil {
		return err
	}

	// 3. 软删除字段
	if err := s.db.Delete(&field).Error; err != nil {
		return fmt.Errorf("删除字段失败: %w", err)
	}

	return nil
}
