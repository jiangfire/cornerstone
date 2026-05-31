package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type PaginationMode string

const (
	PaginationCursor PaginationMode = "cursor"
	PaginationOffset PaginationMode = "offset"
)

type RollbackMode string

const (
	RollbackTable RollbackMode = "table"
	RollbackNone  RollbackMode = "none"
)

type Config struct {
	Source  SourceConfig  `yaml:"source" json:"source"`
	Target  TargetConfig  `yaml:"target" json:"target"`
	Tables  TablesConfig  `yaml:"tables" json:"tables"`
	Data    DataConfig    `yaml:"data" json:"data"`
	Mapping MappingConfig `yaml:"mapping" json:"mapping"`
	Options OptionsConfig `yaml:"options" json:"options"`
}

type SourceConfig struct {
	Type         string            `yaml:"type" json:"type"`
	DSN          string            `yaml:"dsn" json:"dsn"`
	Host         string            `yaml:"host" json:"host"`
	Port         int               `yaml:"port" json:"port"`
	User         string            `yaml:"user" json:"user"`
	Password     string            `yaml:"password" json:"password"`
	Database     string            `yaml:"database" json:"database"`
	Params       map[string]string `yaml:"params" json:"params"`
	ReadOnlyHint bool              `yaml:"read_only_hint" json:"read_only_hint"`
}

type TargetConfig struct {
	DatabaseName string `yaml:"database_name" json:"database_name"`
}

type TablesConfig struct {
	Include []string          `yaml:"include" json:"include"`
	Exclude []string          `yaml:"exclude" json:"exclude"`
	Rename  map[string]string `yaml:"rename" json:"rename"`
}

type DataConfig struct {
	Enabled             bool              `yaml:"enabled" json:"enabled"`
	BatchSize           int               `yaml:"batch_size" json:"batch_size"`
	PaginationStrategy  PaginationMode    `yaml:"pagination_strategy" json:"pagination_strategy"`
	CursorColumn        string            `yaml:"cursor_column" json:"cursor_column"`
	Filters             map[string]string `yaml:"filters" json:"filters"`
	MaxConcurrentTables int               `yaml:"max_concurrent_tables" json:"max_concurrent_tables"`
}

type MappingConfig struct {
	Overrides map[string]string `yaml:"overrides" json:"overrides"`
}

type OptionsConfig struct {
	DryRun             bool         `yaml:"dry_run" json:"dry_run"`
	ContinueOnError    bool         `yaml:"continue_on_error" json:"continue_on_error"`
	LogLevel           string       `yaml:"log_level" json:"log_level"`
	ValidateAfter      bool         `yaml:"validate_after" json:"validate_after"`
	CheckpointInterval int          `yaml:"checkpoint_interval" json:"checkpoint_interval"`
	RollbackOnFailure  RollbackMode `yaml:"rollback_on_failure" json:"rollback_on_failure"`
}

func DefaultConfig() Config {
	return Config{
		Tables: TablesConfig{
			Include: []string{},
			Exclude: []string{},
			Rename:  map[string]string{},
		},
		Data: DataConfig{
			Enabled:             true,
			BatchSize:           500,
			PaginationStrategy:  PaginationCursor,
			Filters:             map[string]string{},
			MaxConcurrentTables: 1,
		},
		Mapping: MappingConfig{
			Overrides: map[string]string{},
		},
		Options: OptionsConfig{
			LogLevel:           "info",
			ValidateAfter:      true,
			CheckpointInterval: 100,
			RollbackOnFailure:  RollbackTable,
		},
	}
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, newMigrationError(ErrCodeConfigInvalid, "解析配置文件失败", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, newMigrationError(ErrCodeConfigInvalid, "配置文件验证失败", err)
	}
	return cfg, nil
}

func (c *Config) ApplyTypeMapOverrideFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取类型映射文件失败: %w", err)
	}

	overrides := map[string]string{}
	if err := json.Unmarshal(data, &overrides); err != nil {
		return fmt.Errorf("解析类型映射文件失败: %w", err)
	}
	if c.Mapping.Overrides == nil {
		c.Mapping.Overrides = map[string]string{}
	}
	for k, v := range overrides {
		c.Mapping.Overrides[k] = v
	}
	return nil
}

func (c Config) Validate() error {
	sourceType := normalizeLower(c.Source.Type)
	switch sourceType {
	case "mysql", "postgres", "sqlite":
	default:
		return fmt.Errorf("source.type %s 不支持: %s", ErrCodeUnsupportedSource, c.Source.Type)
	}

	if c.hasSourceDSN() && c.hasSourceFields() {
		return errors.New("source.dsn 与分字段连接配置互斥")
	}
	if !c.hasSourceDSN() && !c.hasSourceFields() {
		return errors.New("必须提供 source.dsn 或分字段连接配置")
	}

	if sourceType == "sqlite" {
		if c.Source.DSN == "" && strings.TrimSpace(c.Source.Database) == "" {
			return errors.New("sqlite 源必须提供 source.dsn 或 source.database")
		}
	} else if c.Source.DSN == "" {
		if strings.TrimSpace(c.Source.Host) == "" || strings.TrimSpace(c.Source.User) == "" || strings.TrimSpace(c.Source.Database) == "" {
			return errors.New("非 sqlite 源需要完整的 host/user/database 配置")
		}
	}

	if c.Data.BatchSize <= 0 {
		return errors.New("data.batch_size 必须大于 0")
	}
	switch c.Data.PaginationStrategy {
	case "", PaginationCursor, PaginationOffset:
	default:
		return fmt.Errorf("data.pagination_strategy 不支持: %s", c.Data.PaginationStrategy)
	}
	if c.Data.MaxConcurrentTables <= 0 {
		return errors.New("data.max_concurrent_tables 必须大于 0")
	}
	if c.Options.CheckpointInterval <= 0 {
		return errors.New("options.checkpoint_interval 必须大于 0")
	}
	switch c.Options.RollbackOnFailure {
	case "", RollbackTable, RollbackNone:
	default:
		return fmt.Errorf("options.rollback_on_failure 不支持: %s", c.Options.RollbackOnFailure)
	}

	return nil
}

func CheckConfigFileWarnings(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return []string{fmt.Sprintf("无法读取配置文件权限: %v", err)}
	}
	if os.PathSeparator == '\\' {
		return nil
	}
	if info.Mode().Perm()&0o077 != 0 {
		return []string{"配置文件包含敏感信息时建议权限为 0600"}
	}
	return nil
}

func (c Config) BuildSourceDSN() string {
	if strings.TrimSpace(c.Source.DSN) != "" {
		return c.Source.DSN
	}

	switch normalizeLower(c.Source.Type) {
	case "sqlite":
		return c.Source.Database
	case "mysql":
		params := ""
		if len(c.Source.Params) > 0 {
			parts := make([]string, 0, len(c.Source.Params))
			for key, value := range c.Source.Params {
				parts = append(parts, fmt.Sprintf("%s=%s", key, value))
			}
			params = "?" + strings.Join(parts, "&")
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s%s", c.Source.User, c.Source.Password, c.Source.Host, c.Source.Port, c.Source.Database, params)
	case "postgres":
		parts := []string{
			fmt.Sprintf("host=%s", c.Source.Host),
			fmt.Sprintf("port=%d", c.Source.Port),
			fmt.Sprintf("user=%s", c.Source.User),
			fmt.Sprintf("password=%s", c.Source.Password),
			fmt.Sprintf("dbname=%s", c.Source.Database),
			"sslmode=disable",
		}
		for key, value := range c.Source.Params {
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
		return strings.Join(parts, " ")
	default:
		return c.Source.DSN
	}
}

func (c Config) EffectiveTargetDatabase() string {
	if strings.TrimSpace(c.Target.DatabaseName) != "" {
		return c.Target.DatabaseName
	}

	if strings.TrimSpace(c.Source.Database) != "" {
		return c.Source.Database
	}
	if strings.TrimSpace(c.Source.DSN) != "" && normalizeLower(c.Source.Type) == "sqlite" {
		base := filepath.Base(c.Source.DSN)
		ext := filepath.Ext(base)
		return strings.TrimSuffix(base, ext)
	}
	return "migration_target"
}

func (c Config) hasSourceDSN() bool {
	return strings.TrimSpace(c.Source.DSN) != ""
}

func (c Config) hasSourceFields() bool {
	return strings.TrimSpace(c.Source.Host) != "" ||
		c.Source.Port != 0 ||
		strings.TrimSpace(c.Source.User) != "" ||
		strings.TrimSpace(c.Source.Password) != "" ||
		strings.TrimSpace(c.Source.Database) != ""
}

func normalizeLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
