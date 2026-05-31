package cli

import (
	"testing"

	mig "github.com/jiangfire/cornerstone/internal/migration"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMigrationConfigFromCommand_ParsesAdvancedFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "migration"}
	registerMigrationFlags(cmd)
	require.NoError(t, cmd.ParseFlags([]string{
		"--source-type", "sqlite",
		"--source-dsn", "./source.db",
		"--pagination-strategy", "offset",
		"--cursor-column", "custom_id",
		"--checkpoint-interval", "50",
		"--rollback-on-failure", "none",
		"--max-concurrent-tables", "3",
	}))

	cfg, _, err := loadMigrationConfigFromCommand(cmd)
	require.NoError(t, err)
	assert.Equal(t, mig.PaginationOffset, cfg.Data.PaginationStrategy)
	assert.Equal(t, "custom_id", cfg.Data.CursorColumn)
	assert.Equal(t, 50, cfg.Options.CheckpointInterval)
	assert.Equal(t, mig.RollbackNone, cfg.Options.RollbackOnFailure)
	assert.Equal(t, 3, cfg.Data.MaxConcurrentTables)
}
