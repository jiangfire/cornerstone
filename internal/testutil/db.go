package testutil

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/config"
	internaldb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/cache"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbType := os.Getenv("DB_TYPE")
	databaseURL := os.Getenv("DATABASE_URL")

	if dbType == "" {
		// 默认使用 SQLite in-memory 进行单元测试
		dbType = "sqlite"
		databaseURL = ":memory:"
	}

	isSQLiteMemory := dbType == "sqlite" && databaseURL == ":memory:"

	if isSQLiteMemory {
		// SQLite in-memory 每个连接都是独立的数据库，必须每次重新初始化
		err := pkgdb.InitDB(config.DatabaseConfig{
			Type: dbType,
			URL:  databaseURL,
		})
		require.NoError(t, err)

		db := pkgdb.DB()
		require.NoError(t, internaldb.Migrate())

		// 清理函数：硬删除所有测试数据，并关闭连接
		t.Cleanup(func() {
			cleanupTables(db, t)
			_ = pkgdb.CloseDB()
		})

		return db
	}

	// PostgreSQL / MySQL：所有测试共享同一个数据库实例
	// 如果连接被之前的测试关闭（如模拟 DB 错误的测试），需要重新初始化
	needsInit := true
	if pkgdb.IsInitialized() {
		db := pkgdb.DB()
		if sqlDB, err := db.DB(); err == nil && sqlDB.Ping() == nil {
			needsInit = false
		}
	}
	if needsInit {
		_ = pkgdb.CloseDB() // 清理可能已损坏的旧连接
		err := pkgdb.InitDB(config.DatabaseConfig{
			Type:        dbType,
			URL:         databaseURL,
			MaxOpen:     10,
			MaxIdle:     5,
			MaxLifetime: 3600,
		})
		require.NoError(t, err)
		require.NoError(t, internaldb.Migrate())
		// 前一个测试关闭了连接导致 cleanupTables 被跳过，重新初始化后立即清理残余数据
		cleanupTables(pkgdb.DB(), t)
	}

	db := pkgdb.DB()

	// 清理函数：硬删除所有测试数据，但不关闭连接池（复用连接）
	t.Cleanup(func() {
		cleanupTables(db, t)
	})

	return db
}

// cleanupTables 使用 db.Exec raw SQL 清空所有测试表，绕过 GORM callback/hook，
// 避免被测试注册的 mock callback 干扰。同时验证表已清空并清理全局缓存。
func cleanupTables(db *gorm.DB, tb testing.TB) {
	tb.Helper()

	// 如果底层连接已被之前的测试关闭（如模拟 DB 错误的测试），跳过清理
	if sqlDB, err := db.DB(); err != nil || sqlDB.Ping() != nil {
		return
	}

	// MySQL 使用 SET FOREIGN_KEY_CHECKS=0 禁用外键检查，再按任意顺序截断
	// PostgreSQL/SQLite 使用 TRUNCATE ... CASCADE 或直接 DELETE
	if pkgdb.IsMySQL() {
		if err := db.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			tb.Logf("failed to disable FK checks: %v", err)
			return
		}
		defer func() {
			if err := db.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
				tb.Logf("failed to re-enable FK checks: %v", err)
			}
		}()
	}

	tables := []string{"files", "records", "fields", "tables", "databases", "tokens"}
	for _, table := range tables {
		query := quoteIdentifier(db, table)
		if err := db.Exec("DELETE FROM " + query).Error; err != nil {
			tb.Logf("failed to delete from %s: %v", table, err)
		}
	}

	// 清理全局缓存，避免测试间污染
	cache.ClearAll()

	// 强制检查：确认所有表已清空
	var count int64
	for _, m := range []any{&models.File{}, &models.Record{}, &models.Field{}, &models.Table{}, &models.Database{}, &models.Token{}} {
		if err := db.Model(m).Unscoped().Count(&count).Error; err != nil {
			tb.Logf("failed to count %T: %v", m, err)
		} else {
			assert.Zero(tb, count, "%T not empty after cleanup", m)
		}
	}
}

func SetupTestDBWithTokens(t *testing.T, tokenIDs ...string) *gorm.DB {
	db := SetupTestDB(t)

	for _, id := range tokenIDs {
		require.NoError(t, db.Create(&models.Token{
			ID:        id,
			Token:     "cs_" + id + "_master",
			Name:      id,
			IsMaster:  true,
			Scopes:    "{}",
			CreatedAt: time.Now(),
		}).Error)
	}

	return db
}

// quoteIdentifier 根据数据库类型返回正确引用的标识符。
// MySQL 使用反引号，PostgreSQL/SQLite 使用双引号。
func quoteIdentifier(db *gorm.DB, name string) string {
	if pkgdb.IsMySQL() {
		return fmt.Sprintf("`%s`", name)
	}
	return fmt.Sprintf("%q", name)
}
