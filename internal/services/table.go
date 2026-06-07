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
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
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

	return &table, nil
}

func (s *TableService) ListTables(dbID, userID string) ([]TableResponse, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessDatabase(dbID, authz.ActionRead) {
		return nil, errors.New("permission denied: cannot access tables in this database")
	}

	var tables []models.Table
	err = s.db.Where("database_id = ? AND deleted_at IS NULL", dbID).Order("created_at ASC").Find(&tables).Error
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
			CreatedAt:   t.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   t.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return result, nil
}

func (s *TableService) GetTable(tableID, userID string) (*TableResponse, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessTable(tableID, authz.ActionRead) {
		return nil, errors.New("permission denied: cannot access this table")
	}

	table, err := s.getActiveTable(tableID)
	if err != nil {
		return nil, fmt.Errorf("table not found: %w", err)
	}

	return &TableResponse{
		ID:          table.ID,
		DatabaseID:  table.DatabaseID,
		Name:        table.Name,
		Description: table.Description,
		CreatedAt:   table.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   table.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *TableService) UpdateTable(tableID string, req UpdateTableRequest, userID string) (*models.Table, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessTable(tableID, authz.ActionManage) {
		return nil, errors.New("permission denied: cannot modify this table")
	}

	table, err := s.getActiveTable(tableID)
	if err != nil {
		return nil, fmt.Errorf("table not found: %w", err)
	}

	req.Name, req.Description = sanitizeTableInput(req.Name, req.Description)

	if err := validateTableName(req.Name); err != nil {
		return nil, fmt.Errorf("table name validation failed: %w", err)
	}

	var existingTable models.Table
	err = s.db.Where("database_id = ? AND name = ? AND id != ? AND deleted_at IS NULL", table.DatabaseID, req.Name, tableID).First(&existingTable).Error
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
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	if !authorizer.CanAccessTable(tableID, authz.ActionManage) {
		return errors.New("permission denied: cannot delete this table")
	}

	table, err := s.getActiveTable(tableID)
	if err != nil {
		return fmt.Errorf("table not found: %w", err)
	}

	now := time.Now()
	result := s.db.Model(&models.Table{}).
		Where("id = ? AND deleted_at IS NULL", tableID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"name":       buildDeletedTableName(table.Name, tableID),
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
