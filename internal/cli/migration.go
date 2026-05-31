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
	Short: "外部数据库迁移",
	Long:  "将外部关系型数据库结构和数据迁移到 Cornerstone。",
}

var migrationRunCmd = &cobra.Command{
	Use:   "run",
	Short: "执行迁移",
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
	Short: "预览迁移计划",
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
	Short: "输出配置模板",
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
	Short: "管理迁移配置",
}

var migrationConfigCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建配置模板文件",
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
	Short: "校验配置文件",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("config")
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("请通过 --config 指定配置文件")
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
	Short: "列出当前目录下的迁移配置文件",
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

	migrationTemplateCmd.Flags().StringP("output", "o", "", "输出到指定文件")
	migrationConfigCreateCmd.Flags().StringP("output", "o", "migration.yaml", "输出配置文件路径")
	migrationConfigValidateCmd.Flags().StringP("config", "c", "", "迁移配置文件路径")
}

func registerMigrationFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("config", "c", "", "迁移配置文件路径")
	cmd.Flags().String("source-type", "", "源数据库类型：mysql|postgres|sqlite")
	cmd.Flags().String("source-dsn", "", "源数据库连接 DSN")
	cmd.Flags().String("target-db", "", "目标 Cornerstone Database 名称")
	cmd.Flags().String("include-tables", "", "要迁移的表，逗号分隔")
	cmd.Flags().String("exclude-tables", "", "要排除的表，逗号分隔")
	cmd.Flags().Bool("with-data", true, "迁移数据")
	cmd.Flags().Bool("skip-data", false, "仅迁移结构")
	cmd.Flags().Int("batch-size", 500, "批量读取大小")
	cmd.Flags().Bool("dry-run", false, "空跑模式，仅输出计划")
	cmd.Flags().String("type-map-override", "", "自定义类型映射 JSON 文件")
	cmd.Flags().String("resume", "", "从指定迁移任务 ID 恢复")
	cmd.Flags().Bool("validate", true, "迁移后校验")
	cmd.Flags().Bool("continue-on-error", false, "单表错误后继续其他表")
	cmd.Flags().String("pagination-strategy", "", "分页策略：cursor|offset")
	cmd.Flags().String("cursor-column", "", "指定游标列")
	cmd.Flags().Int("checkpoint-interval", 100, "每处理 N 条记录持久化一次位点")
	cmd.Flags().String("rollback-on-failure", "", "失败回滚策略：table|none")
	cmd.Flags().Int("max-concurrent-tables", 1, "同时迁移的表数")
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
		return mig.Config{}, runnerOpts, fmt.Errorf("未提供 --config 时，必须指定 --source-type 和 --source-dsn")
	}
	if withData && skipData {
		return mig.Config{}, runnerOpts, fmt.Errorf("--with-data 与 --skip-data 不能同时为 true")
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
