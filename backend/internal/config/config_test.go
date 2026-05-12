package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidateAppliesSafeDefaults(t *testing.T) {
	strongSecret := "this-is-a-strong-test-secret-with-enough-entropy-for-tests"
	cfg := &Config{
		Database: DatabaseConfig{
			Type: "sqlite",
			URL:  "cornerstone.db",
		},
		Server: ServerConfig{
			Port: "8080",
		},
		JWT: JWTConfig{
			Secret: strongSecret,
		},
		Integrations: IntegrationsConfig{},
		MCP:          MCPConfig{},
	}

	err := cfg.Validate()
	require.NoError(t, err)
	// 相对路径应被解析为绝对路径，并保留文件名
	require.Contains(t, cfg.Database.URL, "cornerstone.db")
	// 强随机 JWT 密钥应被原样保留
	require.Equal(t, strongSecret, cfg.JWT.Secret)
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

func TestConfigValidateRejectsWeakJWTSecretInRelease(t *testing.T) {
	cases := []string{
		"",
		"change-this-secret-key",
		"your-secret-key-here",
		"dev-secret-key-change-in-production",
		"cornerstone-default-secret-key-change-in-production",
		"too-short", // 长度不足 32 字节
	}
	for _, secret := range cases {
		t.Run(secret, func(t *testing.T) {
			cfg := &Config{
				Database: DatabaseConfig{Type: "sqlite", URL: "cornerstone.db"},
				Server:   ServerConfig{Mode: "release", Port: "8080"},
				JWT:      JWTConfig{Secret: secret},
			}
			err := cfg.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "JWT_SECRET")
		})
	}
}

func TestConfigValidateGeneratesRandomJWTSecretInDebug(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Type: "sqlite", URL: "cornerstone.db"},
		Server:   ServerConfig{Mode: "debug", Port: "8080"},
		JWT:      JWTConfig{Secret: "change-this-secret-key"},
	}
	err := cfg.Validate()
	require.NoError(t, err)
	require.NotEqual(t, "change-this-secret-key", cfg.JWT.Secret)
	require.NotEqual(t, "cornerstone-default-secret-key-change-in-production", cfg.JWT.Secret)
	require.GreaterOrEqual(t, len(cfg.JWT.Secret), 32)
}

func TestIsWeakJWTSecret(t *testing.T) {
	require.True(t, isWeakJWTSecret(""))
	require.True(t, isWeakJWTSecret("   "))
	require.True(t, isWeakJWTSecret("change-this-secret-key"))
	require.True(t, isWeakJWTSecret(strings.Repeat("a", 31)))
	require.False(t, isWeakJWTSecret(strings.Repeat("a", 32)))
}

func TestGenerateSecureRandomString(t *testing.T) {
	a, err := generateSecureRandomString(32)
	require.NoError(t, err)
	b, err := generateSecureRandomString(32)
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.GreaterOrEqual(t, len(a), 32)
}
