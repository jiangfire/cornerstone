package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	"gorm.io/gorm"
)

type TableService struct {
	db *gorm.DB
}

func NewTableService(db *gorm.DB) *TableService {
	return &TableService{db: db}
}

// ResolveTable resolves a table identifier within a database to a table model.
// databaseIdentifier can be a database ID or name.
// tableIdentifier can be a table ID or name.
func (s *TableService) ResolveTable(databaseIdentifier, tableIdentifier string) (*models.Table, error) {
	// Resolve database first
	dbService := NewDatabaseService(s.db)
	database, err := dbService.ResolveDatabase(databaseIdentifier)
	if err != nil {
		return nil, err
	}

	var table models.Table
	// Try table ID first
	err = s.db.Where("id = ? AND deleted_at IS NULL", tableIdentifier).First(&table).Error
	if err == nil {
		if table.DatabaseID != database.ID {
			return nil, errors.New("table does not belong to the specified database")
		}
		return &table, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("table query failed: %w", err)
	}
	// Fallback to table name within the database
	err = s.db.Where("database_id = ? AND name = ? AND deleted_at IS NULL", database.ID, tableIdentifier).First(&table).Error
	if err == nil {
		return &table, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("table not found")
	}
	return nil, fmt.Errorf("table query failed: %w", err)
}

type CreateTableRequest struct {
	DatabaseID  string `json:"database_id" binding:"required"`
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
}

type UpdateTableRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
}

type TableResponse struct {
	ID          string `json:"id"`
	DatabaseID  string `json:"database_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func buildDeletedTableName(name, tableID string) string {
	suffix := "__deleted__" + tableID
	maxPrefixLen := 255 - len(suffix)
	if maxPrefixLen < 0 {
		maxPrefixLen = 0
	}
	if len(name) > maxPrefixLen {
		name = name[:maxPrefixLen]
	}
	return name + suffix
}

func (s *TableService) getActiveTable(tableID string) (*models.Table, error) {
	var table models.Table
	err := s.db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error
	if err != nil {
		return nil, err
	}
	return &table, nil
}

// resolveTable resolves a table identifier to a table model.
// It first tries to find by ID, then falls back to name lookup.
func (s *TableService) resolveTable(identifier string) (*models.Table, error) {
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

func validateTableName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 255 {
		return errors.New("table name must be between 2 and 255 characters")
	}
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_]+$`, name)
	if !matched {
		return errors.New("table name can only contain letters, numbers and underscores")
	}
	if matched, _ := regexp.MatchString(`^[0-9]`, name); matched {
		return errors.New("table name must not start with a digit")
	}
	return nil
}

func sanitizeTableInput(name, description string) (string, string) {
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

func (s *TableService) CreateTable(req CreateTableRequest, userID string) (*models.Table, error) {
	// Resolve database identifier (supports ID or name)
	dbService := NewDatabaseService(s.db)
	database, err := dbService.ResolveDatabase(req.DatabaseID)
	if err != nil {
		return nil, err
	}
	req.DatabaseID = database.ID

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessDatabase(req.DatabaseID, authz.ActionManage) {
		return nil, errors.New("permission denied: cannot create tables in this database")
	}

	req.Name, req.Description = sanitizeTableInput(req.Name, req.Description)

	if err := validateTableName(req.Name); err != nil {
		return nil, fmt.Errorf("table name validation failed: %w", err)
	}

	var existingDB models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", req.DatabaseID).First(&existingDB).Error
	if err != nil {
		return nil, errors.New("database not found")
	}

	var existingTable models.Table
	err = s.db.Where("database_id = ? AND name = ? AND deleted_at IS NULL", req.DatabaseID, req.Name).First(&existingTable).Error
	if err == nil {
		return nil, errors.New("a table with this name already exists in the database")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	table := models.Table{
		DatabaseID:  req.DatabaseID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.db.Create(&table).Error; err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	// Reload to get database-generated timestamps
	if err := s.db.First(&table, "id = ?", table.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload table: %w", err)
	}

	return &table, nil
}

func (s *TableService) ListTables(dbID, userID string) ([]TableResponse, error) {
	// Resolve database identifier (supports ID or name)
	dbService := NewDatabaseService(s.db)
	database, err := dbService.ResolveDatabase(dbID)
	if err != nil {
		return nil, err
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessDatabase(database.ID, authz.ActionRead) {
		return nil, errors.New("permission denied: cannot access tables in this database")
	}

	var tables []models.Table
	err = s.db.Where("database_id = ? AND deleted_at IS NULL", database.ID).Order("created_at ASC").Find(&tables).Error
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	result := make([]TableResponse, len(tables))
	for i, t := range tables {
		result[i] = TableResponse{
			ID:          t.ID,
			DatabaseID:  t.DatabaseID,
			Name:        t.Name,
			Description: t.Description,
		}
	}

	return result, nil
}

func (s *TableService) GetTable(tableID, userID string) (*TableResponse, error) {
	// Resolve table identifier (supports ID or name)
	table, err := s.resolveTable(tableID)
	if err != nil {
		return nil, err
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessTable(table.ID, authz.ActionRead) {
		return nil, errors.New("permission denied: cannot access this table")
	}

	return &TableResponse{
		ID:          table.ID,
		DatabaseID:  table.DatabaseID,
		Name:        table.Name,
		Description: table.Description,
	}, nil
}

func (s *TableService) UpdateTable(tableID string, req UpdateTableRequest, userID string) (*models.Table, error) {
	// Resolve table identifier (supports ID or name)
	table, err := s.resolveTable(tableID)
	if err != nil {
		return nil, err
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessTable(table.ID, authz.ActionManage) {
		return nil, errors.New("permission denied: cannot modify this table")
	}

	req.Name, req.Description = sanitizeTableInput(req.Name, req.Description)

	if err := validateTableName(req.Name); err != nil {
		return nil, fmt.Errorf("table name validation failed: %w", err)
	}

	var existingTable models.Table
	err = s.db.Where("database_id = ? AND name = ? AND id != ? AND deleted_at IS NULL", table.DatabaseID, req.Name, table.ID).First(&existingTable).Error
	if err == nil {
		return nil, errors.New("a table with this name already exists in the database")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	table.Name = req.Name
	table.Description = req.Description

	if err := s.db.Save(table).Error; err != nil {
		return nil, fmt.Errorf("failed to update table: %w", err)
	}

	return table, nil
}

func (s *TableService) DeleteTable(tableID, userID string) error {
	// Resolve table identifier (supports ID or name)
	table, err := s.resolveTable(tableID)
	if err != nil {
		return err
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	if !authorizer.CanAccessTable(table.ID, authz.ActionManage) {
		return errors.New("permission denied: cannot delete this table")
	}

	now := time.Now()
	result := s.db.Model(&models.Table{}).
		Where("id = ? AND deleted_at IS NULL", table.ID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"name":       buildDeletedTableName(table.Name, table.ID),
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to delete table: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("table not found: %w", gorm.ErrRecordNotFound)
	}

	return nil
}
