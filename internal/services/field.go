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
	"github.com/jiangfire/cornerstone/pkg/dto"
	"gorm.io/gorm"
)

// FieldService manages field operations
type FieldService struct {
	db *gorm.DB
}

// NewFieldService creates a new FieldService instance
func NewFieldService(db *gorm.DB) *FieldService {
	return &FieldService{db: db}
}

// ResolveField resolves a field identifier within a table to a field model.
// tableIdentifier can be a table ID or name.
// fieldIdentifier can be a field ID or name.
func (s *FieldService) ResolveField(tableIdentifier, fieldIdentifier string) (*models.Field, error) {
	// Resolve table first
	var table models.Table
	err := s.db.Where("id = ? AND deleted_at IS NULL", tableIdentifier).First(&table).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Try resolving via table name in any accessible database
			err = s.db.Where("name = ? AND deleted_at IS NULL", tableIdentifier).First(&table).Error
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("table not found")
			}
			return nil, fmt.Errorf("table query failed: %w", err)
		}
	}

	var field models.Field
	// Try field ID first
	err = s.db.Where("id = ? AND deleted_at IS NULL", fieldIdentifier).First(&field).Error
	if err == nil {
		if field.TableID != table.ID {
			return nil, errors.New("field does not belong to the specified table")
		}
		return &field, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("field query failed: %w", err)
	}
	// Fallback to field name within the table
	err = s.db.Where("table_id = ? AND name = ? AND deleted_at IS NULL", table.ID, fieldIdentifier).First(&field).Error
	if err == nil {
		return &field, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("field not found")
	}
	return nil, fmt.Errorf("field query failed: %w", err)
}

// validateFieldName validates field name format
func validateFieldName(name string) error {
	name = strings.TrimSpace(name)

	if len(name) < 1 || len(name) > 255 {
		return errors.New("field name must be between 1 and 255 characters")
	}

	// Allow letters (including CJK), numbers, underscores
	// \p{L} matches letters from all languages (including CJK)
	// \p{N} matches numbers from all languages
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_]+$`, name)
	if !matched {
		return errors.New("field name can only contain letters, numbers and underscores")
	}

	// Must not start with ASCII digit
	if matched, _ := regexp.MatchString(`^[0-9]`, name); matched {
		return errors.New("field name must not start with a digit")
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

// validateFieldType validates field type
func validateFieldType(fieldType string) error {
	validTypes := []string{"string", "text", "number", "boolean", "date", "datetime", "file", "json", "list"}
	for _, validType := range validTypes {
		if fieldType == validType {
			return nil
		}
	}
	return fmt.Errorf("invalid field type: %s", fieldType)
}

func validateMutableFieldType(fieldType string) error { return nil }

// validateFieldConfig validates field configuration
func validateFieldConfig(config dto.FieldConfig) error {
	// Validate options count
	if len(config.Options) > 100 {
		return errors.New("number of options must not exceed 100")
	}

	// Validate option value length
	for _, option := range config.Options {
		if len(option) > 255 {
			return errors.New("option value must not exceed 255 characters")
		}
	}

	// Validate numeric range
	if config.Min != nil && config.Max != nil && *config.Min > *config.Max {
		return errors.New("min value must not be greater than max value")
	}

	// Validate max length
	if config.MaxLength != nil && *config.MaxLength < 1 {
		return errors.New("max length must be greater than 0")
	}

	// Validate regex pattern
	if config.Validation != "" {
		_, err := regexp.Compile(config.Validation)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	if len(config.AllowedTypes) > 50 {
		return errors.New("number of allowed file types must not exceed 50")
	}

	for _, allowedType := range config.AllowedTypes {
		if len(allowedType) > 100 {
			return errors.New("file type rule must not exceed 100 characters")
		}
	}

	if config.MaxFileSizeMB < 0 {
		return errors.New("max file size must not be negative")
	}

	return nil
}

// sanitizeFieldName sanitizes field name
func sanitizeFieldName(name string) string {
	name = strings.TrimSpace(name)
	// Remove dangerous characters
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "'", "")
	return name
}

// sanitizeFieldConfig sanitizes field config
func sanitizeFieldConfig(config dto.FieldConfig) dto.FieldConfig {
	// Sanitize options
	cleanedOptions := make([]string, 0, len(config.Options))
	for _, option := range config.Options {
		cleanedOption := strings.TrimSpace(option)
		if cleanedOption != "" {
			// Remove dangerous characters
			cleanedOption = strings.ReplaceAll(cleanedOption, "<", "")
			cleanedOption = strings.ReplaceAll(cleanedOption, ">", "")
			cleanedOption = strings.ReplaceAll(cleanedOption, "\"", "")
			cleanedOption = strings.ReplaceAll(cleanedOption, "'", "")
			cleanedOptions = append(cleanedOptions, cleanedOption)
		}
	}
	config.Options = cleanedOptions

	// Sanitize regex
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
		return errors.New("field description must not exceed 1000 characters")
	}
	return nil
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

// checkTableAccess verifies table exists and user has access
func (s *FieldService) checkTableAccess(tableID, userID string, requiredRoles []string) error {
	table, err := s.getActiveTable(tableID)
	if err != nil {
		return errors.New("table not found")
	}

	var db models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", table.DatabaseID).First(&db).Error
	if err != nil {
		return errors.New("database not found")
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	action := requiredActionForRoles(requiredRoles)
	if !authorizer.CanAccessTable(tableID, action) {
		return errors.New("permission denied: cannot access this table")
	}

	return nil
}

// resolveTable resolves a table identifier to a table model.
// It first tries to find by ID, then falls back to name lookup.
func (s *FieldService) resolveTable(identifier string) (*models.Table, error) {
	var table models.Table
	err := s.db.Where("id = ? AND deleted_at IS NULL", identifier).First(&table).Error
	if err == nil {
		return &table, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("table query failed: %w", err)
	}
	err = s.db.Where("name = ? AND deleted_at IS NULL", identifier).First(&table).Error
	if err == nil {
		return &table, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("table not found")
	}
	return nil, fmt.Errorf("table query failed: %w", err)
}

func requiredActionForRoles(roles []string) string {
	switch {
	case containsRole(roles, "viewer"):
		return authz.ActionRead
	case containsRole(roles, "editor"):
		return authz.ActionWrite
	case containsRole(roles, "owner") || containsRole(roles, "admin"):
		return authz.ActionManage
	default:
		return authz.ActionRead
	}
}

func containsRole(roles []string, role string) bool {
	for _, candidate := range roles {
		if strings.EqualFold(candidate, role) {
			return true
		}
	}
	return false
}

// CreateField creates a new field
func (s *FieldService) CreateField(req dto.FieldCreateRequest, userID string) (*models.Field, error) {
	// 1. Resolve table identifier (supports ID or name)
	table, err := s.resolveTable(req.TableID)
	if err != nil {
		return nil, err
	}
	req.TableID = table.ID

	// 2. Check table access (owner, admin, editor can create fields)
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 3. Convert options string to Config if provided
	if req.Options != "" && supportsFieldOptions(req.Type) {
		// Convert comma-separated string to string array
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

	// 4. Input validation and sanitization
	req.Name = sanitizeFieldName(req.Name)
	req.Description = sanitizeFieldDescription(req.Description)
	req.Config = sanitizeFieldConfig(req.Config)

	if err := validateFieldName(req.Name); err != nil {
		return nil, fmt.Errorf("field name validation failed: %w", err)
	}

	if err := validateFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("field type validation failed: %w", err)
	}
	if err := validateMutableFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("field type validation failed: %w", err)
	}
	req.Type = normalizeFieldType(req.Type)

	if err := validateFieldDescription(req.Description); err != nil {
		return nil, fmt.Errorf("field description validation failed: %w", err)
	}

	if err := validateFieldConfig(req.Config); err != nil {
		return nil, fmt.Errorf("field config validation failed: %w", err)
	}

	// 5. Check for duplicate field name
	var existingField models.Field
	err = s.db.Where("table_id = ? AND name = ? AND deleted_at IS NULL", req.TableID, req.Name).First(&existingField).Error
	if err == nil {
		return nil, errors.New("a field with this name already exists in this table")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// 6. Serialize config
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("config serialization failed: %w", err)
	}

	// 7. Create field
	field := models.Field{
		TableID:     req.TableID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Required:    req.Required,
		Options:     string(configJSON),
	}

	if err := s.db.Create(&field).Error; err != nil {
		return nil, fmt.Errorf("failed to create field: %w", err)
	}

	// Reload to get database-generated timestamps
	if err := s.db.First(&field, "id = ?", field.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload field: %w", err)
	}

	InvalidateFieldCache(field.TableID)
	return &field, nil
}

// ListFields lists fields for a table
func (s *FieldService) ListFields(tableID, userID string) ([]dto.FieldObject, error) {
	// 1. Resolve table identifier (supports ID or name)
	table, err := s.resolveTable(tableID)
	if err != nil {
		return nil, err
	}

	// 2. Check table access
	if err := s.checkTableAccess(table.ID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	// 3. Query fields
	var fields []models.Field
	err = s.db.Where("table_id = ? AND deleted_at IS NULL", table.ID).Order("created_at ASC").Find(&fields).Error
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	if len(fields) == 0 {
		return []dto.FieldObject{}, nil
	}

	fieldIDs := make([]string, len(fields))
	for i, field := range fields {
		fieldIDs[i] = field.ID
	}
	readResults, err := s.CheckFieldPermissions(userID, fieldIDs, "read")
	if err != nil {
		return nil, err
	}

	// 3. Convert to response format
	result := make([]dto.FieldObject, len(fields))
	index := 0
	for _, f := range fields {
		if !readResults[f.ID] {
			continue
		}

		var config dto.FieldConfig
		if f.Options != "" {
			_ = json.Unmarshal([]byte(f.Options), &config)
			// config is safely stored in DB; parse failure does not affect core functionality
		}

		result[index] = dto.FieldObject{
			ID:          f.ID,
			TableID:     f.TableID,
			Name:        f.Name,
			Type:        normalizeFieldType(f.Type),
			Description: f.Description,
			Deprecated:  isDeprecatedFieldType(f.Type),
			Required:    f.Required,
			Options:     strings.Join(config.Options, ", "),
			Config:      config,
		}
		index++
	}

	return result[:index], nil
}

// GetField gets field details
func (s *FieldService) GetField(fieldID, userID string) (*dto.FieldObject, error) {
	// 1. Resolve field identifier (supports ID or name)
	// For name lookup, we need a table context; try ID first via getActiveField
	field, err := s.getActiveField(fieldID)
	if err != nil {
		return nil, fmt.Errorf("field not found: %w", err)
	}

	// 2. Check table access
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}
	if err := s.CheckFieldPermission(userID, field.ID, "read"); err != nil {
		return nil, err
	}

	// 3. Parse config
	var config dto.FieldConfig
	if field.Options != "" {
		_ = json.Unmarshal([]byte(field.Options), &config)
		// config is safely stored in DB; parse failure does not affect core functionality
	}

	return &dto.FieldObject{
		ID:          field.ID,
		TableID:     field.TableID,
		Name:        field.Name,
		Type:        normalizeFieldType(field.Type),
		Description: field.Description,
		Deprecated:  isDeprecatedFieldType(field.Type),
		Required:    field.Required,
		Options:     strings.Join(config.Options, ", "),
		Config:      config,
	}, nil
}

// UpdateField updates a field
func (s *FieldService) UpdateField(fieldID string, req dto.FieldUpdateRequest, userID string) (*models.Field, error) {
	// 1. Get field info
	field, err := s.getActiveField(fieldID)
	if err != nil {
		return nil, fmt.Errorf("field not found: %w", err)
	}

	// 2. Check table access (only owner, admin, editor can modify)
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 3. Input validation and sanitization
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
		return nil, fmt.Errorf("field name validation failed: %w", err)
	}

	if err := validateFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("field type validation failed: %w", err)
	}
	if err := validateMutableFieldType(req.Type); err != nil {
		return nil, fmt.Errorf("field type validation failed: %w", err)
	}
	req.Type = normalizeFieldType(req.Type)

	if err := validateFieldDescription(req.Description); err != nil {
		return nil, fmt.Errorf("field description validation failed: %w", err)
	}

	if err := validateFieldConfig(req.Config); err != nil {
		return nil, fmt.Errorf("field config validation failed: %w", err)
	}

	// 4. Check for duplicate field name (excluding current field)
	var existingField models.Field
	err = s.db.Where("table_id = ? AND name = ? AND id != ? AND deleted_at IS NULL", field.TableID, req.Name, fieldID).First(&existingField).Error
	if err == nil {
		return nil, errors.New("a field with this name already exists in this table")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// 5. Serialize config
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("config serialization failed: %w", err)
	}

	// 6. Update field info
	field.Name = req.Name
	field.Type = req.Type
	field.Description = req.Description
	field.Required = req.Required
	field.Options = string(configJSON)

	if err := s.db.Save(field).Error; err != nil {
		return nil, fmt.Errorf("failed to update field: %w", err)
	}

	InvalidateFieldCache(field.TableID)
	return field, nil
}

// DeleteField soft-deletes a field
func (s *FieldService) DeleteField(fieldID, userID string) error {
	// 1. Get field info
	field, err := s.getActiveField(fieldID)
	if err != nil {
		return fmt.Errorf("field not found: %w", err)
	}

	// 2. Check table access (only owner, admin can delete)
	if err := s.checkTableAccess(field.TableID, userID, []string{"owner", "admin"}); err != nil {
		return err
	}

	// 3. Soft-delete field
	now := time.Now()
	result := s.db.Model(&models.Field{}).
		Where("id = ? AND deleted_at IS NULL", fieldID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"name":       buildDeletedFieldName(field.Name, fieldID),
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to delete field: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("field not found: %w", gorm.ErrRecordNotFound)
	}

	InvalidateFieldCache(field.TableID)
	return nil
}

// CheckFieldPermission checks user permission for a specific field
func (s *FieldService) CheckFieldPermission(userID, fieldID, action string) error {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	if !authorizer.CanAccessField(fieldID, action) {
		return errors.New("permission denied: cannot access this field")
	}
	return nil
}

// CheckFieldPermissions batch-checks field permissions with a single DB query.
func (s *FieldService) CheckFieldPermissions(userID string, fieldIDs []string, action string) (map[string]bool, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	return authorizer.CanAccessFields(fieldIDs, action), nil
}
