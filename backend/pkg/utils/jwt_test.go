package utils

import (
	"strings"
	"testing"
	"time"
)

func TestInitJWT_RejectsEmptySecret(t *testing.T) {
	ResetJWTForTests()
	defer ResetJWTForTests()

	if err := InitJWT("", 24); err == nil {
		t.Fatal("InitJWT 应拒绝空 secret")
	}
	if err := InitJWT("   ", 24); err == nil {
		t.Fatal("InitJWT 应拒绝空白 secret")
	}
}

func TestInitJWT_NormalizesNonPositiveExpiration(t *testing.T) {
	ResetJWTForTests()
	defer ResetJWTForTests()

	if err := InitJWT("a-strong-test-secret-1234567890", 0); err != nil {
		t.Fatalf("InitJWT 失败: %v", err)
	}
	_, exp, err := loadJWTConfig()
	if err != nil {
		t.Fatalf("loadJWTConfig 失败: %v", err)
	}
	if exp != 24 {
		t.Fatalf("expirationHours <=0 应该归一化为 24，实际 %d", exp)
	}
}

// TestSignAndVerifyUseSameSecret 是 P1-2 的核心回归用例：
// 同一进程内 GenerateJWT 与 ValidateJWT 必须使用同一密钥，
// 历史上 loadJWTConfig 会调用 config.Load() 在 dev 模式下二次生成临时随机 secret，
// 导致登录后 token 立即失效。本测试断言此类缺陷不再出现。
func TestSignAndVerifyUseSameSecret(t *testing.T) {
	ResetJWTForTests()
	defer ResetJWTForTests()

	if err := InitJWT("p1-2-regression-secret-with-enough-length", 1); err != nil {
		t.Fatalf("InitJWT 失败: %v", err)
	}

	for i := range 5 {
		token, err := GenerateJWT("user-1", "alice", "user")
		if err != nil {
			t.Fatalf("GenerateJWT 第 %d 次失败: %v", i, err)
		}
		claims, err := ValidateJWT(token)
		if err != nil {
			t.Fatalf("ValidateJWT 第 %d 次失败: %v", i, err)
		}
		if claims.UserID != "user-1" || claims.Username != "alice" || claims.Role != "user" {
			t.Fatalf("第 %d 次解析得到的 claims 与签发不一致: %+v", i, claims)
		}
	}
}

func TestValidateJWT_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	ResetJWTForTests()
	defer ResetJWTForTests()

	if err := InitJWT("secret-alpha-with-enough-length-1234567890", 1); err != nil {
		t.Fatalf("InitJWT 失败: %v", err)
	}
	token, err := GenerateJWT("u", "u", "user")
	if err != nil {
		t.Fatalf("GenerateJWT 失败: %v", err)
	}

	ResetJWTForTests()
	if err := InitJWT("secret-beta-with-enough-length-1234567890", 1); err != nil {
		t.Fatalf("InitJWT 失败: %v", err)
	}
	if _, err := ValidateJWT(token); err == nil {
		t.Fatal("用 secret-beta 不应能验证 secret-alpha 签发的 token")
	}
}

func TestLoadJWTConfig_FallsBackToEnv(t *testing.T) {
	ResetJWTForTests()
	defer ResetJWTForTests()

	t.Setenv("JWT_SECRET", "env-fallback-secret-with-enough-length")
	t.Setenv("JWT_EXPIRATION", "48")

	secret, exp, err := loadJWTConfig()
	if err != nil {
		t.Fatalf("loadJWTConfig 失败: %v", err)
	}
	if !strings.HasPrefix(secret, "env-fallback") {
		t.Fatalf("期望读取到环境变量的 secret，实际 %q", secret)
	}
	if exp != 48 {
		t.Fatalf("期望读取到 JWT_EXPIRATION=48，实际 %d", exp)
	}
}

func TestLoadJWTConfig_ErrorsWhenUninitialized(t *testing.T) {
	ResetJWTForTests()
	defer ResetJWTForTests()
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_EXPIRATION", "")

	if _, _, err := loadJWTConfig(); err == nil {
		t.Fatal("loadJWTConfig 在未初始化且无环境变量时应返回错误")
	}
}

func TestGenerateJWT_ContainsStandardClaims(t *testing.T) {
	ResetJWTForTests()
	defer ResetJWTForTests()
	if err := InitJWT("standard-claims-secret-1234567890ABC", 2); err != nil {
		t.Fatalf("InitJWT 失败: %v", err)
	}

	token, err := GenerateJWT("u", "u", "user")
	if err != nil {
		t.Fatalf("GenerateJWT 失败: %v", err)
	}
	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT 失败: %v", err)
	}
	if claims.Issuer != "cornerstone" {
		t.Fatalf("Issuer 期望 cornerstone, 实际 %q", claims.Issuer)
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now()) {
		t.Fatal("ExpiresAt 应在未来")
	}
}
