package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/dto"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type DatabaseService struct {
	db *gorm.DB
}

func NewDatabaseService(db *gorm.DB) *DatabaseService {
	return &DatabaseService{db: db}
}

// ResolveDatabase resolves a database identifier to a database model.
// It first tries to find by ID, then falls back to name lookup.
func (s *DatabaseService) ResolveDatabase(identifier string) (*models.Database, error) {
	var database models.Database
	// Try ID first
	err := s.db.Where("id = ? AND deleted_at IS NULL", identifier).First(&database).Error
	if err == nil {
		return &database, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	// Fallback to name
	err = s.db.Where("name = ? AND deleted_at IS NULL", identifier).First(&database).Error
	if err == nil {
		return &database, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("database not found")
	}
	return nil, fmt.Errorf("database query failed: %w", err)
}

func validateDatabaseName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 255 {
		return errors.New("database name must be between 2 and 255 characters")
	}
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_\-\s]+$`, name)
	if !matched {
		return errors.New("database name can only contain letters, numbers, underscores, hyphens and spaces")
	}
	return nil
}

func validateDescription(desc string) error {
	if len(desc) > 500 {
		return errors.New("description must not exceed 500 characters")
	}
	return nil
}

func sanitizeDatabaseInput(name, description string) (string, string) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "'", "")
	description = strings.ReplaceAll(description, "<", "")
	description = strings.ReplaceAll(description, ">", "")
	description = strings.ReplaceAll(description, "\"", "")
	description = strings.ReplaceAll(description, "'", "")
	return name, description
}

func (s *DatabaseService) CreateDatabase(req dto.DatabaseCreateRequest, ownerID string) (*models.Database, error) {
	authorizer, err := authz.NewAuthorizer(s.db, ownerID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanCreateDatabase() {
		return nil, errors.New("master token required for this operation")
	}

	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("database name validation failed: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("description validation failed: %w", err)
	}

	var existingDB models.Database
	err = s.db.Where("name = ? AND deleted_at IS NULL", req.Name).First(&existingDB).Error
	if err == nil {
		return nil, errors.New("database name already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	database := models.Database{
		Name:        req.Name,
		Description: req.Description,
	}
	if err := s.db.Create(&database).Error; err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Reload to get database-generated timestamps
	if err := s.db.First(&database, "id = ?", database.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload database: %w", err)
	}

	return &database, nil
}

func (s *DatabaseService) ListDatabases(userID string) ([]dto.DatabaseObject, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}

	var databases []models.Database
	query := s.db.Where("deleted_at IS NULL").Order("created_at DESC")
	if !authorizer.IsMaster() {
		ids, err := authorizer.AccessibleDatabaseIDs()
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return []dto.DatabaseObject{}, nil
		}
		query = query.Where("id IN ?", ids)
	}
	err = query.Find(&databases).Error
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	dbs := make([]dto.DatabaseObject, len(databases))
	for i, d := range databases {
		dbs[i] = dto.DatabaseObject{
			ID:          d.ID,
			Name:        d.Name,
			Description: d.Description,
		}
	}

	return dbs, nil
}

func (s *DatabaseService) GetDatabase(dbID, userID string) (*dto.DatabaseObject, error) {
	// Resolve identifier (supports ID or name)
	database, err := s.ResolveDatabase(dbID)
	if err != nil {
		return nil, err
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessDatabase(database.ID, authz.ActionRead) {
		return nil, errors.New("permission denied: cannot access this database")
	}

	return &dto.DatabaseObject{
		ID:          database.ID,
		Name:        database.Name,
		Description: database.Description,
	}, nil
}

func (s *DatabaseService) UpdateDatabase(dbID string, req dto.DatabaseUpdateRequest, userID string) (*models.Database, error) {
	// Resolve identifier (supports ID or name)
	database, err := s.ResolveDatabase(dbID)
	if err != nil {
		return nil, err
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessDatabase(database.ID, authz.ActionManage) {
		return nil, errors.New("permission denied: cannot modify this database")
	}

	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("database name validation failed: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("description validation failed: %w", err)
	}

	var duplicate models.Database
	err = s.db.Where("name = ? AND id <> ? AND deleted_at IS NULL", req.Name, database.ID).First(&duplicate).Error
	if err == nil {
		return nil, errors.New("database name already exists")
	}

	database.Name = req.Name
	database.Description = req.Description

	if err := s.db.Save(database).Error; err != nil {
		return nil, fmt.Errorf("failed to update database: %w", err)
	}

	return database, nil
}

func (s *DatabaseService) DeleteDatabase(dbID, userID string) error {
	// Resolve identifier (supports ID or name)
	database, err := s.ResolveDatabase(dbID)
	if err != nil {
		return err
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	if !authorizer.CanAccessDatabase(database.ID, authz.ActionManage) {
		return errors.New("permission denied: cannot delete this database")
	}

	now := time.Now()
	result := s.db.Model(&models.Database{}).
		Where("id = ? AND deleted_at IS NULL", database.ID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to delete database: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("database not found")
	}

	return nil
}

// CreateDatabaseWithTables creates a database with nested tables and fields from a JSON definition
type CreateDBWithTablesResult struct {
	Database *models.Database `json:"database"`
	Tables   []*models.Table  `json:"tables"`
	Fields   []*models.Field  `json:"fields"`
}

func (s *DatabaseService) CreateDatabaseWithTables(req dto.DatabaseBulkCreateRequest, ownerID string) (*CreateDBWithTablesResult, error) {
	authorizer, err := authz.NewAuthorizer(s.db, ownerID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanCreateDatabase() {
		return nil, errors.New("master token required for this operation")
	}

	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("database name validation failed: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("description validation failed: %w", err)
	}

	var existingDB models.Database
	err = s.db.Where("name = ? AND deleted_at IS NULL", req.Name).First(&existingDB).Error
	if err == nil {
		return nil, errors.New("database name already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	result := &CreateDBWithTablesResult{
		Tables: make([]*models.Table, 0),
		Fields: make([]*models.Field, 0),
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		database := models.Database{
			Name:        req.Name,
			Description: req.Description,
		}
		if err := tx.Create(&database).Error; err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		if err := tx.First(&database, "id = ?", database.ID).Error; err != nil {
			return fmt.Errorf("failed to reload database: %w", err)
		}
		result.Database = &database

		tableService := NewTableService(tx)
		fieldService := NewFieldService(tx)

		for _, tableReq := range req.Tables {
			tableReq.Name, tableReq.Description = sanitizeTableInput(tableReq.Name, tableReq.Description)

			if err := validateTableName(tableReq.Name); err != nil {
				return fmt.Errorf("table name validation failed: %w", err)
			}

			table, err := tableService.CreateTable(dto.TableCreateRequest{
				DatabaseID:  database.ID,
				Name:        tableReq.Name,
				Description: tableReq.Description,
			}, ownerID)
			if err != nil {
				return fmt.Errorf("failed to create table %s: %w", tableReq.Name, err)
			}
			result.Tables = append(result.Tables, table)

			for _, fieldReq := range tableReq.Fields {
				fieldReq.Name = sanitizeFieldName(fieldReq.Name)

				if err := validateFieldName(fieldReq.Name); err != nil {
					return fmt.Errorf("field name validation failed: %w", err)
				}

				if err := validateFieldType(fieldReq.Type); err != nil {
					return fmt.Errorf("field type validation failed: %w", err)
				}

				field, err := fieldService.CreateField(dto.FieldCreateRequest{
					TableID:     table.ID,
					Name:        fieldReq.Name,
					Type:        fieldReq.Type,
					Description: fieldReq.Description,
					Required:    fieldReq.Required,
				}, ownerID)
				if err != nil {
					return fmt.Errorf("failed to create field %s: %w", fieldReq.Name, err)
				}
				result.Fields = append(result.Fields, field)
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// YAMLTemplate returns a commented YAML template for database import.
func YAMLTemplate() []byte {
	template := `# Cornerstone Database Import Template
# Fill in the values below and import via POST /api/v1/databases/import/yaml
name: ""
description: ""
tables:
  - name: ""
    description: ""
    fields:
      - name: ""
        type: ""       # string, text, number, boolean, date, datetime, file, json, list
        description: ""
        required: false
`
	return []byte(template)
}

// ImportYAML parses a YAML document and creates a database with nested tables and fields.
func (s *DatabaseService) ImportYAML(data []byte, ownerID string) (*CreateDBWithTablesResult, error) {
	var req dto.DatabaseBulkCreateRequest
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid YAML format: %w", err)
	}

	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("database name is required")
	}

	return s.CreateDatabaseWithTables(req, ownerID)
}
