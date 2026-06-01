package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/config"
	pkglog "github.com/jiangfire/cornerstone/pkg/log"
)

func initTestLogger(t *testing.T) {
	t.Helper()
	_ = pkglog.InitLogger(config.LoggerConfig{Level: "error"})
	t.Cleanup(func() { pkglog.Sync() })
}

func TestDB_PanicsWhenNotInitialized(t *testing.T) {
	initTestLogger(t)

	orig := db
	db = nil
	t.Cleanup(func() { db = orig })

	defer func() {
		r := recover()
		assert.NotNil(t, r)
		assert.Equal(t, "database not initialized", r)
	}()
	_ = DB()
}

func TestSetDB_And_DB(t *testing.T) {
	initTestLogger(t)

	orig := db
	t.Cleanup(func() { db = orig })

	var mockDB gorm.DB
	SetDB(&mockDB)
	result := DB()
	assert.Equal(t, &mockDB, result)
}

func TestIsSQLite_True(t *testing.T) {
	initTestLogger(t)

	err := InitDB(config.DatabaseConfig{
		Type: "sqlite",
		URL:  ":memory:",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = CloseDB() })

	assert.True(t, IsSQLite())
	assert.False(t, IsPostgres())
}

func TestIsSQLite_NilDB(t *testing.T) {
	orig := db
	db = nil
	t.Cleanup(func() { db = orig })

	assert.False(t, IsSQLite())
}

func TestIsPostgres_NilDB(t *testing.T) {
	orig := db
	db = nil
	t.Cleanup(func() { db = orig })

	assert.False(t, IsPostgres())
}

func TestInitDB_SqliteMemory(t *testing.T) {
	initTestLogger(t)

	err := InitDB(config.DatabaseConfig{
		Type: "sqlite",
		URL:  ":memory:",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = CloseDB() })

	assert.NotNil(t, DB())
}

func TestCloseDB_AfterInitDB(t *testing.T) {
	initTestLogger(t)

	err := InitDB(config.DatabaseConfig{
		Type: "sqlite",
		URL:  ":memory:",
	})
	require.NoError(t, err)

	err = CloseDB()
	assert.NoError(t, err)
}

func TestCloseDB_Nil(t *testing.T) {
	orig := db
	db = nil
	t.Cleanup(func() { db = orig })

	err := CloseDB()
	assert.NoError(t, err)
}

func TestInitDB_MemoryConnectionPool(t *testing.T) {
	initTestLogger(t)

	err := InitDB(config.DatabaseConfig{
		Type:    "sqlite",
		URL:     ":memory:",
		MaxOpen: 10,
		MaxIdle: 5,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = CloseDB() })

	sqlDB, err := DB().DB()
	require.NoError(t, err)

	stats := sqlDB.Stats()
	assert.Equal(t, 1, stats.MaxOpenConnections)
}
