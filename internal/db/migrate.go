package db

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

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
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_table_id ON records(table_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tables_database_id ON tables(database_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_fields_table_id ON fields(table_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_files_record_id ON files(record_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_files_field_id ON files(field_id)").Error; err != nil {
		return err
	}

	// PostgreSQL: GIN 索引加速 JSONB 查询（data @>、JSON 路径等）
	if !isSQLite(db) {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_data_gin ON records USING GIN (data)").Error; err != nil {
			return err
		}
	}

	return nil
}

// IsSQLite 检查当前数据库是否为 SQLite
func IsSQLite() bool {
	return isSQLite(pkgdb.DB())
}

func isSQLite(db *gorm.DB) bool {
	return db.Name() == "sqlite"
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
