package middleware

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jiangfire/cornerstone/internal/config"
	internaldb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/cache"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/require"
)

func BenchmarkValidateToken(b *testing.B) {
	token := setupAuthBenchmarkToken(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record, err := validateToken(token.Token)
		if err != nil {
			b.Fatal(err)
		}
		if record.ID == "" {
			b.Fatal("expected token id")
		}
	}
}

func setupAuthBenchmarkToken(tb testing.TB) *models.Token {
	tb.Helper()

	dbCfg, err := resolveAuthBenchmarkConfig(tb)
	require.NoError(tb, err)
	require.NoError(tb, pkgdb.CloseDB())
	require.NoError(tb, internaldb.InitDB(dbCfg))
	require.NoError(tb, internaldb.Migrate())

	database := pkgdb.DB()
	cache.ClearAll()
	token := &models.Token{
		Name:     "bench-token",
		Token:    "cs_auth_bench",
		IsMaster: false,
		Scopes:   "{}",
	}
	require.NoError(tb, database.Create(token).Error)

	tb.Cleanup(func() {
		cache.ClearAll()
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
		pkgdb.SetDB(nil)
	})

	return token
}

func resolveAuthBenchmarkConfig(tb testing.TB) (config.DatabaseConfig, error) {
	tb.Helper()

	dbType := strings.TrimSpace(os.Getenv("DB_TYPE"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbType == "" {
		dbType = "sqlite"
	}
	if dbType == "sqlite" && databaseURL == "" {
		databaseURL = filepath.Join(tb.TempDir(), "auth-bench.sqlite")
	}

	cfg := config.DatabaseConfig{
		Type:        dbType,
		URL:         databaseURL,
		MaxOpen:     4,
		MaxIdle:     4,
		MaxLifetime: 3600,
	}
	if err := (&config.Config{Database: cfg, Server: config.ServerConfig{Port: "8080"}}).Validate(); err != nil {
		return config.DatabaseConfig{}, err
	}
	return cfg, nil
}
