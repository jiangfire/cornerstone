package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
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

	// 支持字母（包括中文）、数字、下划线
	// \p{L} 匹配所有语言的字母（包括中文）
	// \p{N} 匹配所有语言的数字
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_]+$`, name)
	if !matched {
		return errors.New("字段名称只能包含字母、数字和下划线")
	}

	// 不能以ASCII数字开头
	if matched, _ := regexp.MatchString(`^[0-9]`, name); matched {
		return errors.New("字段名称不能以数字开头")
	}

	return nil
}

func normalizeFieldType(fieldType string) string {
	return fieldType
}

func isDeprecatedFieldType(fieldType string) bool {
	return false
}

func isAttachmentFieldType(fieldType string) bool {
	return fieldType == "file"
}

func supportsFieldOptions(fieldType string) bool {
	return fieldType == "list"
}

// validateFieldType 验证字段类型
func validateFieldType(fieldType string) error {
	validTypes := []string{"string", "text", "number", "boolean", "date", "datetime", "file", "json", "list"}
	for _, validType := range validTypes {
		if fieldType == validType {
			return nil
		}
	}
	return fmt.Errorf("无效的字段类型: %s", fieldType)
}

func validateMutableFieldType(fieldType string) error { return nil }

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

	if len(config.AllowedTypes) > 50 {
		return errors.New("允许的文件类型数量不能超过50个")
	}

	for _, allowedType := range config.AllowedTypes {
		if len(allowedType) > 100 {
			return errors.New("文件类型规则长度不能超过100个字符")
		}
	}

	if config.MaxFileSizeMB < 0 {
		return errors.New("附件大小限制不能小于0")
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

	cleanedAllowedTypes := make([]string, 0, len(config.AllowedTypes))
	for _, allowedType := range config.AllowedTypes {
		cleanedType := strings.TrimSpace(allowedType)
		if cleanedType != "" {
			cleanedAllowedTypes = append(cleanedAllowedTypes, cleanedType)
		}
	}
	config.AllowedTypes = cleanedAllowedTypes

	return config
}

func sanitizeFieldDescription(description string) string {
	return strings.TrimSpace(description)
}

func validateFieldDescription(description string) error {
	if len(description) > 1000 {
		return errors.New("字段备注长度不能超过1000个字符")
	}
	return nil
}

// FieldConfig 字段配置
type FieldConfig struct {
	Options       []string `json:"options,omitempty"`          // 列表建议项
	Required      bool     `json:"required,omitempty"`         // 是否必填
	Min           *float64 `json:"min,omitempty"`              // 最小值
	Max           *float64 `json:"max,omitempty"`              // 最大值
	Format        string   `json:"format,omitempty"`           // 日期格式等
	MaxLength     *int     `json:"max_length,omitempty"`       // 最大长度
	Validation    string   `json:"validation,omitempty"`       // 正则验证
	AllowedTypes  []string `json:"allowed_types,omitempty"`    // 允许的附件类型，如 image/*、.pdf
	MaxFileSizeMB int      `json:"max_file_size_mb,omitempty"` // 附件大小上限（MB）
	Multiple      bool     `json:"multiple,omitempty"`         // 是否允许多个附件
}

// CreateFieldRequest 创建字段请求
type CreateFieldRequest struct {
	TableID     string      `json:"table_id" binding:"required"`
	Name        string      `json:"name" binding:"required,min=1,max=255"`
	Type        string      `json:"type" binding:"required"`
	Description string      `json:"description" binding:"max=1000"`
	Required    bool        `json:"required"`
	Options     string      `json:"options"` // 列表选项，逗号分隔
	Config      FieldConfig `json:"config"`
}

// UpdateFieldRequest 更新字段请求
type UpdateFieldRequest struct {
	Name        string      `json:"name" binding:"required,min=1,max=255"`
	Type        string      `json:"type" binding:"required"`
	Description string      `json:"description" binding:"max=1000"`
	Required    bool        `json:"required"`
	Options     string      `json:"options"`
	Config      FieldConfig `json:"config"`
}

// FieldResponse 字段响应
type FieldResponse struct {
	ID          string      `json:"id"`
	TableID     string      `json:"table_id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Deprecated  bool        `json:"deprecated"`
	Required    bool        `json:"required"`
	Options     string      `json:"options,omitempty"`
	Config      FieldConfig `json:"config"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

func buildDeletedFieldName(name, fieldID string) string {
	suffix := "__deleted__" + fieldID
	maxPrefixLen := 255 - len(suffix)
	if maxPrefixLen < 0 {
		maxPrefixLen = 0
	}
	if len(name) > maxPrefixLen {
		name = name[:maxPrefixLen]
	}
	return name + suffix
}

func (s *FieldService) getActiveTable(tableID string) (*models.Table, error) {
	var table models.Table
	err := s.db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error
	if err != nil {
		return nil, err
	}
	return &table, nil
}

func (s *FieldService) getActiveField(fieldID string) (*models.Field, error) {
	var field models.Field
	err := s.db.Where("id = ? AND deleted_at IS NULL", fieldID).First(&field).Error
	if err != nil {
		return nil, err
	}
	return &field, nil
}

// checkTableAccess 检查表是否存在
func (s *FieldService) checkTableAccess(tableID, userID string, requiredRoles []string) error {
	table, err := s.getActiveTable(tableID)
	if err != nil {
		return errors.New("表不存在")
	}

	var db models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", table.DatabaseID).First(&db).Error
	if err != nil {
		return errors.New("数据库不存在")
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	action := authz.ActionRead
	switch {
	case containsRole(requiredRoles, "owner") || containsRole(requiredRoles, "admin"):
		action = authz.ActionManage
	case containsRole(requiredRoles, "editor"):
		action = authz.ActionWrite
	}
	if !authorizer.CanAccessTable(tableID, action) {
		return errors.New("无权访问该表")
	}

	return nil
}

func containsRole(roles []string, role string) bool {
	for _, candidate := range roles {
		if strings.EqualFold(candidate, role) {
			return true
		}
	}
	return false
}

// CreateField 创建字段
func (s *FieldService) CreateField(req CreateFieldRequest, userID string) (*models.Field, error) {
	// 1. 检查表访问权限（owner, admin, editor可以创建字段）
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 2. 如果前端提供了options字符串，转换为Config
	if req.Options != "" && supportsFieldOptions(req.Type) {
		// 将逗号分隔的字符串转换为字符串数组
		optionsList := strings.Split(req.Options, ",")
		var cleanedOptions []string
		for _, opt := range optionsList {
			opt = strings.TrimSpace(opt)
			if opt != "" {
				cleanedOptions = append(cleanedOptions, opt)
			}
		}
		req.Config.Options = cleanedOptions
	}

	// 3. 输入验证和清理
	req.Name = sanitizeFieldName(req.Name)
	req.Description = sanitizeFieldDescription(req.Description)
	req.Config = sanitizeFieldConfig(req.Config)

	if err := validateFieldName(req.Name); err != nil {
		return nil, fmt.Errorf("字段名称验证失败: %w", err)
	}

	if err := validateFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("字段类型验证失败: %w", err)
	}
	if err := validateMutableFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("字段类型验证失败: %w", err)
	}
	req.Type = normalizeFieldType(req.Type)

	if err := validateFieldDescription(req.Description); err != nil {
		return nil, fmt.Errorf("字段备注验证失败: %w", err)
	}

	if err := validateFieldConfig(req.Config); err != nil {
		return nil, fmt.Errorf("字段配置验证失败: %w", err)
	}

	// 4. 检查是否已存在同名字段
	var existingField models.Field
	err := s.db.Where("table_id = ? AND name = ? AND deleted_at IS NULL", req.TableID, req.Name).First(&existingField).Error
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

	// 6. 创建字段
	field := models.Field{
		TableID:     req.TableID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Required:    req.Required,
		Options:     string(configJSON),
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
	index := 0
	for _, f := range fields {
		if err := s.CheckFieldPermission(userID, f.ID, "read"); err != nil {
			continue
		}

		var config FieldConfig
		if f.Options != "" {
			_ = json.Unmarshal([]byte(f.Options), &config)
			// 配置已安全存储在数据库中，解析失败不影响核心功能
		}

		result[index] = FieldResponse{
			ID:          f.ID,
			TableID:     f.TableID,
			Name:        f.Name,
			Type:        normalizeFieldType(f.Type),
			Description: f.Description,
			Deprecated:  isDeprecatedFieldType(f.Type),
			Required:    f.Required,
			Options:     strings.Join(config.Options, ", "),
			Config:      config,
			CreatedAt:   f.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   f.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		index++
	}

	return result[:index], nil
}

// GetField 获取字段详情
func (s *FieldService) GetField(fieldID, userID string) (*FieldResponse, error) {
	// 1. 获取字段信息
	field, err := s.getActiveField(fieldID)
	if err != nil {
		return nil, fmt.Errorf("字段不存在: %w", err)
	}

	// 2. 检查表访问权限
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}
	if err := s.CheckFieldPermission(userID, field.ID, "read"); err != nil {
		return nil, err
	}

	// 3. 解析配置
	var config FieldConfig
	if field.Options != "" {
		_ = json.Unmarshal([]byte(field.Options), &config)
		// 配置已安全存储在数据库中，解析失败不影响核心功能
	}

	return &FieldResponse{
		ID:          field.ID,
		TableID:     field.TableID,
		Name:        field.Name,
		Type:        normalizeFieldType(field.Type),
		Description: field.Description,
		Deprecated:  isDeprecatedFieldType(field.Type),
		Required:    field.Required,
		Options:     strings.Join(config.Options, ", "),
		Config:      config,
		CreatedAt:   field.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   field.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateField 更新字段信息
func (s *FieldService) UpdateField(fieldID string, req UpdateFieldRequest, userID string) (*models.Field, error) {
	// 1. 获取字段信息
	field, err := s.getActiveField(fieldID)
	if err != nil {
		return nil, fmt.Errorf("字段不存在: %w", err)
	}

	// 2. 检查表访问权限（只有owner, admin, editor可以修改）
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 3. 输入验证和清理
	if req.Options != "" && supportsFieldOptions(req.Type) {
		optionsList := strings.Split(req.Options, ",")
		var cleanedOptions []string
		for _, opt := range optionsList {
			opt = strings.TrimSpace(opt)
			if opt != "" {
				cleanedOptions = append(cleanedOptions, opt)
			}
		}
		req.Config.Options = cleanedOptions
	}

	req.Name = sanitizeFieldName(req.Name)
	req.Description = sanitizeFieldDescription(req.Description)
	req.Config = sanitizeFieldConfig(req.Config)

	if err := validateFieldName(req.Name); err != nil {
		return nil, fmt.Errorf("字段名称验证失败: %w", err)
	}

	if err := validateFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("字段类型验证失败: %w", err)
	}
	if err := validateMutableFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("字段类型验证失败: %w", err)
	}
	req.Type = normalizeFieldType(req.Type)

	if err := validateFieldDescription(req.Description); err != nil {
		return nil, fmt.Errorf("字段备注验证失败: %w", err)
	}

	if err := validateFieldConfig(req.Config); err != nil {
		return nil, fmt.Errorf("字段配置验证失败: %w", err)
	}

	// 4. 检查是否已存在同名字段（排除当前字段）
	var existingField models.Field
	err = s.db.Where("table_id = ? AND name = ? AND id != ? AND deleted_at IS NULL", field.TableID, req.Name, fieldID).First(&existingField).Error
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
	field.Description = req.Description
	field.Required = req.Required
	field.Options = string(configJSON)

	if err := s.db.Save(field).Error; err != nil {
		return nil, fmt.Errorf("更新字段失败: %w", err)
	}

	return field, nil
}

// DeleteField 删除字段（软删除）
func (s *FieldService) DeleteField(fieldID, userID string) error {
	// 1. 获取字段信息
	field, err := s.getActiveField(fieldID)
	if err != nil {
		return fmt.Errorf("字段不存在: %w", err)
	}

	// 2. 检查表访问权限（只有owner, admin可以删除）
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin"}); err != nil {
		return err
	}

	// 3. 软删除字段
	now := time.Now()
	result := s.db.Model(&models.Field{}).
		Where("id = ? AND deleted_at IS NULL", fieldID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"name":       buildDeletedFieldName(field.Name, fieldID),
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("删除字段失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("字段不存在: %w", gorm.ErrRecordNotFound)
	}

	return nil
}

// CheckFieldPermission 检查用户对特定字段的权限
func (s *FieldService) CheckFieldPermission(userID, fieldID, action string) error {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	if !authorizer.CanAccessField(fieldID, action) {
		return errors.New("无权访问该字段")
	}
	return nil
}

// CheckFieldPermissions 批量检查字段权限，只查一次数据库。
func (s *FieldService) CheckFieldPermissions(userID string, fieldIDs []string, action string) (map[string]bool, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	return authorizer.CanAccessFields(fieldIDs, action), nil
}
