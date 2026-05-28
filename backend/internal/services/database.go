package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/authz"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

type DatabaseService struct {
	db *gorm.DB
}

func NewDatabaseService(db *gorm.DB) *DatabaseService {
	return &DatabaseService{db: db}
}

type CreateDBRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
}

type UpdateDBRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description" binding:"max=500"`
}

type DBResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func validateDatabaseName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 255 {
		return errors.New("数据库名称长度必须在2-255个字符之间")
	}
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_\-\s]+$`, name)
	if !matched {
		return errors.New("数据库名称只能包含字母、数字、下划线、连字符和空格")
	}
	return nil
}

func validateDescription(desc string) error {
	if len(desc) > 500 {
		return errors.New("描述长度不能超过500个字符")
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

func (s *DatabaseService) CreateDatabase(req CreateDBRequest, ownerID string) (*models.Database, error) {
	authorizer, err := authz.NewAuthorizer(s.db, ownerID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanCreateDatabase() {
		return nil, errors.New("此操作需要 Master Token")
	}

	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("数据库名称验证失败: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("描述验证失败: %w", err)
	}

	var existingDB models.Database
	err = s.db.Where("name = ? AND deleted_at IS NULL", req.Name).First(&existingDB).Error
	if err == nil {
		return nil, errors.New("已存在同名数据库")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	database := models.Database{
		Name:        req.Name,
		Description: req.Description,
	}
	if err := s.db.Create(&database).Error; err != nil {
		return nil, fmt.Errorf("创建数据库失败: %w", err)
	}

	return &database, nil
}

func (s *DatabaseService) ListDatabases(userID string) ([]DBResponse, error) {
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
			return []DBResponse{}, nil
		}
		query = query.Where("id IN ?", ids)
	}
	err = query.Find(&databases).Error
	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	dbs := make([]DBResponse, len(databases))
	for i, d := range databases {
		dbs[i] = DBResponse{
			ID:          d.ID,
			Name:        d.Name,
			Description: d.Description,
			CreatedAt:   d.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   d.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return dbs, nil
}

func (s *DatabaseService) GetDatabase(dbID, userID string) (*DBResponse, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessDatabase(dbID, authz.ActionRead) {
		return nil, errors.New("无权访问该数据库")
	}

	var database models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", dbID).First(&database).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("数据库不存在")
		}
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	return &DBResponse{
		ID:          database.ID,
		Name:        database.Name,
		Description: database.Description,
		CreatedAt:   database.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   database.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *DatabaseService) UpdateDatabase(dbID string, req UpdateDBRequest, userID string) (*models.Database, error) {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanAccessDatabase(dbID, authz.ActionManage) {
		return nil, errors.New("无权修改该数据库")
	}

	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("数据库名称验证失败: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("描述验证失败: %w", err)
	}

	var database models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", dbID).First(&database).Error
	if err != nil {
		return nil, errors.New("数据库不存在")
	}

	var duplicate models.Database
	err = s.db.Where("name = ? AND id <> ? AND deleted_at IS NULL", req.Name, dbID).First(&duplicate).Error
	if err == nil {
		return nil, errors.New("已存在同名数据库")
	}

	database.Name = req.Name
	database.Description = req.Description

	if err := s.db.Save(&database).Error; err != nil {
		return nil, fmt.Errorf("更新数据库失败: %w", err)
	}

	return &database, nil
}

func (s *DatabaseService) DeleteDatabase(dbID, userID string) error {
	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	if !authorizer.CanAccessDatabase(dbID, authz.ActionManage) {
		return errors.New("无权删除该数据库")
	}

	var database models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", dbID).First(&database).Error
	if err != nil {
		return errors.New("数据库不存在")
	}

	now := time.Now()
	result := s.db.Model(&models.Database{}).
		Where("id = ? AND deleted_at IS NULL", dbID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("删除数据库失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("数据库不存在")
	}

	return nil
}

// CreateDatabaseWithTables 通过 JSON 定义创建数据库及嵌套的表和字段
type CreateTableWithFieldsRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Fields      []struct {
		Name        string `json:"name" binding:"required"`
		Type        string `json:"type" binding:"required"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
	} `json:"fields"`
}

type CreateDBWithTablesRequest struct {
	Name        string                         `json:"name" binding:"required,min=2,max=255"`
	Description string                         `json:"description"`
	Tables      []CreateTableWithFieldsRequest `json:"tables"`
}

type CreateDBWithTablesResult struct {
	Database *models.Database   `json:"database"`
	Tables   []*models.Table    `json:"tables"`
	Fields   []*models.Field    `json:"fields"`
}

func (s *DatabaseService) CreateDatabaseWithTables(req CreateDBWithTablesRequest, ownerID string) (*CreateDBWithTablesResult, error) {
	authorizer, err := authz.NewAuthorizer(s.db, ownerID)
	if err != nil {
		return nil, err
	}
	if !authorizer.CanCreateDatabase() {
		return nil, errors.New("此操作需要 Master Token")
	}

	req.Name, req.Description = sanitizeDatabaseInput(req.Name, req.Description)

	if err := validateDatabaseName(req.Name); err != nil {
		return nil, fmt.Errorf("数据库名称验证失败: %w", err)
	}

	if err := validateDescription(req.Description); err != nil {
		return nil, fmt.Errorf("描述验证失败: %w", err)
	}

	var existingDB models.Database
	err = s.db.Where("name = ? AND deleted_at IS NULL", req.Name).First(&existingDB).Error
	if err == nil {
		return nil, errors.New("已存在同名数据库")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
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
			return fmt.Errorf("创建数据库失败: %w", err)
		}
		result.Database = &database

		tableService := NewTableService(tx)
		fieldService := NewFieldService(tx)

		for _, tableReq := range req.Tables {
			tableReq.Name, tableReq.Description = sanitizeTableInput(tableReq.Name, tableReq.Description)

			if err := validateTableName(tableReq.Name); err != nil {
				return fmt.Errorf("表名称验证失败: %w", err)
			}

			table, err := tableService.CreateTable(CreateTableRequest{
				DatabaseID:  database.ID,
				Name:        tableReq.Name,
				Description: tableReq.Description,
			}, ownerID)
			if err != nil {
				return fmt.Errorf("创建表 %s 失败: %w", tableReq.Name, err)
			}
			result.Tables = append(result.Tables, table)

			for _, fieldReq := range tableReq.Fields {
				fieldReq.Name = sanitizeFieldName(fieldReq.Name)

				if err := validateFieldName(fieldReq.Name); err != nil {
					return fmt.Errorf("字段名称验证失败: %w", err)
				}

				if err := validateFieldType(fieldReq.Type); err != nil {
					return fmt.Errorf("字段类型验证失败: %w", err)
				}

				field, err := fieldService.CreateField(CreateFieldRequest{
					TableID:     table.ID,
					Name:        fieldReq.Name,
					Type:        fieldReq.Type,
					Description: fieldReq.Description,
					Required:    fieldReq.Required,
				}, ownerID)
				if err != nil {
					return fmt.Errorf("创建字段 %s 失败: %w", fieldReq.Name, err)
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
