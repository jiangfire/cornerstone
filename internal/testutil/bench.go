package testutil

import (
	"encoding/json"
	"fmt"
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
	"gorm.io/gorm"
)

type BenchmarkSeedConfig struct {
	RecordCount     int
	ExtraFieldCount int
}

type BenchmarkFixture struct {
	DB          *gorm.DB
	DBType      string
	DBPath      string
	Database    *models.Database
	Table       *models.Table
	Fields      []models.Field
	MasterToken *models.Token
	ScopedToken *models.Token
}

func resolveBenchmarkDatabaseConfig(tb testing.TB) (config.DatabaseConfig, error) {
	tb.Helper()

	dbType := strings.TrimSpace(os.Getenv("DB_TYPE"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbType == "" {
		dbType = "sqlite"
	}
	if dbType == "sqlite" && databaseURL == "" {
		databaseURL = filepath.Join(tb.TempDir(), "cornerstone-bench.sqlite")
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

func SetupBenchmarkFixture(tb testing.TB, cfg BenchmarkSeedConfig) *BenchmarkFixture {
	tb.Helper()

	if cfg.RecordCount <= 0 {
		cfg.RecordCount = 2000
	}

	dbCfg, err := resolveBenchmarkDatabaseConfig(tb)
	require.NoError(tb, err)
	require.NoError(tb, pkgdb.CloseDB())
	require.NoError(tb, internaldb.InitDB(dbCfg))

	database := pkgdb.DB()
	require.NoError(tb, internaldb.Migrate())
	cleanupTables(database, tb)
	cache.ClearAll()

	tb.Cleanup(func() {
		cleanupTables(database, tb)
		cache.ClearAll()
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
		pkgdb.SetDB(nil)
	})

	fixture := &BenchmarkFixture{
		DB:     database,
		DBType: database.Name(),
		DBPath: dbCfg.URL,
	}

	fixture.seed(tb, cfg)
	return fixture
}

func SetupSQLiteBenchmarkFixture(tb testing.TB, cfg BenchmarkSeedConfig) *BenchmarkFixture {
	tb.Helper()
	tb.Setenv("DB_TYPE", "sqlite")
	tb.Setenv("DATABASE_URL", filepath.Join(tb.TempDir(), "cornerstone-bench.sqlite"))
	return SetupBenchmarkFixture(tb, cfg)
}

func (f *BenchmarkFixture) seed(tb testing.TB, cfg BenchmarkSeedConfig) {
	tb.Helper()

	master := &models.Token{
		Name:     "bench-master",
		Token:    "cs_bench_master",
		IsMaster: true,
		Scopes:   "{}",
	}
	require.NoError(tb, f.DB.Create(master).Error)

	database := &models.Database{
		Name:        "bench_db",
		Description: "benchmark dataset",
	}
	require.NoError(tb, f.DB.Create(database).Error)

	table := &models.Table{
		DatabaseID:  database.ID,
		Name:        "bench_records",
		Description: "benchmark records",
	}
	require.NoError(tb, f.DB.Create(table).Error)

	fields := []models.Field{
		{TableID: table.ID, Name: "name", Type: "string"},
		{TableID: table.ID, Name: "status", Type: "string"},
		{TableID: table.ID, Name: "category", Type: "string"},
		{TableID: table.ID, Name: "score", Type: "number"},
		{TableID: table.ID, Name: "payload", Type: "json"},
	}

	for i := 0; i < cfg.ExtraFieldCount; i++ {
		fields = append(fields, models.Field{
			TableID: table.ID,
			Name:    fmt.Sprintf("extra_%02d", i),
			Type:    "string",
		})
	}

	require.NoError(tb, f.DB.Create(&fields).Error)

	scopeJSON := fmt.Sprintf(
		`{"databases":{"%s":"viewer"},"tables":{"%s":{"role":"viewer"}}}`,
		database.ID,
		table.ID,
	)
	scoped := &models.Token{
		Name:     "bench-scoped",
		Token:    "cs_bench_scoped",
		IsMaster: false,
		Scopes:   scopeJSON,
	}
	require.NoError(tb, f.DB.Create(scoped).Error)

	records := make([]models.Record, 0, cfg.RecordCount)
	statuses := []string{"new", "paid", "archived", "shipped"}
	categories := []string{"alpha", "beta", "gamma", "delta", "omega"}

	for i := 0; i < cfg.RecordCount; i++ {
		payload := map[string]any{
			"name":     fmt.Sprintf("user-%06d", i),
			"status":   statuses[i%len(statuses)],
			"category": categories[i%len(categories)],
			"score":    i % 1000,
			"payload": map[string]any{
				"index": i,
				"flags": []string{"a", "b", "c"},
			},
		}
		for extra := 0; extra < cfg.ExtraFieldCount; extra++ {
			payload[fmt.Sprintf("extra_%02d", extra)] = fmt.Sprintf("value-%d-%d", extra, i%17)
		}

		dataJSON, err := json.Marshal(payload)
		require.NoError(tb, err)

		records = append(records, models.Record{
			TableID: table.ID,
			Data:    models.JSONField(dataJSON),
			Version: 1,
		})
	}

	require.NoError(tb, f.DB.CreateInBatches(&records, 500).Error)
	cache.ClearAll()

	f.Database = database
	f.Table = table
	f.Fields = fields
	f.MasterToken = master
	f.ScopedToken = scoped
}
