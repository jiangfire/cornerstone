package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
)

func setupScopedExecutorTestDB(t *testing.T) *gorm.DB {
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
	err = db.AutoMigrate(
		&models.Token{},
		&models.Database{},
		&models.Table{},
		&models.Field{},
		&models.Record{},
		&models.File{},
	)
	require.NoError(t, err)

	return db
}

func TestExecutor_ExecuteHonorsTokenDatabaseScopes(t *testing.T) {
	db := setupScopedExecutorTestDB(t)

	allowedDB := &models.Database{Name: "allowed"}
	blockedDB := &models.Database{Name: "blocked"}
	require.NoError(t, db.Create(allowedDB).Error)
	require.NoError(t, db.Create(blockedDB).Error)

	allowedTable := &models.Table{DatabaseID: allowedDB.ID, Name: "allowed_records"}
	blockedTable := &models.Table{DatabaseID: blockedDB.ID, Name: "blocked_records"}
	require.NoError(t, db.Create(allowedTable).Error)
	require.NoError(t, db.Create(blockedTable).Error)

	require.NoError(t, db.Create(&models.Record{TableID: allowedTable.ID, Data: `{"name":"allowed"}`}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: blockedTable.ID, Data: `{"name":"blocked"}`}).Error)

	token := &models.Token{
		Name:   "viewer",
		Token:  "cs_viewer_scope",
		Scopes: `{"databases":{"` + allowedDB.ID + `":"viewer"}}`,
	}
	require.NoError(t, db.Create(token).Error)

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "records",
		Select: []string{"id", "table_id"},
		Page:   1,
		Size:   20,
	}, token.ID)
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, allowedTable.ID, result.Data[0]["table_id"])
}
