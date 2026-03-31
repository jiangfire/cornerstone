package config

import "testing"

import "github.com/stretchr/testify/require"

func TestConfigValidateAppliesSafeDefaults(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Type: "sqlite",
			URL:  "",
		},
		Server: ServerConfig{
			Port: "8080",
		},
		JWT: JWTConfig{
			Secret: "change-this-secret-key",
		},
		Integrations: IntegrationsConfig{},
		MCP:          MCPConfig{},
	}

	err := cfg.Validate()
	require.NoError(t, err)
	// SQLite URL 现在使用绝对路径和 file:// 前缀
	require.Contains(t, cfg.Database.URL, "cornerstone.db")
	require.True(t, cfg.Database.URL == "cornerstone.db" || cfg.Database.URL[:7] == "file://")
	require.Equal(t, "cornerstone-default-secret-key-change-in-production", cfg.JWT.Secret)
	require.Equal(t, 5, cfg.Integrations.OutboundTimeoutSec)
	require.Equal(t, 5, cfg.Integrations.OutboxMaxRetries)
	require.Equal(t, 60, cfg.Integrations.OutboxRetryInterval)
	require.Equal(t, 25, cfg.MCP.SSEKeepaliveSec)
	require.Equal(t, 3000, cfg.MCP.SSERetryMS)
	require.Equal(t, 128, cfg.MCP.SSEReplayBuffer)
}

func TestConfigValidateRejectsUnsupportedDatabaseType(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Type: "mysql",
			URL:  "dsn",
		},
		Server: ServerConfig{
			Port: "8080",
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "不支持的数据库类型")
}

func TestConfigValidateRejectsEmptyPort(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Type: "sqlite",
			URL:  "cornerstone.db",
		},
		Server: ServerConfig{},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "PORT 不能为空")
}
