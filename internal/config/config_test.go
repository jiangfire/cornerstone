package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	orig := os.Getenv(key)
	os.Setenv(key, value)
	t.Cleanup(func() { os.Setenv(key, orig) })
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig := os.Getenv(key)
	os.Unsetenv(key)
	t.Cleanup(func() { os.Setenv(key, orig) })
}

func TestLoad_Defaults(t *testing.T) {
	unsetEnv(t, "DB_TYPE")
	unsetEnv(t, "DATABASE_URL")
	unsetEnv(t, "DB_MAX_OPEN")
	unsetEnv(t, "DB_MAX_IDLE")
	unsetEnv(t, "DB_MAX_LIFETIME")
	unsetEnv(t, "SERVER_MODE")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "LLM_API_KEY")
	unsetEnv(t, "LLM_MODEL")
	unsetEnv(t, "LLM_BASE_URL")
	unsetEnv(t, "MCP_SSE_KEEPALIVE_SEC")
	unsetEnv(t, "MCP_SSE_RETRY_MS")
	unsetEnv(t, "MCP_SSE_REPLAY_BUFFER")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "sqlite", cfg.Database.Type)
	assert.Equal(t, "./cornerstone.db", cfg.Database.URL)
	assert.Equal(t, 10, cfg.Database.MaxOpen)
	assert.Equal(t, 5, cfg.Database.MaxIdle)
	assert.Equal(t, 3600, cfg.Database.MaxLifetime)
	assert.Equal(t, "release", cfg.Server.Mode)
	assert.Equal(t, "8080", cfg.Server.Port)
}

func TestValidate_SqliteEmptyURL(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "sqlite", URL: ""},
		Server:   ServerConfig{Port: "8080"},
	}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "./cornerstone.db", cfg.Database.URL)
}

func TestValidate_SqliteMemoryURL(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "sqlite", URL: ":memory:"},
		Server:   ServerConfig{Port: "8080"},
	}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, ":memory:", cfg.Database.URL)
}

func TestValidate_SqliteWithPostgresURL(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "sqlite", URL: "postgres://user:pass@localhost/db"},
		Server:   ServerConfig{Port: "8080"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PostgreSQL")
}

func TestValidate_PostgresEmptyURL(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "postgres", URL: ""},
		Server:   ServerConfig{Port: "8080"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestValidate_UnsupportedDBType(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "mysql", URL: "some-url"},
		Server:   ServerConfig{Port: "8080"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的数据库类型")
}

func TestValidate_EmptyPort(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "sqlite", URL: ":memory:"},
		Server:   ServerConfig{Port: ""},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PORT")
}

func TestIsSQLite(t *testing.T) {
	cfg := &Config{Database: DatabaseConfig{Type: "sqlite"}}
	assert.True(t, cfg.IsSQLite())
	assert.False(t, cfg.IsPostgres())
}

func TestIsPostgres(t *testing.T) {
	cfg := &Config{Database: DatabaseConfig{Type: "postgres"}}
	assert.True(t, cfg.IsPostgres())
	assert.False(t, cfg.IsSQLite())
}

func TestGetServerAddr(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Port: "8080"}}
	assert.Equal(t, ":8080", cfg.GetServerAddr())
}

func TestMCPDefaults(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "sqlite", URL: ":memory:"},
		Server:   ServerConfig{Port: "8080"},
		MCP:      MCPConfig{SSEKeepaliveSec: 25, SSERetryMS: 3000, SSEReplayBuffer: 128},
	}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, 25, cfg.MCP.SSEKeepaliveSec)
	assert.Equal(t, 3000, cfg.MCP.SSERetryMS)
	assert.Equal(t, 128, cfg.MCP.SSEReplayBuffer)
}

func TestMCPZeroValuesGetDefaults(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "sqlite", URL: ":memory:"},
		Server:   ServerConfig{Port: "8080"},
		MCP:      MCPConfig{},
	}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, 25, cfg.MCP.SSEKeepaliveSec)
	assert.Equal(t, 3000, cfg.MCP.SSERetryMS)
	assert.Equal(t, 128, cfg.MCP.SSEReplayBuffer)
}

func TestGetEnv_Set(t *testing.T) {
	setEnv(t, "TEST_GETENV_SET", "hello")
	val := getEnv("TEST_GETENV_SET", "default")
	assert.Equal(t, "hello", val)
}

func TestGetEnv_Unset(t *testing.T) {
	unsetEnv(t, "TEST_GETENV_UNSET")
	val := getEnv("TEST_GETENV_UNSET", "default")
	assert.Equal(t, "default", val)
}

func TestGetEnvAsInt_Valid(t *testing.T) {
	setEnv(t, "TEST_INT_VALID", "42")
	val := getEnvAsInt("TEST_INT_VALID", 0)
	assert.Equal(t, 42, val)
}

func TestGetEnvAsInt_Invalid(t *testing.T) {
	setEnv(t, "TEST_INT_INVALID", "notanumber")
	val := getEnvAsInt("TEST_INT_INVALID", 99)
	assert.Equal(t, 99, val)
}

func TestGetEnvAsInt_Unset(t *testing.T) {
	unsetEnv(t, "TEST_INT_UNSET")
	val := getEnvAsInt("TEST_INT_UNSET", 7)
	assert.Equal(t, 7, val)
}

func TestLoggerDefaults(t *testing.T) {
	unsetEnv(t, "LOG_LEVEL")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "info", cfg.Logger.Level)
}

func TestLLMDefaults(t *testing.T) {
	unsetEnv(t, "LLM_MODEL")
	unsetEnv(t, "LLM_API_KEY")
	unsetEnv(t, "LLM_BASE_URL")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", cfg.LLM.Model)
	assert.Equal(t, "", cfg.LLM.APIKey)
	assert.Equal(t, "", cfg.LLM.BaseURL)
}
