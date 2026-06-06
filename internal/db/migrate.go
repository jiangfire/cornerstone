package db

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
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
	tokenCleanupBreaker = newCircuitBreaker(3, 2*time.Minute)
)

// InitDB 初始化数据库连接
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

	if err := database.AutoMigrate(
		&models.Token{},
		&models.Database{},
		&models.Table{},
		&models.Field{},
		&models.Record{},
		&models.File{},
	); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	logger.Info("表结构迁移完成")

	if err := createIndexes(database); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	logger.Info("索引创建完成")

	masterToken := os.Getenv("MASTER_TOKEN")
	if masterToken == "" {
		logger.Warn("MASTER_TOKEN 环境变量未设置，Master Token 认证将不可用")
	} else {
		logger.Info("MASTER_TOKEN 已从环境变量加载")
	}

	logger.Info("数据库迁移完成 ✅")
	return nil
}

func createIndexes(db *gorm.DB) error {
	// records 列表主路径需要覆盖 table_id + deleted_at + created_at 排序
	if err := createIndexIfNotExists(db, "records", "idx_records_table_deleted_created", "table_id, deleted_at, created_at DESC"); err != nil {
		return err
	}

	// PostgreSQL: GIN 索引加速 JSONB 查询（data @>, JSON 路径等）
	if isPostgres(db) {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_data_gin ON records USING GIN (data)").Error; err != nil {
			return err
		}
	}

	return nil
}

// createIndexIfNotExists 跨数据库兼容的索引创建
func createIndexIfNotExists(db *gorm.DB, table, indexName, column string) error {
	exists, err := indexExists(db, table, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	sql := fmt.Sprintf("CREATE INDEX %s ON %s(%s)", indexName, table, column)
	return db.Exec(sql).Error
}

// indexExists 检查索引是否已存在
func indexExists(db *gorm.DB, table, indexName string) (bool, error) {
	var count int64
	switch db.Name() {
	case "sqlite":
		// SQLite: 查询 sqlite_master
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&count).Error; err != nil {
			return false, err
		}
	case "postgres":
		// PostgreSQL: 查询 pg_indexes
		if err := db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE indexname=?", indexName).Scan(&count).Error; err != nil {
			return false, err
		}
	case "mysql":
		// MySQL: 查询 information_schema.STATISTICS
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?", table, indexName).Scan(&count).Error; err != nil {
			return false, err
		}
	default:
		return false, fmt.Errorf("不支持的数据库类型: %s", db.Name())
	}
	return count > 0, nil
}

func isPostgres(db *gorm.DB) bool {
	return db.Name() == "postgres"
}

// CleanupExpiredTokens 清理过期的 Token
func CleanupExpiredTokens() error {
	database := pkgdb.DB()
	logger := zap.L()

	result := database.Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).Delete(&models.Token{})
	if result.Error != nil {
		return fmt.Errorf("清理过期 Token 失败: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Info("清理过期 Token", zap.Int64("count", result.RowsAffected))
		authz.ClearTokenCache()
	}
	return nil
}

// SetupPeriodicTasks 设置定时任务
func SetupPeriodicTasks(ctx context.Context) *sync.WaitGroup {
	wg := &sync.WaitGroup{}

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
				if err := runProtectedTask("清理过期 Token", tokenCleanupBreaker, CleanupExpiredTokens); err != nil {
					zap.L().Error("定时清理过期 Token 失败", zap.Error(err))
				}
			}
		}
	}()

	return wg
}

func runProtectedTask(name string, breaker *circuitBreaker, task func() error) error {
	if !breaker.allow() {
		zap.L().Warn("任务熔断中，跳过执行", zap.String("task", name))
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
