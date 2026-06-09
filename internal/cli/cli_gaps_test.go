package cli

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", []string{}},
		{"whitespace only", "   ", []string{}},
		{"single value", "users", []string{"users"}},
		{"multiple values", "users,orders,products", []string{"users", "orders", "products"}},
		{"with spaces", " users , orders , products ", []string{"users", "orders", "products"}},
		{"trailing comma", "users,", []string{"users"}},
		{"leading comma", ",users", []string{"users"}},
		{"multiple commas", "users,,orders", []string{"users", "orders"}},
		{"all commas", ",,,", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCSV(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func newMigrationCmd() *cobra.Command {
	cmd := &cobra.Command{}
	registerMigrationFlags(cmd)
	return cmd
}

func TestLoadMigrationConfigFromCommand_ConfigFile(t *testing.T) {
	content := `source:
  type: mysql
  dsn: "user:pass@tcp(localhost:3306)/mydb"

target:
  database_name: "target_db"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("config", tmpFile.Name()))

	cfg, runnerOpts, err := loadMigrationConfigFromCommand(cmd)
	require.NoError(t, err)
	assert.Equal(t, "mysql", cfg.Source.Type)
	assert.Equal(t, "user:pass@tcp(localhost:3306)/mydb", cfg.Source.DSN)
	assert.Equal(t, "target_db", cfg.Target.DatabaseName)
	assert.Equal(t, "", runnerOpts.ResumeID)
}

func TestLoadMigrationConfigFromCommand_MissingSourceTypeAndDSN(t *testing.T) {
	cmd := newMigrationCmd()
	_, _, err := loadMigrationConfigFromCommand(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source-type")
	assert.Contains(t, err.Error(), "source-dsn")
}

func TestLoadMigrationConfigFromCommand_MissingSourceType(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("source-dsn", "user:pass@tcp(localhost:3306)/mydb"))
	_, _, err := loadMigrationConfigFromCommand(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source-type")
}

func TestLoadMigrationConfigFromCommand_MissingSourceDSN(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("source-type", "mysql"))
	_, _, err := loadMigrationConfigFromCommand(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source-dsn")
}

func TestLoadMigrationConfigFromCommand_WithDataSkipDataConflict(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("source-type", "mysql"))
	require.NoError(t, cmd.Flags().Set("source-dsn", "user:pass@tcp(localhost:3306)/mydb"))
	require.NoError(t, cmd.Flags().Set("with-data", "true"))
	require.NoError(t, cmd.Flags().Set("skip-data", "true"))
	_, _, err := loadMigrationConfigFromCommand(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "with-data")
	assert.Contains(t, err.Error(), "skip-data")
}

func TestLoadMigrationConfigFromCommand_SkipDataOverridesDefaultWithData(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("source-type", "sqlite"))
	require.NoError(t, cmd.Flags().Set("source-dsn", "./source.db"))
	require.NoError(t, cmd.Flags().Set("skip-data", "true"))

	cfg, _, err := loadMigrationConfigFromCommand(cmd)
	require.NoError(t, err)
	assert.False(t, cfg.Data.Enabled)
}

func TestLoadMigrationConfigFromCommand_FlagsOnly(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("source-type", "mysql"))
	require.NoError(t, cmd.Flags().Set("source-dsn", "user:pass@tcp(localhost:3306)/mydb"))
	require.NoError(t, cmd.Flags().Set("target-db", "my_target"))
	require.NoError(t, cmd.Flags().Set("include-tables", "users,orders"))
	require.NoError(t, cmd.Flags().Set("exclude-tables", "logs"))
	require.NoError(t, cmd.Flags().Set("with-data", "true"))
	require.NoError(t, cmd.Flags().Set("skip-data", "false"))
	require.NoError(t, cmd.Flags().Set("batch-size", "200"))
	require.NoError(t, cmd.Flags().Set("dry-run", "true"))
	require.NoError(t, cmd.Flags().Set("validate", "false"))
	require.NoError(t, cmd.Flags().Set("continue-on-error", "true"))
	require.NoError(t, cmd.Flags().Set("pagination-strategy", "offset"))
	require.NoError(t, cmd.Flags().Set("cursor-column", "id"))
	require.NoError(t, cmd.Flags().Set("checkpoint-interval", "50"))
	require.NoError(t, cmd.Flags().Set("rollback-on-failure", "none"))
	require.NoError(t, cmd.Flags().Set("max-concurrent-tables", "4"))
	require.NoError(t, cmd.Flags().Set("resume", "task-123"))

	cfg, runnerOpts, err := loadMigrationConfigFromCommand(cmd)
	require.NoError(t, err)

	assert.Equal(t, "mysql", cfg.Source.Type)
	assert.Equal(t, "user:pass@tcp(localhost:3306)/mydb", cfg.Source.DSN)
	assert.Equal(t, "my_target", cfg.Target.DatabaseName)
	assert.Equal(t, []string{"users", "orders"}, cfg.Tables.Include)
	assert.Equal(t, []string{"logs"}, cfg.Tables.Exclude)
	assert.True(t, cfg.Data.Enabled)
	assert.Equal(t, 200, cfg.Data.BatchSize)
	assert.Equal(t, "offset", string(cfg.Data.PaginationStrategy))
	assert.Equal(t, "id", cfg.Data.CursorColumn)
	assert.Equal(t, 4, cfg.Data.MaxConcurrentTables)
	assert.True(t, cfg.Options.DryRun)
	assert.False(t, cfg.Options.ValidateAfter)
	assert.True(t, cfg.Options.ContinueOnError)
	assert.Equal(t, 50, cfg.Options.CheckpointInterval)
	assert.Equal(t, "none", string(cfg.Options.RollbackOnFailure))
	assert.Equal(t, "task-123", runnerOpts.ResumeID)
}

func TestLoadMigrationConfigFromCommand_SkipDataOnly(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("source-type", "postgres"))
	require.NoError(t, cmd.Flags().Set("source-dsn", "host=localhost port=5432 user=postgres dbname=test sslmode=disable"))
	require.NoError(t, cmd.Flags().Set("with-data", "false"))
	require.NoError(t, cmd.Flags().Set("skip-data", "false"))

	cfg, _, err := loadMigrationConfigFromCommand(cmd)
	require.NoError(t, err)
	assert.False(t, cfg.Data.Enabled)
}

func TestLoadMigrationConfigFromCommand_ConfigFileNotFound(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("config", "/nonexistent/path/config.yaml"))
	_, _, err := loadMigrationConfigFromCommand(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}
func TestLoadMigrationConfigFromCommand_InvalidSourceType(t *testing.T) {
	cmd := newMigrationCmd()
	require.NoError(t, cmd.Flags().Set("source-type", "oracle"))
	require.NoError(t, cmd.Flags().Set("source-dsn", "some-dsn"))
	_, _, err := loadMigrationConfigFromCommand(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}
