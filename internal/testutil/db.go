package testutil

import (
	"testing"

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

	err := pkgdb.InitDB(config.DatabaseConfig{
		Type: "sqlite",
		URL:  ":memory:",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
	})

	db := pkgdb.DB()
	err = db.AutoMigrate(autoMigrateModels...)
	require.NoError(t, err)

	return db
}

func SetupTestDBWithTokens(t *testing.T, tokenIDs ...string) *gorm.DB {
	db := SetupTestDB(t)

	for _, id := range tokenIDs {
		require.NoError(t, db.Create(&models.Token{
			ID:       id,
			Token:    "cs_" + id + "_master",
			Name:     id,
			IsMaster: true,
			Scopes:   "{}",
		}).Error)
	}

	return db
}
