package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appdb "github.com/jiangfire/cornerstone/internal/db"
	mig "github.com/jiangfire/cornerstone/internal/migration"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
)

var migrationCmd = &cobra.Command{
	Use:   "migration",
	Short: "external database migration",
	Long:  "Migrate external relational database schema and data to Cornerstone.",
}

var migrationRunCmd = &cobra.Command{
	Use:   "run",
	Short: "run migration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, runnerOpts, err := loadMigrationConfigFromCommand(cmd)
		if err != nil {
			return err
		}

		if cfg.Options.DryRun {
			runner, err := mig.NewRunner(nil, "", cfg, runnerOpts)
			if err != nil {
				return err
			}
			plan, err := runner.Preview()
			if err != nil {
				return err
			}
			return printJSON(plan)
		}

		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		runner, err := mig.NewRunner(db.DB(), token, cfg, runnerOpts)
		if err != nil {
			return err
		}
		report, err := runner.Run()
		if err != nil {
			return err
		}
		return printJSON(report)
	},
}

var migrationPreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "preview migration plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, runnerOpts, err := loadMigrationConfigFromCommand(cmd)
		if err != nil {
			return err
		}
		runner, err := mig.NewRunner(nil, "", cfg, runnerOpts)
		if err != nil {
			return err
		}
		plan, err := runner.Preview()
		if err != nil {
			return err
		}
		return printJSON(plan)
	},
}

var migrationTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "output config template",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		if strings.TrimSpace(output) == "" {
			fmt.Print(mig.DefaultTemplate)
			return nil
		}
		return os.WriteFile(output, []byte(mig.DefaultTemplate), 0o600)
	},
}

var migrationConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "manage migration config",
}

var migrationConfigCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create config template file",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		if strings.TrimSpace(output) == "" {
			output = "migration.yaml"
		}
		return os.WriteFile(output, []byte(mig.DefaultTemplate), 0o600)
	},
}

var migrationConfigValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "validate config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("config")
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("please specify config file with --config")
		}
		cfg, err := mig.LoadConfig(path)
		if err != nil {
			return err
		}
		return printJSON(map[string]interface{}{
			"valid":    true,
			"path":     path,
			"source":   cfg.Source.Type,
			"target":   cfg.EffectiveTargetDatabase(),
			"warnings": mig.CheckConfigFileWarnings(path),
		})
	},
}

var migrationConfigListCmd = &cobra.Command{
	Use:   "list",
	Short: "list migration config files in current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		matches := make([]string, 0)
		for _, pattern := range []string{"migration*.yaml", "migration*.yml"} {
			files, err := filepath.Glob(pattern)
			if err != nil {
				return err
			}
			matches = append(matches, files...)
		}
		return printJSON(matches)
	},
}

func init() {
	rootCmd.AddCommand(migrationCmd)
	migrationCmd.AddCommand(migrationRunCmd)
	migrationCmd.AddCommand(migrationPreviewCmd)
	migrationCmd.AddCommand(migrationTemplateCmd)
	migrationCmd.AddCommand(migrationConfigCmd)
	migrationConfigCmd.AddCommand(migrationConfigCreateCmd)
	migrationConfigCmd.AddCommand(migrationConfigValidateCmd)
	migrationConfigCmd.AddCommand(migrationConfigListCmd)

	registerMigrationFlags(migrationRunCmd)
	registerMigrationFlags(migrationPreviewCmd)

	migrationTemplateCmd.Flags().StringP("output", "o", "", "output to specified file")
	migrationConfigCreateCmd.Flags().StringP("output", "o", "migration.yaml", "output config file path")
	migrationConfigValidateCmd.Flags().StringP("config", "c", "", "migration config file path")
}

func registerMigrationFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("config", "c", "", "migration config file path")
	cmd.Flags().String("source-type", "", "source database type: mysql|postgres|sqlite")
	cmd.Flags().String("source-dsn", "", "source database connection DSN")
	cmd.Flags().String("target-db", "", "target Cornerstone database name")
	cmd.Flags().String("include-tables", "", "tables to migrate, comma-separated")
	cmd.Flags().String("exclude-tables", "", "tables to exclude, comma-separated")
	cmd.Flags().Bool("with-data", true, "migrate data")
	cmd.Flags().Bool("skip-data", false, "schema only")
	cmd.Flags().Int("batch-size", 500, "batch read size")
	cmd.Flags().Bool("dry-run", false, "dry-run mode, output plan only")
	cmd.Flags().String("type-map-override", "", "custom type mapping JSON file")
	cmd.Flags().String("resume", "", "resume from migration task ID")
	cmd.Flags().Bool("validate", true, "validate after migration")
	cmd.Flags().Bool("continue-on-error", false, "continue with other tables on single table error")
	cmd.Flags().String("pagination-strategy", "", "pagination strategy: cursor|offset")
	cmd.Flags().String("cursor-column", "", "cursor column")
	cmd.Flags().Int("checkpoint-interval", 100, "checkpoint every N records")
	cmd.Flags().String("rollback-on-failure", "", "rollback strategy on failure: table|none")
	cmd.Flags().Int("max-concurrent-tables", 1, "max concurrent tables")
}

func loadMigrationConfigFromCommand(cmd *cobra.Command) (mig.Config, mig.RunnerOptions, error) {
	configPath, _ := cmd.Flags().GetString("config")
	resumeID, _ := cmd.Flags().GetString("resume")
	runnerOpts := mig.RunnerOptions{
		ResumeID: resumeID,
	}

	if strings.TrimSpace(configPath) != "" {
		cfg, err := mig.LoadConfig(configPath)
		if err != nil {
			return mig.Config{}, runnerOpts, err
		}
		overridePath, _ := cmd.Flags().GetString("type-map-override")
		if err := cfg.ApplyTypeMapOverrideFile(overridePath); err != nil {
			return mig.Config{}, runnerOpts, err
		}
		return cfg, runnerOpts, nil
	}

	cfg := mig.DefaultConfig()
	sourceType, _ := cmd.Flags().GetString("source-type")
	sourceDSN, _ := cmd.Flags().GetString("source-dsn")
	targetDB, _ := cmd.Flags().GetString("target-db")
	includeTables, _ := cmd.Flags().GetString("include-tables")
	excludeTables, _ := cmd.Flags().GetString("exclude-tables")
	withData, _ := cmd.Flags().GetBool("with-data")
	skipData, _ := cmd.Flags().GetBool("skip-data")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	validate, _ := cmd.Flags().GetBool("validate")
	continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
	paginationStrategy, _ := cmd.Flags().GetString("pagination-strategy")
	cursorColumn, _ := cmd.Flags().GetString("cursor-column")
	checkpointInterval, _ := cmd.Flags().GetInt("checkpoint-interval")
	rollbackOnFailure, _ := cmd.Flags().GetString("rollback-on-failure")
	maxConcurrentTables, _ := cmd.Flags().GetInt("max-concurrent-tables")
	overridePath, _ := cmd.Flags().GetString("type-map-override")

	if strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceDSN) == "" {
		return mig.Config{}, runnerOpts, fmt.Errorf("--source-type and --source-dsn are required when --config is not provided")
	}
	if withData && skipData {
		return mig.Config{}, runnerOpts, fmt.Errorf("--with-data and --skip-data cannot both be true")
	}

	cfg.Source.Type = sourceType
	cfg.Source.DSN = sourceDSN
	cfg.Target.DatabaseName = targetDB
	cfg.Tables.Include = splitCSV(includeTables)
	cfg.Tables.Exclude = splitCSV(excludeTables)
	cfg.Data.Enabled = withData && !skipData
	cfg.Data.BatchSize = batchSize
	cfg.Data.CursorColumn = cursorColumn
	cfg.Data.MaxConcurrentTables = maxConcurrentTables
	cfg.Options.DryRun = dryRun
	cfg.Options.ValidateAfter = validate
	cfg.Options.ContinueOnError = continueOnError
	cfg.Options.CheckpointInterval = checkpointInterval

	if strings.TrimSpace(paginationStrategy) != "" {
		cfg.Data.PaginationStrategy = mig.PaginationMode(strings.TrimSpace(paginationStrategy))
	}
	if strings.TrimSpace(rollbackOnFailure) != "" {
		cfg.Options.RollbackOnFailure = mig.RollbackMode(strings.TrimSpace(rollbackOnFailure))
	}

	if err := cfg.ApplyTypeMapOverrideFile(overridePath); err != nil {
		return mig.Config{}, runnerOpts, err
	}
	if err := cfg.Validate(); err != nil {
		return mig.Config{}, runnerOpts, err
	}

	return cfg, runnerOpts, nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
