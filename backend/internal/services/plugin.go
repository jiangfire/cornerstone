package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/jiangfire/cornerstone/backend/pkg/asyncworker"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PluginService 插件服务
type PluginService struct {
	db *gorm.DB
}

// NewPluginService 创建插件服务实例
func NewPluginService(db *gorm.DB) *PluginService {
	return &PluginService{db: db}
}

var (
	pluginPoolMu sync.RWMutex
	pluginPool   *asyncworker.Pool
)

// SetDefaultPluginPool 注入进程级异步任务池。
// 由 cmd/server/main.go 在启动时调用一次；测试可在 setup 中注入临时池。
// 传入 nil 表示清除（不再有池）。
func SetDefaultPluginPool(p *asyncworker.Pool) {
	pluginPoolMu.Lock()
	pluginPool = p
	pluginPoolMu.Unlock()
}

func currentPluginPool() *asyncworker.Pool {
	pluginPoolMu.RLock()
	defer pluginPoolMu.RUnlock()
	return pluginPool
}

// ensureSystemAdmin 验证调用方是否为系统管理员。
// 插件可以执行任意脚本进程，必须限制在系统管理员才能创建/更新；
// 否则任何注册用户都能上传脚本到服务端运行（RCE 风险）。
func (s *PluginService) ensureSystemAdmin(userID string) error {
	if userID == "" {
		return errors.New("仅系统管理员可管理插件")
	}
	authService := NewAuthService(s.db)
	isAdmin, err := authService.IsSystemAdmin(userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return errors.New("仅系统管理员可管理插件")
	}
	return nil
}

// CreatePluginRequest 创建插件请求
type CreatePluginRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=255"`
	Description  string `json:"description" binding:"max=500"`
	Language     string `json:"language" binding:"required,oneof=go python bash"`
	EntryFile    string `json:"entry_file" binding:"required"`
	Timeout      int    `json:"timeout" binding:"min=1,max=300"`
	Config       string `json:"config" binding:"omitempty"`        // JSON config schema
	ConfigValues string `json:"config_values" binding:"omitempty"` // JSON config values
}

// UpdatePluginRequest 更新插件请求
type UpdatePluginRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=255"`
	Description  string `json:"description" binding:"max=500"`
	Timeout      int    `json:"timeout" binding:"min=1,max=300"`
	Config       string `json:"config" binding:"omitempty"`
	ConfigValues string `json:"config_values" binding:"omitempty"`
}

// ExecutePluginRequest 手动执行插件请求
type ExecutePluginRequest struct {
	TableID  string                 `json:"table_id" binding:"required"`
	RecordID string                 `json:"record_id"`
	Trigger  string                 `json:"trigger" binding:"required,oneof=create update delete manual"`
	Payload  map[string]interface{} `json:"payload"`
}

// CreatePlugin 创建插件
func (s *PluginService) CreatePlugin(req CreatePluginRequest, userID string) (*models.Plugin, error) {
	if err := s.ensureSystemAdmin(userID); err != nil {
		return nil, err
	}

	// 检查插件名称是否已存在
	var existing models.Plugin
	if err := s.db.Where("name = ? AND created_by = ?", req.Name, userID).First(&existing).Error; err == nil {
		return nil, errors.New("插件名称已存在")
	}

	plugin := models.Plugin{
		Name:         req.Name,
		Description:  req.Description,
		Language:     req.Language,
		EntryFile:    req.EntryFile,
		Timeout:      req.Timeout,
		Config:       req.Config,
		ConfigValues: req.ConfigValues,
		CreatedBy:    userID,
	}

	if err := s.db.Create(&plugin).Error; err != nil {
		return nil, fmt.Errorf("创建插件失败: %w", err)
	}

	return &plugin, nil
}

// ListPlugins 列出插件
func (s *PluginService) ListPlugins(userID string) ([]models.Plugin, error) {
	var plugins []models.Plugin
	if err := s.db.Where("created_by = ?", userID).Find(&plugins).Error; err != nil {
		return nil, fmt.Errorf("查询插件列表失败: %w", err)
	}
	return plugins, nil
}

func (s *PluginService) getOwnedPlugin(pluginID, userID string) (*models.Plugin, error) {
	var plugin models.Plugin
	if err := s.db.Where("id = ? AND created_by = ?", pluginID, userID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件不存在或无权限")
		}
		return nil, fmt.Errorf("查询插件失败: %w", err)
	}
	return &plugin, nil
}

// GetPlugin 获取插件详情
func (s *PluginService) GetPlugin(pluginID, userID string) (*models.Plugin, error) {
	return s.getOwnedPlugin(pluginID, userID)
}

// UpdatePlugin 更新插件
func (s *PluginService) UpdatePlugin(pluginID string, req UpdatePluginRequest, userID string) error {
	if err := s.ensureSystemAdmin(userID); err != nil {
		return err
	}

	plugin, err := s.getOwnedPlugin(pluginID, userID)
	if err != nil {
		return err
	}

	plugin.Name = req.Name
	plugin.Description = req.Description
	plugin.Timeout = req.Timeout
	plugin.Config = req.Config
	plugin.ConfigValues = req.ConfigValues

	if err := s.db.Save(&plugin).Error; err != nil {
		return fmt.Errorf("更新插件失败: %w", err)
	}

	return nil
}

// DeletePlugin 删除插件
func (s *PluginService) DeletePlugin(pluginID, userID string) error {
	if _, err := s.getOwnedPlugin(pluginID, userID); err != nil {
		return err
	}

	// 硬删：Plugin 有 uk_plugin_creator_name 唯一约束，软删会让用户无法用同名重建。
	// 插件本体是用户资源（同 created_by），删除后用户应能用相同 name 重新注册。
	result := s.db.Unscoped().Where("id = ? AND created_by = ?", pluginID, userID).Delete(&models.Plugin{})
	if result.Error != nil {
		return fmt.Errorf("删除插件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("插件不存在")
	}
	return nil
}

// BindPlugin 绑定插件到表
func (s *PluginService) BindPlugin(pluginID, tableID, trigger, userID string) error {
	if _, err := s.getOwnedPlugin(pluginID, userID); err != nil {
		return err
	}

	if err := NewRecordService(s.db).checkTableAccess(tableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return err
	}

	// 创建绑定（幂等并发安全）
	binding := models.PluginBinding{
		PluginID: pluginID,
		TableID:  tableID,
		Trigger:  trigger,
	}
	result := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&binding)
	if result.Error != nil {
		return fmt.Errorf("绑定插件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("绑定关系已存在")
	}

	return nil
}

// UnbindPlugin 解绑插件
func (s *PluginService) UnbindPlugin(pluginID, tableID, userID string) error {
	if _, err := s.getOwnedPlugin(pluginID, userID); err != nil {
		return err
	}
	if err := NewRecordService(s.db).checkTableAccess(tableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return err
	}

	// 关系表硬删：PluginBinding 有 uk_plugin_table_trigger 唯一约束，软删会让"重新绑定"撞上残留约束
	result := s.db.Unscoped().Where("plugin_id = ? AND table_id = ?", pluginID, tableID).Delete(&models.PluginBinding{})
	if result.Error != nil {
		return fmt.Errorf("解绑插件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("绑定关系不存在")
	}
	return nil
}

// BindingDetail 绑定详情
type BindingDetail struct {
	ID           string    `json:"id"`
	TableID      string    `json:"table_id"`
	TableName    string    `json:"table_name"`
	DatabaseID   string    `json:"database_id"`
	DatabaseName string    `json:"database_name"`
	Trigger      string    `json:"trigger"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListBindings 列出插件的所有绑定
func (s *PluginService) ListBindings(pluginID, userID string) ([]BindingDetail, error) {
	if _, err := s.getOwnedPlugin(pluginID, userID); err != nil {
		return nil, err
	}

	var details []BindingDetail
	err := s.db.Table("plugin_bindings pb").
		Select(`
			pb.id,
			pb.table_id,
			t.name AS table_name,
			t.database_id,
			d.name AS database_name,
			pb.trigger,
			pb.created_at
		`).
		Joins("JOIN tables t ON t.id = pb.table_id").
		Joins("JOIN databases d ON d.id = t.database_id").
		Where("pb.plugin_id = ?", pluginID).
		Order("pb.created_at DESC").
		Scan(&details).Error
	if err != nil {
		return nil, fmt.Errorf("查询绑定失败: %w", err)
	}

	return details, nil
}

func truncateText(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "...(truncated)"
}

func clonePayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return map[string]interface{}{}
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return map[string]interface{}{}
	}

	var copied map[string]interface{}
	if err := json.Unmarshal(b, &copied); err != nil {
		return map[string]interface{}{}
	}
	return copied
}

func buildPluginCommand(language, scriptPath string) (string, []string, error) {
	switch language {
	case "go":
		return "go", []string{"run", scriptPath}, nil
	case "python":
		return "python", []string{scriptPath}, nil
	case "bash":
		return "bash", []string{scriptPath}, nil
	default:
		return "", nil, fmt.Errorf("不支持的插件语言: %s", language)
	}
}

var windowsDrivePathPattern = regexp.MustCompile(`^[A-Za-z]:`)

func isUnsafeAbsolutePluginPath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	if windowsDrivePathPattern.MatchString(path) {
		return true
	}
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//")
}

// sensitivePluginBasenames 入口文件名禁用列表（大小写不敏感）。
// 这些名字常用于凭据/密钥/版本控制元数据，绝对不应该作为脚本入口暴露在 ./plugins 下。
var sensitivePluginBasenames = map[string]struct{}{
	".env":             {},
	".env.local":       {},
	".env.production":  {},
	".env.development": {},
	".git":             {},
	".gitignore":       {},
	".gitconfig":       {},
	".ssh":             {},
	"id_rsa":           {},
	"id_dsa":           {},
	"id_ecdsa":         {},
	"id_ed25519":       {},
	"authorized_keys":  {},
	"known_hosts":      {},
	"passwd":           {},
	"shadow":           {},
}

func resolveScriptPath(workDir, entryFile string) (string, error) {
	cleanEntry := filepath.Clean(strings.TrimSpace(entryFile))
	if cleanEntry == "" || cleanEntry == "." {
		return "", errors.New("插件入口文件不能为空")
	}
	if strings.Contains(cleanEntry, "..") {
		return "", errors.New("插件入口文件路径非法")
	}
	if isUnsafeAbsolutePluginPath(cleanEntry) {
		return "", errors.New("插件入口文件不能是绝对路径")
	}

	base := strings.ToLower(filepath.Base(cleanEntry))
	if _, blocked := sensitivePluginBasenames[base]; blocked {
		return "", errors.New("插件入口文件名包含敏感名称，禁止使用")
	}

	scriptPath := filepath.Join(workDir, cleanEntry)

	// 二次校验：确保 join 之后还在 workDir 之内（filepath.Clean 已经吃掉了 ..,
	// 但混合分隔符 / 大小写边界等仍可能让 join 结果越界，这里用 Rel 兜底）。
	absWork, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("解析插件工作目录失败: %w", err)
	}
	absScript, err := filepath.Abs(scriptPath)
	if err != nil {
		return "", fmt.Errorf("解析插件脚本路径失败: %w", err)
	}
	rel, err := filepath.Rel(absWork, absScript)
	if err != nil {
		return "", errors.New("插件入口文件路径非法")
	}
	if rel == "" || rel == "." {
		return "", errors.New("插件入口文件不能为空")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("插件入口文件路径非法")
	}

	return scriptPath, nil
}

// assertScriptResolvesSafely 在执行前对解析出的脚本做文件系统层面的安全检查：
//   - 必须存在且可访问；
//   - 不能是符号链接（防止用符号链接逃逸 workDir 边界）；
//   - 不能是目录。
func assertScriptResolvesSafely(scriptPath string) error {
	info, err := os.Lstat(scriptPath)
	if err != nil {
		return fmt.Errorf("插件入口文件不可访问: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("插件入口文件不能是符号链接")
	}
	if info.IsDir() {
		return errors.New("插件入口文件不能是目录")
	}
	return nil
}

func (s *PluginService) executePlugin(plugin models.Plugin, tableID, recordID, trigger string, payload map[string]interface{}, actorID string) (*models.PluginExecution, error) {
	timeoutSec := plugin.Timeout
	workDir := "./plugins"
	settingsService := NewSettingsService(s.db)
	defaultTimeout, defaultWorkDir, settingsErr := settingsService.GetPluginRuntimeConfig()
	if settingsErr == nil {
		workDir = defaultWorkDir
		if timeoutSec <= 0 {
			timeoutSec = defaultTimeout
		}
	} else if timeoutSec <= 0 {
		timeoutSec = 300
	}

	scriptPath, err := resolveScriptPath(workDir, plugin.EntryFile)
	if err != nil {
		return nil, err
	}
	if err := assertScriptResolvesSafely(scriptPath); err != nil {
		return nil, err
	}

	command, args, err := buildPluginCommand(plugin.Language, scriptPath)
	if err != nil {
		return nil, err
	}

	execution := &models.PluginExecution{
		PluginID:  plugin.ID,
		TableID:   tableID,
		RecordID:  recordID,
		Trigger:   trigger,
		Status:    "running",
		StartedAt: time.Now(),
		CreatedBy: actorID,
	}
	if execution.CreatedBy == "" {
		execution.CreatedBy = plugin.CreatedBy
	}

	if err := s.db.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("创建插件执行记录失败: %w", err)
	}

	inputPayload := map[string]interface{}{
		"plugin_id": plugin.ID,
		"trigger":   trigger,
		"table_id":  tableID,
		"record_id": recordID,
		"payload":   payload,
	}
	inputBytes, _ := json.Marshal(inputPayload)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(inputBytes)
	cmd.Env = append(cmd.Environ(),
		fmt.Sprintf("PLUGIN_ID=%s", plugin.ID),
		fmt.Sprintf("PLUGIN_TRIGGER=%s", trigger),
		fmt.Sprintf("PLUGIN_CONFIG=%s", plugin.ConfigValues),
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()
	finished := time.Now()
	durationMS := finished.Sub(started).Milliseconds()

	status := "success"
	errMsg := ""
	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = "timeout"
			errMsg = "插件执行超时"
		} else {
			status = "failed"
			errMsg = runErr.Error()
		}
	}

	execution.Status = status
	execution.Output = truncateText(stdout.String(), 8192)
	execution.Error = truncateText(strings.TrimSpace(fmt.Sprintf("%s\n%s", errMsg, stderr.String())), 8192)
	execution.DurationMS = durationMS
	execution.FinishedAt = &finished

	if err := s.db.Save(execution).Error; err != nil {
		return nil, fmt.Errorf("更新插件执行记录失败: %w", err)
	}

	if runErr != nil {
		return execution, fmt.Errorf("插件执行失败: %w", runErr)
	}

	return execution, nil
}

// TriggerByTable 根据表和触发器执行绑定插件（异步最佳努力）。
//
// 提交到进程级 asyncworker.Pool；如果没有注入池（典型见单元测试），
// 退化为直接 `go func()`，行为与旧版保持兼容但失去 panic 兜底。
func (s *PluginService) TriggerByTable(tableID, trigger, recordID, actorID string, payload map[string]interface{}) {
	payloadCopy := clonePayload(payload)
	work := func(ctx context.Context) {
		var bindings []models.PluginBinding
		if err := s.db.Where("table_id = ? AND trigger = ?", tableID, trigger).Find(&bindings).Error; err != nil {
			zap.L().Error("查询插件绑定失败", zap.String("table_id", tableID), zap.String("trigger", trigger), zap.Error(err))
			return
		}

		for _, binding := range bindings {
			if ctx != nil {
				if err := ctx.Err(); err != nil {
					zap.L().Info("插件触发被取消", zap.String("plugin_id", binding.PluginID), zap.Error(err))
					return
				}
			}

			var plugin models.Plugin
			if err := s.db.Where("id = ?", binding.PluginID).First(&plugin).Error; err != nil {
				zap.L().Warn("读取插件信息失败", zap.String("plugin_id", binding.PluginID), zap.Error(err))
				continue
			}

			if _, err := s.executePlugin(plugin, tableID, recordID, trigger, payloadCopy, actorID); err != nil {
				zap.L().Warn("插件触发执行失败",
					zap.String("plugin_id", plugin.ID),
					zap.String("table_id", tableID),
					zap.String("trigger", trigger),
					zap.Error(err),
				)
			}
		}
	}

	if pool := currentPluginPool(); pool != nil {
		taskName := fmt.Sprintf("plugin_trigger:%s:%s", tableID, trigger)
		if err := pool.Submit(taskName, work); err != nil {
			zap.L().Warn("提交插件触发任务失败，退化为内联 goroutine",
				zap.String("table_id", tableID),
				zap.String("trigger", trigger),
				zap.Error(err),
			)
			go work(context.Background())
		}
		return
	}
	go work(context.Background())
}

// ExecutePlugin 手动执行插件
func (s *PluginService) ExecutePlugin(pluginID, userID string, req ExecutePluginRequest) (*models.PluginExecution, error) {
	plugin, err := s.getOwnedPlugin(pluginID, userID)
	if err != nil {
		return nil, err
	}
	if err := NewRecordService(s.db).checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	var binding models.PluginBinding
	if err := s.db.Where("plugin_id = ? AND table_id = ? AND trigger = ?", pluginID, req.TableID, req.Trigger).
		First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件未绑定到该表和触发器")
		}
		return nil, fmt.Errorf("查询插件绑定失败: %w", err)
	}

	return s.executePlugin(*plugin, req.TableID, req.RecordID, req.Trigger, req.Payload, userID)
}

// ListExecutions 查询插件执行记录
func (s *PluginService) ListExecutions(pluginID, userID string, limit int) ([]models.PluginExecution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	if _, err := s.getOwnedPlugin(pluginID, userID); err != nil {
		return nil, err
	}

	var executions []models.PluginExecution
	if err := s.db.Where("plugin_id = ?", pluginID).
		Order("created_at DESC").
		Limit(limit).
		Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("查询插件执行记录失败: %w", err)
	}

	return executions, nil
}
