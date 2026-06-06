package authz

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/cache"
	"gorm.io/gorm"
)

// tokenCache 缓存 Token 及其解析后的权限配置，减少每次请求都查 DB 的开销。
// TTL 5 分钟，在大多数业务场景下 Token 和 Scopes 不会频繁变更。
var tokenCache = cache.NewString[*Authorizer]("token", 5*time.Minute)

func init() {
	cache.Register(tokenCache)
}

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

// jsonToken 用于 Authorizer 的 JSON 序列化/反序列化
type jsonToken struct {
	Token  models.Token `json:"token"`
	Scopes ScopeConfig  `json:"scopes"`
}

func (a Authorizer) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonToken{Token: a.token, Scopes: a.scopes})
}

func (a *Authorizer) UnmarshalJSON(data []byte) error {
	var aux jsonToken
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.token = aux.Token
	a.scopes = aux.Scopes
	return nil
}

func NewAuthorizer(db *gorm.DB, tokenID string) (*Authorizer, error) {
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	// 尝试从缓存读取
	if a, ok := tokenCache.Get(tokenID); ok {
		// 缓存中的 Authorizer 需要指向当前的 db（连接可能变化，如测试时）
		// 所以只缓存 token 和 scopes，db 仍然用传入的
		return &Authorizer{db: db, token: a.token, scopes: a.scopes}, nil
	}

	var token models.Token
	if err := db.Where("id = ?", tokenID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("token 不存在")
		}
		return nil, fmt.Errorf("查询 Token 失败: %w", err)
	}

	scopes, err := parseScopes(token.Scopes)
	if err != nil {
		return nil, err
	}

	authorizer := &Authorizer{db: db, token: token, scopes: scopes}
	tokenCache.Set(tokenID, authorizer)
	return authorizer, nil
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

// CanAccessFields 批量检查字段权限，只查一次数据库获取字段信息。
func (a *Authorizer) CanAccessFields(fieldIDs []string, action string) map[string]bool {
	results := make(map[string]bool, len(fieldIDs))
	if len(fieldIDs) == 0 {
		return results
	}
	if a.IsMaster() {
		for _, id := range fieldIDs {
			results[id] = true
		}
		return results
	}

	// 一次性查询所有字段定义
	var fields []models.Field
	if err := a.db.Where("id IN ? AND deleted_at IS NULL", fieldIDs).Find(&fields).Error; err != nil {
		for _, id := range fieldIDs {
			results[id] = false
		}
		return results
	}

	fieldMap := make(map[string]models.Field, len(fields))
	tableIDSet := make(map[string]struct{})
	for _, f := range fields {
		fieldMap[f.ID] = f
		tableIDSet[f.TableID] = struct{}{}
	}

	// 批量缓存表权限，避免对每个字段重复查询
	tablePerms := make(map[string]bool, len(tableIDSet))
	for tableID := range tableIDSet {
		tablePerms[tableID] = a.CanAccessTable(tableID, action)
	}

	for _, id := range fieldIDs {
		field, ok := fieldMap[id]
		if !ok {
			results[id] = false
			continue
		}

		if scope, ok := a.scopes.Tables[field.TableID]; ok && len(scope.Fields) > 0 {
			if actions, ok := scope.Fields[field.ID]; ok && containsAction(actions, action) {
				results[id] = true
				continue
			}
			if actions, ok := scope.Fields[field.Name]; ok && containsAction(actions, action) {
				results[id] = true
				continue
			}
		}
		results[id] = tablePerms[field.TableID]
	}
	return results
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

// InvalidateTokenCache 失效指定 Token 的缓存。
func InvalidateTokenCache(tokenID string) {
	tokenCache.Delete(tokenID)
}

// ClearTokenCache 清空所有 Token 缓存。
func ClearTokenCache() {
	tokenCache.Clear()
}
