package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/jiangfire/cornerstone/backend/pkg/utils"
	"gorm.io/gorm"
)

// AuthService 认证服务
type AuthService struct {
	db *gorm.DB
}

// NewAuthService 创建认证服务实例
func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=50"`
}

// LoginRequest 用户登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// validateUsername 验证用户名格式
func validateUsername(username string) error {
	// 去除首尾空格
	username = strings.TrimSpace(username)

	if len(username) < 3 || len(username) > 50 {
		return errors.New("用户名长度必须在3-50个字符之间")
	}

	// 支持字母（包括中文）、数字、下划线和连字符
	// \p{L} 匹配所有语言的字母（包括中文）
	// \p{N} 匹配所有语言的数字
	matched, _ := regexp.MatchString(`^[\p{L}\p{N}_-]+$`, username)
	if !matched {
		return errors.New("用户名只能包含字母、数字、下划线和连字符")
	}

	// 不能以数字开头
	if matched, _ := regexp.MatchString(`^[\p{N}]`, username); matched {
		return errors.New("用户名不能以数字开头")
	}

	return nil
}

// validateEmail 验证邮箱格式
func validateEmail(email string) error {
	email = strings.TrimSpace(email)

	if len(email) > 255 {
		return errors.New("邮箱地址过长")
	}

	// 使用更严格的邮箱验证正则
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
	if !matched {
		return errors.New("邮箱格式不正确")
	}

	return nil
}

// validatePassword 验证密码强度
func validatePassword(password string) error {
	if len(password) < 6 || len(password) > 50 {
		return errors.New("密码长度必须在6-50个字符之间")
	}

	// 检查是否包含至少一个字母和一个数字
	hasLetter, _ := regexp.MatchString(`[a-zA-Z]`, password)
	hasDigit, _ := regexp.MatchString(`[0-9]`, password)

	if !hasLetter || !hasDigit {
		return errors.New("密码必须包含至少一个字母和一个数字")
	}

	return nil
}

// sanitizeInput 清理输入，防止注入攻击
func sanitizeInput(input string) string {
	// 去除首尾空格
	input = strings.TrimSpace(input)
	// 替换可能有害的字符 - 使用简单的移除或替换
	input = strings.ReplaceAll(input, "<", "")
	input = strings.ReplaceAll(input, ">", "")
	input = strings.ReplaceAll(input, "\"", "")
	input = strings.ReplaceAll(input, "'", "")
	return input
}

// Register 用户注册
func (s *AuthService) Register(req RegisterRequest) (*AuthResponse, error) {
	// 1. 输入验证和清理
	req.Username = sanitizeInput(req.Username)
	req.Email = sanitizeInput(req.Email)

	if err := validateUsername(req.Username); err != nil {
		return nil, fmt.Errorf("用户名验证失败: %w", err)
	}

	if err := validateEmail(req.Email); err != nil {
		return nil, fmt.Errorf("邮箱验证失败: %w", err)
	}

	if err := validatePassword(req.Password); err != nil {
		return nil, fmt.Errorf("密码验证失败: %w", err)
	}

	// 2. 检查用户名是否已存在
	var existingUser models.User
	err := s.db.Where("username = ?", req.Username).First(&existingUser).Error
	if err == nil {
		return nil, errors.New("用户名已存在")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 3. 检查邮箱是否已存在
	err = s.db.Where("email = ?", req.Email).First(&existingUser).Error
	if err == nil {
		return nil, errors.New("邮箱已被注册")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 4. 密码哈希
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 5. 创建用户
	user := models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	// 6. 生成JWT Token
	token, err := utils.GenerateToken(user.ID, user.Username, "user")
	if err != nil {
		return nil, fmt.Errorf("生成Token失败: %w", err)
	}

	// 7. 清除密码字段（不返回给前端）
	user.Password = ""

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// Login 用户登录
func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
	// 1. 输入清理和验证
	req.Username = sanitizeInput(req.Username)

	if err := validateUsername(req.Username); err != nil && req.Username != "" {
		// 如果用户名验证失败，尝试作为邮箱验证
		if err := validateEmail(req.Username); err != nil {
			return nil, fmt.Errorf("用户名或邮箱格式错误: %w", err)
		}
	}

	if req.Password == "" {
		return nil, errors.New("密码不能为空")
	}

	// 2. 查询用户（支持用户名或邮箱登录）
	var user models.User
	err := s.db.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 使用通用错误信息，避免信息泄露
			return nil, errors.New("用户名或密码错误")
		}
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 3. 验证密码
	if err := utils.CheckPassword(req.Password, user.Password); err != nil {
		// 使用通用错误信息，避免信息泄露
		return nil, errors.New("用户名或密码错误")
	}

	// 4. 生成JWT Token
	token, err := utils.GenerateToken(user.ID, user.Username, "user")
	if err != nil {
		return nil, fmt.Errorf("生成Token失败: %w", err)
	}

	// 5. 清除密码字段
	user.Password = ""

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// GetUserByID 根据ID获取用户信息
func (s *AuthService) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	err := s.db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 清除密码字段
	user.Password = ""

	return &user, nil
}

// Logout 用户登出（将Token加入黑名单）
func (s *AuthService) Logout(token string) error {
	// 1. 解析Token获取过期时间
	claims, err := utils.ParseToken(token)
	if err != nil {
		// Token无效，无需加入黑名单
		return nil
	}

	// 2. 将Token加入黑名单
	tokenHash := utils.HashToken(token)
	blacklist := models.TokenBlacklist{
		TokenHash: tokenHash,
		ExpiredAt: claims.ExpiresAt.Time,
	}

	if err := s.db.Create(&blacklist).Error; err != nil {
		return fmt.Errorf("添加黑名单失败: %w", err)
	}

	return nil
}
