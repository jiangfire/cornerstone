package testutil

import (
	"path/filepath"
	"testing"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/stretchr/testify/require"
)

func TestResolveBenchmarkDatabaseConfigDefaultsToSQLiteTempFile(t *testing.T) {
	t.Setenv("DB_TYPE", "")
	t.Setenv("DATABASE_URL", "")

	cfg, err := resolveBenchmarkDatabaseConfig(t)
	require.NoError(t, err)
	require.Equal(t, "sqlite", cfg.Type)
	require.NotEmpty(t, cfg.URL)
	require.True(t, filepath.IsAbs(cfg.URL))
	require.Contains(t, cfg.URL, "cornerstone-bench.sqlite")
}

func TestResolveBenchmarkDatabaseConfigUsesExplicitEnv(t *testing.T) {
	t.Setenv("DB_TYPE", "postgres")
	t.Setenv("DATABASE_URL", "host=127.0.0.1 port=5432 user=postgres password=postgres dbname=cornerstone_test sslmode=disable")

	cfg, err := resolveBenchmarkDatabaseConfig(t)
	require.NoError(t, err)
	require.Equal(t, "postgres", cfg.Type)
	require.Equal(t, "host=127.0.0.1 port=5432 user=postgres password=postgres dbname=cornerstone_test sslmode=disable", cfg.URL)
}

func TestResolveBenchmarkDatabaseConfigRejectsMissingURLForServerDB(t *testing.T) {
	t.Setenv("DB_TYPE", "mysql")
	t.Setenv("DATABASE_URL", "")

	_, err := resolveBenchmarkDatabaseConfig(t)
	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL")
}

func TestSetupBenchmarkFixtureSeedsSQLite(t *testing.T) {
	t.Setenv("DB_TYPE", "")
	t.Setenv("DATABASE_URL", "")

	fixture := SetupBenchmarkFixture(t, BenchmarkSeedConfig{
		RecordCount:     12,
		ExtraFieldCount: 3,
	})

	require.Equal(t, "sqlite", fixture.DB.Name())
	require.NotNil(t, fixture.Database)
	require.NotNil(t, fixture.Table)
	require.Len(t, fixture.Fields, 8)

	var recordCount int64
	require.NoError(t, fixture.DB.Model(&models.Record{}).Count(&recordCount).Error)
	require.EqualValues(t, 12, recordCount)
}

func TestSetupBenchmarkFixtureSeedsNoiseTables(t *testing.T) {
	t.Setenv("DB_TYPE", "")
	t.Setenv("DATABASE_URL", "")

	fixture := SetupBenchmarkFixture(t, BenchmarkSeedConfig{
		RecordCount:          12,
		ExtraFieldCount:      2,
		NoiseTableCount:      3,
		NoiseRecordsPerTable: 5,
	})

	require.Len(t, fixture.NoiseTables, 3)

	var tableCount int64
	require.NoError(t, fixture.DB.Model(&models.Table{}).Count(&tableCount).Error)
	require.EqualValues(t, 4, tableCount)

	var recordCount int64
	require.NoError(t, fixture.DB.Model(&models.Record{}).Count(&recordCount).Error)
	require.EqualValues(t, 27, recordCount)

	for _, noiseTable := range fixture.NoiseTables {
		var noiseFieldCount int64
		require.NoError(t, fixture.DB.Model(&models.Field{}).Where("table_id = ?", noiseTable.ID).Count(&noiseFieldCount).Error)
		require.EqualValues(t, len(fixture.Fields), noiseFieldCount)
	}
}
