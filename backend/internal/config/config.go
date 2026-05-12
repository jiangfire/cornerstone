package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
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
	LLMGovernorURL      string
	LLMGovernorToken    string `json:"-"`
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
			URL:         getEnv("DATABASE_URL", ":memory:"),
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
			LLMGovernorURL:      getEnv("LLM_GOVERNOR_URL", ""),
			LLMGovernorToken:    getEnv("LLM_GOVERNOR_TOKEN", ""),
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

	// JWT Secret 检查：release 模式必须显式配置强随机密钥；
	// 开发模式下若未配置则生成一次性临时密钥，并向 stderr 发出警告。
	if isWeakJWTSecret(c.JWT.Secret) {
		if c.IsProduction() {
			return fmt.Errorf("JWT_SECRET 未配置或仍为默认占位值，release 模式必须显式设置强随机密钥（建议 ≥32 字节）")
		}
		generated, err := generateSecureRandomString(48)
		if err != nil {
			return fmt.Errorf("生成临时 JWT_SECRET 失败: %w", err)
		}
		c.JWT.Secret = generated
		fmt.Fprintln(os.Stderr, "[WARN] JWT_SECRET 未配置或仍为默认占位值，已生成一次性临时密钥用于开发调试；")
		fmt.Fprintln(os.Stderr, "       重启后所有已签发 token 将失效。生产环境必须显式配置 JWT_SECRET。")
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
		url := strings.TrimSpace(c.Database.URL)
		// 处理内存数据库
		if url == "" || url == ":memory:" {
			c.Database.URL = ":memory:"
			return nil
		}
		// 处理文件数据库 - 如果已经配置了，不做修改
		if strings.HasPrefix(url, "file://") {
			return nil
		}
		// 如果不是绝对路径，转换为绝对路径
		if !filepath.IsAbs(url) {
			if absPath, err := os.Getwd(); err == nil {
				c.Database.URL = filepath.Join(absPath, url)
			}
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

// GetIntegrationURL 获取指定系统的集成 URL
func (c *Config) GetIntegrationURL(system string) string {
	return parseIntegrationValue(c.Integrations.BaseURLs, system)
}

// GetIntegrationToken 获取指定系统的集成 Token
func (c *Config) GetIntegrationToken(system string) string {
	return parseIntegrationValue(c.Integrations.OutboundTokens, system)
}

// parseIntegrationValue 从 "key=value,key2=value2" 格式中提取指定 key 的值
func parseIntegrationValue(input, key string) string {
	if input == "" {
		return ""
	}

	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == key {
			return strings.TrimSpace(kv[1])
		}
	}

	return ""
}

// 已知的 JWT_SECRET 默认/占位值，运行时若命中则视为未配置。
var weakJWTSecrets = map[string]struct{}{
	"":                                                   {},
	"change-this-secret-key":                             {},
	"your-secret-key-here":                               {},
	"dev-secret-key-change-in-production":                {},
	"cornerstone-default-secret-key-change-in-production": {},
}

// isWeakJWTSecret 判断 JWT secret 是否为空、占位值或长度过短。
func isWeakJWTSecret(secret string) bool {
	trimmed := strings.TrimSpace(secret)
	if _, ok := weakJWTSecrets[trimmed]; ok {
		return true
	}
	if len(trimmed) < 32 {
		return true
	}
	return false
}

// generateSecureRandomString 使用 crypto/rand 生成 base64url 编码的随机串。
// byteLen 指定原始随机字节数（输出长度约为 ⌈byteLen*4/3⌉）。
func generateSecureRandomString(byteLen int) (string, error) {
	if byteLen <= 0 {
		byteLen = 32
	}
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
