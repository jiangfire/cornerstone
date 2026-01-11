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
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
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
	Level      string
	OutputPath string
	ErrorPath  string
	MaxSize    int
	MaxAge     int
	MaxBackups int
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret     string
	Expiration int // 单位：小时
}

// Load 加载配置从环境变量
func Load() (*Config, error) {
	// 尝试加载 .env 文件（开发环境）
	_ = godotenv.Load()

	// 读取环境变量
	config := &Config{
		Database: DatabaseConfig{
			URL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cornerstone?sslmode=disable"),
			MaxOpen:     getEnvAsInt("DB_MAX_OPEN", 10),
			MaxIdle:     getEnvAsInt("DB_MAX_IDLE", 5),
			MaxLifetime: getEnvAsInt("DB_MAX_LIFETIME", 3600),
		},
		Server: ServerConfig{
			Mode: getEnv("SERVER_MODE", "debug"),
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
			Secret:     getEnv("JWT_SECRET", "your-secret-key-here"),
			Expiration: getEnvAsInt("JWT_EXPIRATION", 24),
		},
	}

	// 验证关键配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
}

// Validate 验证配置是否完整
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Database.URL) == "" {
		return fmt.Errorf("DATABASE_URL 不能为空")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" || c.JWT.Secret == "your-secret-key-here" {
		return fmt.Errorf("JWT_SECRET 必须设置且不能使用默认值")
	}
	if strings.TrimSpace(c.Server.Port) == "" {
		return fmt.Errorf("PORT 不能为空")
	}
	return nil
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
