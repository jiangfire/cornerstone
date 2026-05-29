package authz

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

const (
	ActionRead   = "read"
	ActionWrite  = "write"
	ActionDelete = "delete"
	ActionManage = "manage"
)

type TableScope struct {
	Role   string              `json:"role"`
	Fields map[string][]string `json:"fields,omitempty"`
}

type ScopeConfig struct {
	Databases map[string]string     `json:"databases"`
	Tables    map[string]TableScope `json:"tables"`
}

type Authorizer struct {
	db     *gorm.DB
	token  models.Token
	scopes ScopeConfig
}

func NewAuthorizer(db *gorm.DB, tokenID string) (*Authorizer, error) {
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	var token models.Token
	if err := db.Where("id = ?", tokenID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Token 不存在")
		}
		return nil, fmt.Errorf("查询 Token 失败: %w", err)
	}

	scopes, err := parseScopes(token.Scopes)
	if err != nil {
		return nil, err
	}

	return &Authorizer{db: db, token: token, scopes: scopes}, nil
}

func parseScopes(raw string) (ScopeConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ScopeConfig{
			Databases: map[string]string{},
			Tables:    map[string]TableScope{},
		}, nil
	}

	var scopes ScopeConfig
	if err := json.Unmarshal([]byte(raw), &scopes); err != nil {
		return ScopeConfig{}, fmt.Errorf("解析 Token Scopes 失败: %w", err)
	}
	if scopes.Databases == nil {
		scopes.Databases = map[string]string{}
	}
	if scopes.Tables == nil {
		scopes.Tables = map[string]TableScope{}
	}
	return scopes, nil
}

func (a *Authorizer) IsMaster() bool {
	return a != nil && a.token.IsMaster
}

func (a *Authorizer) RequireMaster() error {
	if a.IsMaster() {
		return nil
	}
	return errors.New("此操作需要 Master Token")
}

func roleLevel(role string) int {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "viewer":
		return 1
	case "editor":
		return 2
	case "admin":
		return 3
	default:
		return 0
	}
}

func requiredRoleLevel(action string) int {
	switch action {
	case ActionRead:
		return roleLevel("viewer")
	case ActionWrite:
		return roleLevel("editor")
	case ActionDelete, ActionManage:
		return roleLevel("admin")
	default:
		return 0
	}
}

func (a *Authorizer) CanCreateDatabase() bool {
	return a.IsMaster()
}

func (a *Authorizer) CanAccessDatabase(dbID, action string) bool {
	if a.IsMaster() {
		return true
	}
	return roleLevel(a.scopes.Databases[dbID]) >= requiredRoleLevel(action)
}

func (a *Authorizer) CanAccessTable(tableID, action string) bool {
	if a.IsMaster() {
		return true
	}

	if scope, ok := a.scopes.Tables[tableID]; ok && roleLevel(scope.Role) >= requiredRoleLevel(action) {
		return true
	}

	dbID, err := a.lookupTableDatabaseID(tableID)
	if err == nil && a.CanAccessDatabase(dbID, action) {
		return true
	}

	return false
}

func (a *Authorizer) CanAccessField(fieldID, action string) bool {
	if a.IsMaster() {
		return true
	}

	field, err := a.lookupField(fieldID)
	if err != nil {
		return false
	}

	if scope, ok := a.scopes.Tables[field.TableID]; ok && len(scope.Fields) > 0 {
		if actions, ok := scope.Fields[field.ID]; ok && containsAction(actions, action) {
			return true
		}
		if actions, ok := scope.Fields[field.Name]; ok && containsAction(actions, action) {
			return true
		}
	}

	return a.CanAccessTable(field.TableID, action)
}

func containsAction(actions []string, action string) bool {
	for _, candidate := range actions {
		if strings.EqualFold(strings.TrimSpace(candidate), action) {
			return true
		}
	}
	return false
}

func (a *Authorizer) lookupTableDatabaseID(tableID string) (string, error) {
	var table models.Table
	if err := a.db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error; err != nil {
		return "", err
	}
	return table.DatabaseID, nil
}

func (a *Authorizer) lookupField(fieldID string) (*models.Field, error) {
	var field models.Field
	if err := a.db.Where("id = ? AND deleted_at IS NULL", fieldID).First(&field).Error; err != nil {
		return nil, err
	}
	return &field, nil
}

func (a *Authorizer) AccessibleDatabaseIDs() ([]string, error) {
	if a.IsMaster() {
		var ids []string
		if err := a.db.Model(&models.Database{}).Where("deleted_at IS NULL").Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	}

	ids := make([]string, 0, len(a.scopes.Databases))
	for dbID, role := range a.scopes.Databases {
		if roleLevel(role) >= requiredRoleLevel(ActionRead) {
			ids = append(ids, dbID)
		}
	}
	return ids, nil
}

func (a *Authorizer) AccessibleTableIDs() ([]string, error) {
	if a.IsMaster() {
		var ids []string
		if err := a.db.Model(&models.Table{}).Where("deleted_at IS NULL").Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	}

	seen := map[string]struct{}{}
	ids := make([]string, 0, len(a.scopes.Tables))

	dbIDs, err := a.AccessibleDatabaseIDs()
	if err != nil {
		return nil, err
	}
	if len(dbIDs) > 0 {
		var dbTableIDs []string
		if err := a.db.Model(&models.Table{}).
			Where("deleted_at IS NULL AND database_id IN ?", dbIDs).
			Pluck("id", &dbTableIDs).Error; err != nil {
			return nil, err
		}
		for _, id := range dbTableIDs {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	for tableID, scope := range a.scopes.Tables {
		if roleLevel(scope.Role) < requiredRoleLevel(ActionRead) {
			continue
		}
		if _, exists := seen[tableID]; exists {
			continue
		}
		seen[tableID] = struct{}{}
		ids = append(ids, tableID)
	}

	return ids, nil
}

func (a *Authorizer) AccessibleRecordIDs() ([]string, error) {
	tableIDs, err := a.AccessibleTableIDs()
	if err != nil {
		return nil, err
	}
	if len(tableIDs) == 0 {
		return []string{}, nil
	}

	var ids []string
	if err := a.db.Model(&models.Record{}).
		Where("deleted_at IS NULL AND table_id IN ?", tableIDs).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
