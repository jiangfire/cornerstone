package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type circuitBreaker struct {
	mu        sync.Mutex
	failures  int
	threshold int
	cooldown  time.Duration
	openUntil time.Time
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return time.Now().After(cb.openUntil)
}

func (cb *circuitBreaker) markSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.openUntil = time.Time{}
}

func (cb *circuitBreaker) markFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	if cb.failures >= cb.threshold {
		cb.openUntil = time.Now().Add(cb.cooldown)
	}
}

var (
	viewRefreshBreaker  = newCircuitBreaker(3, 2*time.Minute)
	tokenCleanupBreaker = newCircuitBreaker(3, 2*time.Minute)
)

// InitDB 初始化数据库连接（包装pkg/db中的函数）
func InitDB(cfg config.DatabaseConfig) error {
	return pkgdb.InitDB(cfg)
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	return pkgdb.CloseDB()
}

// Migrate 执行所有数据库迁移
func Migrate() error {
	database := pkgdb.DB()
	logger := zap.L()

	logger.Info("开始数据库迁移...")

	// 注册插件（如果需要）
	if err := database.AutoMigrate(
		// 核心用户和组织模型
		&models.User{},
		&models.Organization{},
		&models.OrganizationMember{},

		// 数据库和权限模型
		&models.Database{},
		&models.DatabaseAccess{},

		// 数据结构模型
		&models.Table{},
		&models.Field{},
		&models.Record{},

		// 字段级权限模型
		&models.FieldPermission{},

		// 文件和插件模型
		&models.File{},
		&models.Plugin{},
		&models.PluginBinding{},
		&models.PluginExecution{},

		// 活动日志
		&models.ActivityLog{},
		&models.AppSettings{},

		// 安全相关
		&models.TokenBlacklist{},
	); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	logger.Info("基础表结构迁移完成")

	// 创建复合索引
	if err := createIndexes(database); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	logger.Info("复合索引创建完成")

	// 创建物化视图（权限缓存）
	if err := createMaterializedViews(database); err != nil {
		return fmt.Errorf("创建物化视图失败: %w", err)
	}

	logger.Info("物化视图创建完成")

	// 创建token_blacklist表的特殊索引
	if err := createTokenBlacklistIndexes(database); err != nil {
		return fmt.Errorf("创建token blacklist索引失败: %w", err)
	}

	logger.Info("Token blacklist索引创建完成")

	// 初始化默认系统设置（单例）
	if err := database.FirstOrCreate(&models.AppSettings{ID: 1}, &models.AppSettings{
		ID:                1,
		SystemName:        "Cornerstone",
		SystemDescription: "数据管理平台",
		AllowRegistration: true,
		MaxFileSize:       50,
		DBType:            "postgresql",
		DBPoolSize:        10,
		DBTimeout:         30,
		PluginTimeout:     300,
		PluginWorkDir:     "./plugins",
		PluginAutoUpdate:  false,
	}).Error; err != nil {
		return fmt.Errorf("初始化系统设置失败: %w", err)
	}

	logger.Info("数据库迁移完成 ✅")
	return nil
}

// createIndexes 创建复合索引以提升查询性能
func createIndexes(db *gorm.DB) error {
	// records表的JSONB GIN索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_data ON records USING GIN(data)").Error; err != nil {
		return err
	}

	// records表的外键索引（数据查询优化）
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_table_id ON records(table_id)").Error; err != nil {
		return err
	}

	// tables表的外键索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tables_database_id ON tables(database_id)").Error; err != nil {
		return err
	}

	// fields表的外键索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_fields_table_id ON fields(table_id)").Error; err != nil {
		return err
	}

	// files表的外键索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_files_record_id ON files(record_id)").Error; err != nil {
		return err
	}

	// plugin_bindings复合索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_plugin_bindings_plugin_table ON plugin_bindings(plugin_id, table_id)").Error; err != nil {
		return err
	}

	// field_permissions按角色索引（权限查询优化）
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_field_permissions_role ON field_permissions(role)").Error; err != nil {
		return err
	}

	// plugin_executions 索引（执行记录查询优化）
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_plugin_executions_plugin_created ON plugin_executions(plugin_id, created_at DESC)").Error; err != nil {
		return err
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_plugin_executions_table_trigger ON plugin_executions(table_id, trigger)").Error; err != nil {
		return err
	}

	return nil
}

// createMaterializedViews 创建权限缓存物化视图
func createMaterializedViews(db *gorm.DB) error {
	// 删除旧视图（如果存在）
	db.Exec("DROP MATERIALIZED VIEW IF EXISTS user_database_permissions")

	// 创建物化视图
	viewSQL := `
CREATE MATERIALIZED VIEW user_database_permissions AS
SELECT
    da.id as access_id,
    da.user_id,
    da.database_id,
    da.role,
    d.name as db_name,
    d.owner_id,
    d.created_at,
    d.updated_at
FROM database_access da
JOIN databases d ON da.database_id = d.id
WHERE d.deleted_at IS NULL
`

	if err := db.Exec(viewSQL).Error; err != nil {
		return err
	}

	// 在物化视图上创建索引
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_db_permissions_unique ON user_database_permissions(user_id, database_id)").Error; err != nil {
		return err
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_db_permissions_user ON user_database_permissions(user_id)").Error; err != nil {
		return err
	}

	return nil
}

// createTokenBlacklistIndexes 创建token黑名单的特殊索引
func createTokenBlacklistIndexes(db *gorm.DB) error {
	// 创建普通索引用于查询性能
	// 条件索引在PostgreSQL中需要IMMUTABLE函数，这里我们使用应用层过滤
	indexSQL := `
CREATE INDEX IF NOT EXISTS idx_blacklist_expired
ON token_blacklist(expired_at)
`

	if err := db.Exec(indexSQL).Error; err != nil {
		return err
	}

	return nil
}

// RefreshMaterializedViews 刷新物化视图（用于定时任务）
func RefreshMaterializedViews() error {
	database := pkgdb.DB()
	logger := zap.L()

	logger.Info("开始刷新物化视图...")

	if err := database.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY user_database_permissions").Error; err != nil {
		return fmt.Errorf("刷新物化视图失败: %w", err)
	}

	logger.Info("物化视图刷新完成")
	return nil
}

// CleanupExpiredTokens 清理过期的token黑名单记录
func CleanupExpiredTokens() error {
	database := pkgdb.DB()
	logger := zap.L()

	result := database.Where("expired_at <= ?", time.Now()).Delete(&models.TokenBlacklist{})
	if result.Error != nil {
		return fmt.Errorf("清理过期token失败: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Info("清理过期token记录", zap.Int64("count", result.RowsAffected))
	}

	return nil
}

// SetupPeriodicTasks 设置定时任务并返回用于等待退出的 WaitGroup
func SetupPeriodicTasks(ctx context.Context) *sync.WaitGroup {
	wg := &sync.WaitGroup{}

	// 每5分钟刷新一次物化视图
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := runProtectedTask("刷新物化视图", viewRefreshBreaker, RefreshMaterializedViews); err != nil {
					zap.L().Error("定时刷新物化视图失败", zap.Error(err))
				}
			}
		}
	}()

	// 每小时清理一次过期token
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := runProtectedTask("清理过期token", tokenCleanupBreaker, CleanupExpiredTokens); err != nil {
					zap.L().Error("定时清理过期token失败", zap.Error(err))
				}
			}
		}
	}()

	return wg
}

func runProtectedTask(name string, breaker *circuitBreaker, task func() error) error {
	if !breaker.allow() {
		zap.L().Warn("任务熔断中，进入降级模式并跳过执行", zap.String("task", name))
		return nil
	}

	err := retry(task, 3, 500*time.Millisecond)
	if err != nil {
		breaker.markFailure()
		return err
	}

	breaker.markSuccess()
	return nil
}

func retry(task func() error, maxAttempts int, baseDelay time.Duration) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := task(); err != nil {
			lastErr = err
			if i < maxAttempts-1 {
				time.Sleep(baseDelay * time.Duration(i+1))
			}
			continue
		}
		return nil
	}
	return lastErr
}
