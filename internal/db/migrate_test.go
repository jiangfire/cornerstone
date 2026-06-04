package db

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbType := os.Getenv("DB_TYPE")
	databaseURL := os.Getenv("DATABASE_URL")
	if dbType == "" {
		dbType = "sqlite"
		databaseURL = ":memory:"
	}

	err := pkgdb.InitDB(config.DatabaseConfig{Type: dbType, URL: databaseURL})
	require.NoError(t, err)

	db := pkgdb.DB()
	require.NoError(t, db.AutoMigrate(&models.Token{}, &models.Database{}, &models.Table{}, &models.Field{}, &models.Record{}, &models.File{}))

	// 清理函数：硬删除所有测试数据
	t.Cleanup(func() {
		db.Unscoped().Where("1 = 1").Delete(&models.File{})
		db.Unscoped().Where("1 = 1").Delete(&models.Record{})
		db.Unscoped().Where("1 = 1").Delete(&models.Field{})
		db.Unscoped().Where("1 = 1").Delete(&models.Table{})
		db.Unscoped().Where("1 = 1").Delete(&models.Database{})
		db.Unscoped().Where("1 = 1").Delete(&models.Token{})
		_ = pkgdb.CloseDB()
	})

	return db
}

func TestCircuitBreaker_Allow(t *testing.T) {
	cb := newCircuitBreaker(2, 50*time.Millisecond)
	assert.True(t, cb.allow())
	cb.markFailure()
	assert.True(t, cb.allow())
	cb.markFailure()
	assert.False(t, cb.allow())
	time.Sleep(60 * time.Millisecond)
	assert.True(t, cb.allow())
}

func TestCircuitBreaker_MarkSuccess(t *testing.T) {
	cb := newCircuitBreaker(1, time.Hour)
	cb.markFailure()
	assert.False(t, cb.allow())
	cb.markSuccess()
	assert.True(t, cb.allow())
}

func TestRetry_SuccessOnFirst(t *testing.T) {
	err := retry(func() error { return nil }, 3, time.Millisecond)
	assert.NoError(t, err)
}

func TestRetry_SuccessOnRetry(t *testing.T) {
	var attempts int
	err := retry(func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("fail")
		}
		return nil
	}, 3, time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_AllFail(t *testing.T) {
	err := retry(func() error { return fmt.Errorf("always") }, 3, time.Millisecond)
	assert.Error(t, err)
}

func TestCleanupExpiredTokens_WithExpired(t *testing.T) {
	setupTestDB(t)
	past := time.Now().Add(-time.Hour)
	pkgdb.DB().Create(&models.Token{Name: "expired", IsMaster: false, Scopes: "{}", ExpiresAt: &past, CreatedAt: time.Now()})
	pkgdb.DB().Create(&models.Token{Name: "valid", IsMaster: false, Scopes: "{}", CreatedAt: time.Now()})

	err := CleanupExpiredTokens()
	require.NoError(t, err)

	var count int64
	pkgdb.DB().Model(&models.Token{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestCleanupExpiredTokens_NoExpired(t *testing.T) {
	setupTestDB(t)
	pkgdb.DB().Create(&models.Token{Name: "valid1", IsMaster: false, Scopes: "{}", CreatedAt: time.Now()})
	pkgdb.DB().Create(&models.Token{Name: "valid2", IsMaster: false, Scopes: "{}", CreatedAt: time.Now()})

	err := CleanupExpiredTokens()
	require.NoError(t, err)

	var count int64
	pkgdb.DB().Model(&models.Token{}).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestMigrate(t *testing.T) {
	dbType := os.Getenv("DB_TYPE")
	databaseURL := os.Getenv("DATABASE_URL")
	if dbType == "" {
		dbType = "sqlite"
		databaseURL = ":memory:"
	}

	err := pkgdb.InitDB(config.DatabaseConfig{Type: dbType, URL: databaseURL})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pkgdb.CloseDB() })

	err = Migrate()
	require.NoError(t, err)

	assert.True(t, pkgdb.DB().Migrator().HasTable("tokens"))
	assert.True(t, pkgdb.DB().Migrator().HasTable("databases"))
	assert.True(t, pkgdb.DB().Migrator().HasTable("tables"))
	assert.True(t, pkgdb.DB().Migrator().HasTable("fields"))
	assert.True(t, pkgdb.DB().Migrator().HasTable("records"))
	assert.True(t, pkgdb.DB().Migrator().HasTable("files"))
}

func TestIsSQLite(t *testing.T) {
	dbType := os.Getenv("DB_TYPE")
	if dbType != "" && dbType != "sqlite" {
		t.Skip("Skipping IsSQLite test on non-SQLite database")
	}

	err := pkgdb.InitDB(config.DatabaseConfig{Type: "sqlite", URL: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pkgdb.CloseDB() })

	assert.True(t, pkgdb.IsSQLite())
}

func TestCreateIndexes(t *testing.T) {
	db := setupTestDB(t)

	err := createIndexes(db)
	require.NoError(t, err)
}
