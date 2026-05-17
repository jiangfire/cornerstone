package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

// JWTClaims JWT声明结构
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

var (
	jwtConfigMu   sync.RWMutex
	jwtCfgLoaded  bool
	jwtSecret     string
	jwtExpiration int
)

// InitJWT 显式注入 JWT 配置；推荐由 main.go 在启动早期调用一次。
// 重复调用会覆盖现有配置（便于测试 setup/teardown）。
func InitJWT(secret string, expirationHours int) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("JWT secret 不能为空")
	}
	if expirationHours <= 0 {
		expirationHours = 24
	}
	jwtConfigMu.Lock()
	defer jwtConfigMu.Unlock()
	jwtSecret = secret
	jwtExpiration = expirationHours
	jwtCfgLoaded = true
	return nil
}

// ResetJWTForTests 清空全局 JWT 配置，仅用于测试。
func ResetJWTForTests() {
	jwtConfigMu.Lock()
	defer jwtConfigMu.Unlock()
	jwtSecret = ""
	jwtExpiration = 0
	jwtCfgLoaded = false
}

// loadJWTConfig 返回 JWT 配置；若尚未通过 InitJWT 显式注入，则降级读取环境变量。
//
// 注意：严禁在此处调用 config.Load()——历史上这样做会在 dev 模式下生成与 main.go
// 不同的临时密钥，导致签发/验证不一致；详见 docs/REVIEW-FIX-PLAN-2026-05.md P1-2。
func loadJWTConfig() (string, int, error) {
	jwtConfigMu.RLock()
	if jwtCfgLoaded {
		secret := jwtSecret
		expiration := jwtExpiration
		jwtConfigMu.RUnlock()
		return secret, expiration, nil
	}
	jwtConfigMu.RUnlock()

	jwtConfigMu.Lock()
	defer jwtConfigMu.Unlock()
	if jwtCfgLoaded {
		return jwtSecret, jwtExpiration, nil
	}

	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "", 0, errors.New("JWT 未初始化：请先调用 utils.InitJWT 或设置 JWT_SECRET 环境变量")
	}
	expiration := 24
	if v := strings.TrimSpace(os.Getenv("JWT_EXPIRATION")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			expiration = n
		}
	}

	jwtSecret = secret
	jwtExpiration = expiration
	jwtCfgLoaded = true

	return jwtSecret, jwtExpiration, nil
}

// HashPassword 使用bcrypt哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash 验证密码哈希
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT 生成JWT令牌
func GenerateJWT(userID, username, role string) (string, error) {
	secret, expiration, err := loadJWTConfig()
	if err != nil {
		return "", err
	}

	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(expiration))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "cornerstone",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT 验证JWT令牌
func ValidateJWT(tokenString string) (*JWTClaims, error) {
	secret, _, err := loadJWTConfig()
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// HashToken 计算token的SHA256哈希
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// BlacklistToken 将token加入黑名单
func BlacklistToken(token string, expiration time.Time) error {
	hash := HashToken(token)
	tokenBlacklist := models.TokenBlacklist{
		TokenHash: hash,
		ExpiredAt: expiration,
	}
	return db.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_hash"}},
		DoNothing: true,
	}).Create(&tokenBlacklist).Error
}

// IsTokenBlacklisted 检查token是否在黑名单中
func IsTokenBlacklisted(token string) bool {
	hash := HashToken(token)
	var count int64
	db.DB().Model(&models.TokenBlacklist{}).Where("token_hash = ? AND expired_at > ?", hash, time.Now()).Count(&count)
	return count > 0
}

// Logout 注销token（加入黑名单）
func Logout(tokenString string) error {
	// 解析token获取过期时间
	claims, err := ValidateJWT(tokenString)
	if err != nil {
		return err
	}

	// 计算剩余过期时间
	expiration := time.Unix(claims.ExpiresAt.Unix(), 0)
	return BlacklistToken(tokenString, expiration)
}

// 别名函数，用于简化调用
func GenerateToken(userID, username, role string) (string, error) {
	return GenerateJWT(userID, username, role)
}

func ParseToken(tokenString string) (*JWTClaims, error) {
	return ValidateJWT(tokenString)
}

func CheckPassword(password, hash string) error {
	if !CheckPasswordHash(password, hash) {
		return fmt.Errorf("密码错误")
	}
	return nil
}
