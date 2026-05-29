package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config 包含所有应用配置
type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
	Logger   LoggerConfig
	LLM      LLMConfig
	MCP      MCPConfig
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type        string
	URL         string
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
	Level string
}

// LLMConfig LLM 配置
type LLMConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

// MCPConfig HTTP MCP / SSE 配置
type MCPConfig struct {
	SSEKeepaliveSec int
	SSERetryMS      int
	SSEReplayBuffer int
}

func loadEnvFiles() {
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "backend", ".env"))
	}
	for _, p := range paths {
		_ = godotenv.Load(p)
	}
}

// Load 加载配置
func Load() (*Config, error) {
	loadEnvFiles()

	config := &Config{
		Database: DatabaseConfig{
			Type:        getEnv("DB_TYPE", "sqlite"),
			URL:         getEnv("DATABASE_URL", ""),
			MaxOpen:     getEnvAsInt("DB_MAX_OPEN", 10),
			MaxIdle:     getEnvAsInt("DB_MAX_IDLE", 5),
			MaxLifetime: getEnvAsInt("DB_MAX_LIFETIME", 3600),
		},
		Server: ServerConfig{
			Mode: getEnv("SERVER_MODE", "release"),
			Port: getEnv("PORT", "8080"),
		},
		Logger: LoggerConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		LLM: LLMConfig{
			APIKey:  getEnv("LLM_API_KEY", ""),
			Model:   getEnv("LLM_MODEL", "gpt-4o"),
			BaseURL: getEnv("LLM_BASE_URL", ""),
		},
		MCP: MCPConfig{
			SSEKeepaliveSec: getEnvAsInt("MCP_SSE_KEEPALIVE_SEC", 25),
			SSERetryMS:      getEnvAsInt("MCP_SSE_RETRY_MS", 3000),
			SSEReplayBuffer: getEnvAsInt("MCP_SSE_REPLAY_BUFFER", 128),
		},
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	switch c.Database.Type {
	case "postgres":
		if strings.TrimSpace(c.Database.URL) == "" {
			return fmt.Errorf("DATABASE_URL 不能为空")
		}
	case "sqlite":
		url := strings.TrimSpace(c.Database.URL)
		if url == "" {
			c.Database.URL = "./cornerstone.db"
		} else if url == ":memory:" {
			// OK
		} else if strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
			return fmt.Errorf("DATABASE_URL 看起来是 PostgreSQL 连接串，但 DB_TYPE 配置为 sqlite")
		} else if !filepath.IsAbs(url) && url != ":memory:" {
			if absPath, err := os.Getwd(); err == nil {
				c.Database.URL = filepath.Join(absPath, url)
			}
		}
	default:
		return fmt.Errorf("不支持的数据库类型: %s，支持 postgres 或 sqlite")
	}

	if strings.TrimSpace(c.Server.Port) == "" {
		return fmt.Errorf("PORT 不能为空")
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

func (c *Config) IsSQLite() bool {
	return c.Database.Type == "sqlite"
}

func (c *Config) IsPostgres() bool {
	return c.Database.Type == "postgres"
}

func (c *Config) GetServerAddr() string {
	return ":" + c.Server.Port
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

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
