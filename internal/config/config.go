package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Database    DatabaseConfig
	Server      ServerConfig
	Logger      LoggerConfig
	LLM         LLMConfig
	MCP         MCPConfig
	FileStorage FileStorageConfig
}

// DatabaseConfig is the database configuration
type DatabaseConfig struct {
	Type        string
	URL         string
	MaxOpen     int
	MaxIdle     int
	MaxLifetime int
}

// ServerConfig is the server configuration
type ServerConfig struct {
	Mode string
	Port string
}

// LoggerConfig is the logger configuration
type LoggerConfig struct {
	Level string
}

// LLMConfig is the LLM configuration
type LLMConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

// MCPConfig is the HTTP MCP / SSE configuration
type MCPConfig struct {
	SSEKeepaliveSec int
	SSERetryMS      int
	SSEReplayBuffer int
}

// FileStorageConfig is the file storage configuration
type FileStorageConfig struct {
	Type        string // "local" | "s3"
	LocalDir    string // default "./uploads"
	S3Endpoint  string
	S3Bucket    string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3Secure    bool // default true (use TLS for S3 connections)
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

// Load loads configuration from environment
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
		FileStorage: FileStorageConfig{
			Type:        getEnv("FILE_STORAGE_TYPE", "local"),
			LocalDir:    getEnv("FILE_STORAGE_LOCAL_DIR", "./uploads"),
			S3Endpoint:  getEnv("FILE_STORAGE_S3_ENDPOINT", ""),
			S3Bucket:    getEnv("FILE_STORAGE_S3_BUCKET", ""),
			S3Region:    getEnv("FILE_STORAGE_S3_REGION", ""),
			S3AccessKey: getEnv("FILE_STORAGE_S3_ACCESS_KEY", ""),
			S3SecretKey: getEnv("FILE_STORAGE_S3_SECRET_KEY", ""),
			S3Secure:    getEnvAsBool("FILE_STORAGE_S3_SECURE", true),
		},
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	switch c.Database.Type {
	case "postgres":
		if strings.TrimSpace(c.Database.URL) == "" {
			return fmt.Errorf("DATABASE_URL is required")
		}
	case "mysql":
		if strings.TrimSpace(c.Database.URL) == "" {
			return fmt.Errorf("DATABASE_URL is required")
		}
	case "sqlite":
		url := strings.TrimSpace(c.Database.URL)
		if url == "" {
			c.Database.URL = "./cornerstone.db"
		} else if url == ":memory:" {
			// OK
		} else if strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
			return fmt.Errorf("DATABASE_URL looks like a PostgreSQL connection string, but DB_TYPE is set to sqlite")
		} else if strings.HasPrefix(url, "mysql://") || strings.HasPrefix(url, "tcp(") {
			return fmt.Errorf("DATABASE_URL looks like a MySQL connection string, but DB_TYPE is set to sqlite")
		} else if !filepath.IsAbs(url) && url != ":memory:" {
			if absPath, err := os.Getwd(); err == nil {
				c.Database.URL = filepath.Join(absPath, url)
			}
		}
	default:
		return fmt.Errorf("unsupported database type: %s, supported: sqlite, postgres, mysql", c.Database.Type)
	}

	if strings.TrimSpace(c.Server.Port) == "" {
		return fmt.Errorf("PORT is required")
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

	switch c.FileStorage.Type {
	case "s3":
		if strings.TrimSpace(c.FileStorage.S3Endpoint) == "" {
			return fmt.Errorf("FILE_STORAGE_S3_ENDPOINT is required when FILE_STORAGE_TYPE=s3")
		}
		if strings.TrimSpace(c.FileStorage.S3Bucket) == "" {
			return fmt.Errorf("FILE_STORAGE_S3_BUCKET is required when FILE_STORAGE_TYPE=s3")
		}
		if strings.TrimSpace(c.FileStorage.S3AccessKey) == "" {
			return fmt.Errorf("FILE_STORAGE_S3_ACCESS_KEY is required when FILE_STORAGE_TYPE=s3")
		}
		if strings.TrimSpace(c.FileStorage.S3SecretKey) == "" {
			return fmt.Errorf("FILE_STORAGE_S3_SECRET_KEY is required when FILE_STORAGE_TYPE=s3")
		}
	case "local", "":
		if strings.TrimSpace(c.FileStorage.LocalDir) == "" {
			c.FileStorage.LocalDir = "./uploads"
		}
	default:
		return fmt.Errorf("unsupported FILE_STORAGE_TYPE: %q, supported: local, s3", c.FileStorage.Type)
	}

	return nil
}

func (c *Config) IsSQLite() bool {
	return c.Database.Type == "sqlite"
}

func (c *Config) IsPostgres() bool {
	return c.Database.Type == "postgres"
}

func (c *Config) IsMySQL() bool {
	return c.Database.Type == "mysql"
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

func getEnvAsBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	switch strings.ToLower(value) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	return defaultValue
}
