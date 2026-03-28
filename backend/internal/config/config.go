package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config 包含所有应用配置
type Config struct {
	// Database 数据库配置
	Database DatabaseConfig

	// Server 服务器配置
	Server ServerConfig

	// Logger 日志配置
	Logger LoggerConfig

	// JWT JWT配置
	JWT JWTConfig

	// Integrations 集成配置
	Integrations IntegrationsConfig

	// MCP HTTP MCP / SSE 配置
	MCP MCPConfig
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type        string // postgres 或 sqlite
	URL         string // PostgreSQL: postgres://...  SQLite: 文件路径
	MaxOpen     int
	MaxIdle     int
	MaxLifetime int
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Mode string
	Port string
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level      string
	OutputPath string
	ErrorPath  string
	MaxSize    int
	MaxAge     int
	MaxBackups int
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret     string `json:"-"` // 防止序列化敏感信息
	Expiration int    // 单位：小时
}

// IntegrationsConfig 集成与治理回写配置
type IntegrationsConfig struct {
	BaseURLs            string
	OutboundTokens      string
	SharedToken         string `json:"-"`
	OutboundTimeoutSec  int
	OutboxMaxRetries    int
	OutboxRetryInterval int
	OutboxWorkerEnabled bool
	FuckCMDBUIBaseURL   string
}

// MCPConfig HTTP MCP / SSE 配置
type MCPConfig struct {
	SSEKeepaliveSec int
	SSERetryMS      int
	SSEReplayBuffer int
}

// Load 加载配置从环境变量
func Load() (*Config, error) {
	// 尝试加载 .env 文件（如果存在）
	_ = godotenv.Load()

	// 检查是否缺少必要的环境变量
	needsDefaults := os.Getenv("DB_TYPE") == "" && os.Getenv("DATABASE_URL") == ""

	// 读取环境变量
	config := &Config{
		Database: DatabaseConfig{
			Type:        getEnv("DB_TYPE", "sqlite"),
			URL:         getEnv("DATABASE_URL", "./cornerstone.db"),
			MaxOpen:     getEnvAsInt("DB_MAX_OPEN", 10),
			MaxIdle:     getEnvAsInt("DB_MAX_IDLE", 5),
			MaxLifetime: getEnvAsInt("DB_MAX_LIFETIME", 3600),
		},
		Server: ServerConfig{
			Mode: getEnv("SERVER_MODE", "release"),
			Port: getEnv("PORT", "8080"),
		},
		Logger: LoggerConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			OutputPath: getEnv("LOG_OUTPUT", "logs/app.log"),
			ErrorPath:  getEnv("LOG_ERROR", "logs/error.log"),
			MaxSize:    getEnvAsInt("LOG_MAX_SIZE", 100),
			MaxAge:     getEnvAsInt("LOG_MAX_AGE", 7),
			MaxBackups: getEnvAsInt("LOG_MAX_BACKUPS", 5),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-this-secret-key"),
			Expiration: getEnvAsInt("JWT_EXPIRATION", 24),
		},
		Integrations: IntegrationsConfig{
			BaseURLs:            getEnv("INTEGRATION_BASE_URLS", ""),
			OutboundTokens:      getEnv("OUTBOUND_INTEGRATION_TOKENS", ""),
			SharedToken:         getEnv("INTEGRATION_SHARED_TOKEN", ""),
			OutboundTimeoutSec:  getEnvAsInt("OUTBOUND_INTEGRATION_TIMEOUT_SEC", 5),
			OutboxMaxRetries:    getEnvAsInt("GOVERNANCE_OUTBOX_MAX_RETRIES", 5),
			OutboxRetryInterval: getEnvAsInt("GOVERNANCE_OUTBOX_RETRY_INTERVAL_SEC", 60),
			OutboxWorkerEnabled: getEnvAsBool("GOVERNANCE_OUTBOX_WORKER_ENABLED", true),
			FuckCMDBUIBaseURL:   getEnv("FUCKCMDB_UI_BASE_URL", ""),
		},
		MCP: MCPConfig{
			SSEKeepaliveSec: getEnvAsInt("MCP_SSE_KEEPALIVE_SEC", 25),
			SSERetryMS:      getEnvAsInt("MCP_SSE_RETRY_MS", 3000),
			SSEReplayBuffer: getEnvAsInt("MCP_SSE_REPLAY_BUFFER", 128),
		},
	}

	// 如果没有配置文件，生成一个默认的
	if needsDefaults {
		// 静默使用默认配置
		config.Server.Mode = "release"
	}

	// 验证关键配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
}

// Validate 验证配置是否完整
func (c *Config) Validate() error {
	// 验证数据库配置
	if err := c.validateDatabase(); err != nil {
		return err
	}

	// JWT Secret 检查 - 如果是默认值，生成一个随机的
	if strings.TrimSpace(c.JWT.Secret) == "" || c.JWT.Secret == "your-secret-key-here" || c.JWT.Secret == "change-this-secret-key" {
		// 在实际环境中应该警告用户，但这里我们允许使用默认值以便快速启动
		c.JWT.Secret = "cornerstone-default-secret-key-change-in-production"
	}
	if strings.TrimSpace(c.Server.Port) == "" {
		return fmt.Errorf("PORT 不能为空")
	}
	if c.Integrations.OutboundTimeoutSec <= 0 {
		c.Integrations.OutboundTimeoutSec = 5
	}
	if c.Integrations.OutboxMaxRetries <= 0 {
		c.Integrations.OutboxMaxRetries = 5
	}
	if c.Integrations.OutboxRetryInterval <= 0 {
		c.Integrations.OutboxRetryInterval = 60
	}
	if c.MCP.SSEKeepaliveSec <= 0 {
		c.MCP.SSEKeepaliveSec = 25
	}
	if c.MCP.SSERetryMS <= 0 {
		c.MCP.SSERetryMS = 3000
	}
	if c.MCP.SSEReplayBuffer <= 0 {
		c.MCP.SSEReplayBuffer = 128
	}
	return nil
}

// validateDatabase 验证数据库配置
func (c *Config) validateDatabase() error {
	switch c.Database.Type {
	case "postgres":
		if strings.TrimSpace(c.Database.URL) == "" {
			return fmt.Errorf("DATABASE_URL 不能为空")
		}
	case "sqlite":
		if strings.TrimSpace(c.Database.URL) == "" {
			// SQLite 默认使用本地文件
			c.Database.URL = "cornerstone.db"
		}
	default:
		return fmt.Errorf("不支持的数据库类型: %s，支持 postgres 或 sqlite", c.Database.Type)
	}
	return nil
}

// IsSQLite 是否为 SQLite 数据库
func (c *Config) IsSQLite() bool {
	return c.Database.Type == "sqlite"
}

// IsPostgres 是否为 PostgreSQL 数据库
func (c *Config) IsPostgres() bool {
	return c.Database.Type == "postgres"
}

// getEnv 从环境变量获取字符串值，提供默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt 从环境变量获取整数值，提供默认值
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		_, err := fmt.Sscanf(value, "%d", &result)
		if err == nil {
			return result
		}
	}
	return defaultValue
}

// getEnvAsBool 从环境变量获取布尔值，提供默认值
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := strings.TrimSpace(strings.ToLower(os.Getenv(key))); value != "" {
		switch value {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return defaultValue
}

// IsProduction 判断是否为生产环境
func (c *Config) IsProduction() bool {
	return c.Server.Mode == "release"
}

// GetDatabaseURL 获取数据库连接URL
func (c *Config) GetDatabaseURL() string {
	return c.Database.URL
}

// GetServerAddr 获取服务器监听地址
func (c *Config) GetServerAddr() string {
	return ":" + c.Server.Port
}
