package services

import (
	"os"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建测试数据库
func setupAuthTestDB(t *testing.T) *gorm.DB {
	// 设置测试环境变量
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&models.User{}, &models.TokenBlacklist{})
	assert.NoError(t, err)

	return db
}

// TestValidateUsername 测试用户名验证
func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"Valid username", "john_doe", false},
		{"Valid with Chinese", "张三_test", false},
		{"Valid with hyphen", "user-name", false},
		{"Too short", "ab", true},
		{"Too long", "a123456789012345678901234567890123456789012345678901", true},
		{"Starts with number", "123user", true},
		{"Contains special chars", "user@name", true},
		{"Empty string", "", true},
		{"Only spaces", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsername(tt.username)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateEmail 测试邮箱验证
func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"Valid email", "test@example.com", false},
		{"Valid with subdomain", "user@mail.example.com", false},
		{"Valid with plus", "user+tag@example.com", false},
		{"Invalid no @", "testexample.com", true},
		{"Invalid no domain", "test@", true},
		{"Invalid no TLD", "test@example", true},
		{"Too long", "a" + string(make([]byte, 250)) + "@example.com", true},
		{"Empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.email)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidatePassword 测试密码验证
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"Valid password", "Pass123", false},
		{"Valid complex", "MyP@ssw0rd", false},
		{"Too short", "Pa1", true},
		{"No letter", "123456", true},
		{"No digit", "Password", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestAuthService_Register 测试用户注册
func TestAuthService_Register(t *testing.T) {
	db := setupAuthTestDB(t)
	service := NewAuthService(db)

	t.Run("Successful registration", func(t *testing.T) {
		req := RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "Pass123",
		}

		resp, err := service.Register(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token)
		assert.Equal(t, "testuser", resp.User.Username)
	})

	t.Run("Duplicate username", func(t *testing.T) {
		req := RegisterRequest{
			Username: "testuser",
			Email:    "another@example.com",
			Password: "Pass123",
		}

		_, err := service.Register(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "用户名已存在")
	})
}

// TestAuthService_Login 测试用户登录
func TestAuthService_Login(t *testing.T) {
	db := setupAuthTestDB(t)
	service := NewAuthService(db)

	// 先注册一个用户
	regReq := RegisterRequest{
		Username: "loginuser",
		Email:    "login@example.com",
		Password: "Pass123",
	}
	_, err := service.Register(regReq)
	assert.NoError(t, err)

	t.Run("Successful login", func(t *testing.T) {
		loginReq := LoginRequest{
			Username: "loginuser",
			Password: "Pass123",
		}

		resp, err := service.Login(loginReq)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token)
	})

	t.Run("Wrong password", func(t *testing.T) {
		loginReq := LoginRequest{
			Username: "loginuser",
			Password: "WrongPass123",
		}

		_, err := service.Login(loginReq)
		assert.Error(t, err)
	})
}
