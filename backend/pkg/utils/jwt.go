package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"golang.org/x/crypto/bcrypt"
)

// JWTClaims JWT声明结构
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
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
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(cfg.JWT.Expiration))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "cornerstone",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

// ValidateJWT 验证JWT令牌
func ValidateJWT(tokenString string) (*JWTClaims, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(cfg.JWT.Secret), nil
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
	return db.DB().Create(&tokenBlacklist).Error
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
		return errors.New("密码错误")
	}
	return nil
}
