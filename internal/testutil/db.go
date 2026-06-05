package testutil

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/cache"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
)

var autoMigrateModels = []any{
	&models.Token{},
	&models.Database{},
	&models.Table{},
	&models.Field{},
	&models.Record{},
	&models.File{},
}

var (
	testDBOnce    sync.Once
	testDBInitErr error
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
		err = db.AutoMigrate(autoMigrateModels...)
		require.NoError(t, err)

		// 清理函数：硬删除所有测试数据，并关闭连接
		t.Cleanup(func() {
			cleanupTables(db, t)
			_ = pkgdb.CloseDB()
		})

		return db
	}

	// PostgreSQL / MySQL：所有测试共享同一个数据库实例，只初始化一次
	testDBOnce.Do(func() {
		testDBInitErr = pkgdb.InitDB(config.DatabaseConfig{
			Type:        dbType,
			URL:         databaseURL,
			MaxOpen:     10,
			MaxIdle:     5,
			MaxLifetime: 3600,
		})
		if testDBInitErr != nil {
			return
		}
		testDBInitErr = pkgdb.DB().AutoMigrate(autoMigrateModels...)
	})
	require.NoError(t, testDBInitErr)

	db := pkgdb.DB()

	// 清理函数：硬删除所有测试数据，但不关闭连接池（复用连接）
	t.Cleanup(func() {
		cleanupTables(db, t)
	})

	return db
}

// cleanupTables 使用 db.Exec raw SQL 清空所有测试表，绕过 GORM callback/hook，
// 避免被测试注册的 mock callback 干扰。同时验证表已清空并清理全局缓存。
func cleanupTables(db *gorm.DB, t *testing.T) {
	// 按外键依赖顺序清理（先子表后父表）
	tables := []string{"files", "records", "fields", "tables", "databases", "tokens"}
	for _, table := range tables {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Logf("failed to delete from %s: %v", table, err)
		}
	}

	// 清理全局缓存，避免测试间污染
	cache.ClearAll()

	// 强制检查：确认所有表已清空
	var count int64
	for _, m := range []any{&models.File{}, &models.Record{}, &models.Field{}, &models.Table{}, &models.Database{}, &models.Token{}} {
		if err := db.Model(m).Unscoped().Count(&count).Error; err != nil {
			t.Logf("failed to count %T: %v", m, err)
		} else {
			assert.Zero(t, count, "%T not empty after cleanup", m)
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
