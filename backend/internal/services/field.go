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

// validateFieldType 验证字段类型
func validateFieldType(fieldType string) error {
	validTypes := []string{"string", "number", "boolean", "date", "datetime", "select", "multiselect", "single_select", "multi_select"}
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
	Type      string      `json:"type" binding:"required,oneof=string number boolean date datetime select multiselect single_select multi_select"`
	Required  bool        `json:"required"`
	Options   string      `json:"options"` // 下拉选项，逗号分隔
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

	// 2. 如果前端提供了options字符串，转换为Config
	if req.Options != "" && (req.Type == "select" || req.Type == "multiselect") {
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

	// 4. 检查是否已存在同名字段
	var existingField models.Field
	err := s.db.Where("table_id = ? AND name = ?", req.TableID, req.Name).First(&existingField).Error
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

// FieldPermissionRequest 字段权限请求
type FieldPermissionRequest struct {
	FieldID   string `json:"field_id" binding:"required"`
	Role      string `json:"role" binding:"required,oneof=owner admin editor viewer"`
	CanRead   bool   `json:"can_read"`
	CanWrite  bool   `json:"can_write"`
	CanDelete bool   `json:"can_delete"`
}

// BatchFieldPermissionsRequest 批量字段权限请求
type BatchFieldPermissionsRequest struct {
	Permissions []FieldPermissionRequest `json:"permissions" binding:"required"`
}

// getUserRole 获取用户在表所属数据库中的角色
func (s *FieldService) getUserRole(tableID, userID string) (string, error) {
	var table models.Table
	err := s.db.Where("id = ?", tableID).First(&table).Error
	if err != nil {
		return "", fmt.Errorf("表不存在: %w", err)
	}

	var access models.DatabaseAccess
	err = s.db.Where("database_id = ? AND user_id = ?", table.DatabaseID, userID).First(&access).Error
	if err != nil {
		return "", errors.New("无权访问该数据库")
	}

	return access.Role, nil
}

// CheckFieldPermission 检查用户对特定字段的权限
func (s *FieldService) CheckFieldPermission(userID, fieldID, action string) error {
	// 1. 获取字段信息
	var field models.Field
	err := s.db.Where("id = ?", fieldID).First(&field).Error
	if err != nil {
		return fmt.Errorf("字段不存在: %w", err)
	}

	// 2. 获取用户角色
	role, err := s.getUserRole(field.TableID, userID)
	if err != nil {
		return err
	}

	// 3. 查询字段级权限配置
	var permission models.FieldPermission
	err = s.db.Where("field_id = ? AND role = ?", fieldID, role).First(&permission).Error

	// 如果没有配置字段级权限，使用表级默认权限
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// owner 和 admin 默认有所有权限
		// editor 默认有读和写权限
		// viewer 默认只有读权限
		switch role {
		case "owner", "admin":
			return nil
		case "editor":
			if action == "delete" {
				return errors.New("无删除权限")
			}
			return nil
		case "viewer":
			if action == "read" {
				return nil
			}
			return errors.New("只读用户无此权限")
		}
	}

	if err != nil {
		return fmt.Errorf("权限查询失败: %w", err)
	}

	// 4. 检查具体操作权限
	switch action {
	case "read":
		if !permission.CanRead {
			return errors.New("无读取权限")
		}
	case "write":
		if !permission.CanWrite {
			return errors.New("无写入权限")
		}
	case "delete":
		if !permission.CanDelete {
			return errors.New("无删除权限")
		}
	default:
		return errors.New("无效的操作类型")
	}

	return nil
}

// GetFieldPermissions 获取表的字段权限配置
func (s *FieldService) GetFieldPermissions(tableID, userID string) ([]models.FieldPermission, error) {
	// 1. 检查表访问权限（只有owner, admin可以查看权限配置）
	if err := s.checkTableAccess(tableID, userID, []string{"owner", "admin"}); err != nil {
		return nil, err
	}

	// 2. 查询字段权限配置
	var permissions []models.FieldPermission
	err := s.db.Where("table_id = ?", tableID).Find(&permissions).Error
	if err != nil {
		return nil, fmt.Errorf("查询字段权限失败: %w", err)
	}

	return permissions, nil
}

// SetFieldPermission 设置单个字段的权限
func (s *FieldService) SetFieldPermission(tableID string, req FieldPermissionRequest, userID string) error {
	// 1. 检查表访问权限（只有owner, admin可以设置权限）
	if err := s.checkTableAccess(tableID, userID, []string{"owner", "admin"}); err != nil {
		return err
	}

	// 2. 验证字段是否存在
	var field models.Field
	err := s.db.Where("id = ? AND table_id = ?", req.FieldID, tableID).First(&field).Error
	if err != nil {
		return errors.New("字段不存在")
	}

	// 3. 查找是否已存在权限配置
	var permission models.FieldPermission
	err = s.db.Where("field_id = ? AND role = ?", req.FieldID, req.Role).First(&permission).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新权限配置
		permission = models.FieldPermission{
			TableID:   tableID,
			FieldID:   req.FieldID,
			Role:      req.Role,
			CanRead:   req.CanRead,
			CanWrite:  req.CanWrite,
			CanDelete: req.CanDelete,
		}
		if err := s.db.Create(&permission).Error; err != nil {
			return fmt.Errorf("创建权限配置失败: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("查询权限配置失败: %w", err)
	} else {
		// 更新已有权限配置
		permission.CanRead = req.CanRead
		permission.CanWrite = req.CanWrite
		permission.CanDelete = req.CanDelete
		if err := s.db.Save(&permission).Error; err != nil {
			return fmt.Errorf("更新权限配置失败: %w", err)
		}
	}

	return nil
}

// BatchSetFieldPermissions 批量设置字段权限
func (s *FieldService) BatchSetFieldPermissions(tableID string, req BatchFieldPermissionsRequest, userID string) error {
	// 1. 检查表访问权限（只有owner, admin可以设置权限）
	if err := s.checkTableAccess(tableID, userID, []string{"owner", "admin"}); err != nil {
		return err
	}

	// 2. 使用事务批量更新权限
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, permReq := range req.Permissions {
			// 验证字段是否存在
			var field models.Field
			err := tx.Where("id = ? AND table_id = ?", permReq.FieldID, tableID).First(&field).Error
			if err != nil {
				return fmt.Errorf("字段 %s 不存在", permReq.FieldID)
			}

			// 查找是否已存在权限配置
			var permission models.FieldPermission
			err = tx.Where("field_id = ? AND role = ?", permReq.FieldID, permReq.Role).First(&permission).Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 创建新权限配置
				permission = models.FieldPermission{
					TableID:   tableID,
					FieldID:   permReq.FieldID,
					Role:      permReq.Role,
					CanRead:   permReq.CanRead,
					CanWrite:  permReq.CanWrite,
					CanDelete: permReq.CanDelete,
				}
				if err := tx.Create(&permission).Error; err != nil {
					return fmt.Errorf("创建权限配置失败: %w", err)
				}
			} else if err != nil {
				return fmt.Errorf("查询权限配置失败: %w", err)
			} else {
				// 更新已有权限配置
				permission.CanRead = permReq.CanRead
				permission.CanWrite = permReq.CanWrite
				permission.CanDelete = permReq.CanDelete
				if err := tx.Save(&permission).Error; err != nil {
					return fmt.Errorf("更新权限配置失败: %w", err)
				}
			}
		}
		return nil
	})
}
