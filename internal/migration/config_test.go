package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "migration.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
source:
  type: sqlite
  dsn: ./source.db
target:
  database_name: migrated_db
`), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "sqlite", cfg.Source.Type)
	assert.Equal(t, "./source.db", cfg.Source.DSN)
	assert.Equal(t, "migrated_db", cfg.Target.DatabaseName)
	assert.True(t, cfg.Data.Enabled)
	assert.Equal(t, 500, cfg.Data.BatchSize)
	assert.Equal(t, PaginationCursor, cfg.Data.PaginationStrategy)
	assert.Equal(t, 1, cfg.Data.MaxConcurrentTables)
	assert.True(t, cfg.Options.ValidateAfter)
	assert.Equal(t, 100, cfg.Options.CheckpointInterval)
	assert.Equal(t, RollbackTable, cfg.Options.RollbackOnFailure)
}

func TestLoadConfig_RejectsInvalidSourceType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "migration.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
source:
  type: oracle
  dsn: example
`), 0o600))

	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source.type")
}

func TestConfigValidate_RejectsConflictingSourceSettings(t *testing.T) {
	cfg := Config{
		Source: SourceConfig{
			Type:     "mysql",
			DSN:      "dsn",
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "pass",
			Database: "demo",
		},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           100,
			PaginationStrategy:  PaginationCursor,
			MaxConcurrentTables: 1,
		},
		Options: OptionsConfig{
			CheckpointInterval: 100,
			RollbackOnFailure:  RollbackTable,
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source.dsn")
}
