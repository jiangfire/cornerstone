package testutil

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/models"
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

func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbType := os.Getenv("DB_TYPE")
	databaseURL := os.Getenv("DATABASE_URL")

	if dbType == "" {
		// 默认使用 SQLite in-memory 进行单元测试
		dbType = "sqlite"
		databaseURL = ":memory:"
	}

	err := pkgdb.InitDB(config.DatabaseConfig{
		Type: dbType,
		URL:  databaseURL,
	})
	require.NoError(t, err)

	db := pkgdb.DB()
	err = db.AutoMigrate(autoMigrateModels...)
	require.NoError(t, err)

	// 清理函数：硬删除所有测试数据（使用 Unscoped 绕过软删除）
	t.Cleanup(func() {
		// 按依赖顺序清理（先清理子表，再清理父表）
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
