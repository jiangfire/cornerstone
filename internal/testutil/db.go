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
		// Default to SQLite in-memory for unit tests
		dbType = "sqlite"
		databaseURL = ":memory:"
	}

	isSQLiteMemory := dbType == "sqlite" && databaseURL == ":memory:"

	if isSQLiteMemory {
		// SQLite in-memory: each connection is an independent database, must reinitialize every time
		err := pkgdb.InitDB(config.DatabaseConfig{
			Type: dbType,
			URL:  databaseURL,
		})
		require.NoError(t, err)

		db := pkgdb.DB()
		require.NoError(t, internaldb.Migrate())

		// Cleanup function: hard-delete all test data and close connection
		t.Cleanup(func() {
			cleanupTables(db, t)
			_ = pkgdb.CloseDB()
		})

		return db
	}

	// PostgreSQL / MySQL: all tests share the same database instance
	// If the connection was closed by a previous test (e.g. simulating DB errors), reinitialize
	needsInit := true
	if pkgdb.IsInitialized() {
		db := pkgdb.DB()
		if sqlDB, err := db.DB(); err == nil && sqlDB.Ping() == nil {
			needsInit = false
		}
	}
	if needsInit {
		_ = pkgdb.CloseDB() // Clean up possibly corrupted old connection
		err := pkgdb.InitDB(config.DatabaseConfig{
			Type:        dbType,
			URL:         databaseURL,
			MaxOpen:     10,
			MaxIdle:     5,
			MaxLifetime: 3600,
		})
		require.NoError(t, err)
		require.NoError(t, internaldb.Migrate())
		// Previous test closed connection causing cleanupTables to be skipped, clean up residual data immediately after reinitialization
		cleanupTables(pkgdb.DB(), t)
	}

	db := pkgdb.DB()

	// Cleanup function: hard-delete all test data, but don't close the connection pool (reuse connections)
	t.Cleanup(func() {
		cleanupTables(db, t)
	})

	return db
}

// cleanupTables uses db.Exec raw SQL to clear all test tables, bypassing GORM callbacks/hooks
// to avoid interference from mock callbacks registered in tests. Also verifies tables are empty and clears global cache.
func cleanupTables(db *gorm.DB, tb testing.TB) {
	tb.Helper()

	// If the underlying connection was closed by a previous test (e.g. simulating DB errors), skip cleanup
	if sqlDB, err := db.DB(); err != nil || sqlDB.Ping() != nil {
		return
	}

	// MySQL uses SET FOREIGN_KEY_CHECKS=0 to disable foreign key checks, then truncate in any order
	// PostgreSQL/SQLite use TRUNCATE ... CASCADE or direct DELETE
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

	tables := []string{"files", "record_field_indexes", "records", "fields", "tables", "databases", "tokens"}
	for _, table := range tables {
		query := quoteIdentifier(db, table)
		if err := db.Exec("DELETE FROM " + query).Error; err != nil {
			tb.Logf("failed to delete from %s: %v", table, err)
		}
	}

	// Clear global cache to avoid cross-test pollution
	cache.ClearAll()

	// Force check: confirm all tables are empty
	var count int64
	for _, m := range []any{&models.File{}, &models.RecordFieldIndex{}, &models.Record{}, &models.Field{}, &models.Table{}, &models.Database{}, &models.Token{}} {
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

// quoteIdentifier returns the correctly quoted identifier for the database type.
// MySQL uses backticks, PostgreSQL/SQLite use double quotes.
func quoteIdentifier(db *gorm.DB, name string) string {
	if pkgdb.IsMySQL() {
		return fmt.Sprintf("`%s`", name)
	}
	return fmt.Sprintf("%q", name)
}
