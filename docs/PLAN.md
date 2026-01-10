# 硬件工程数据管理平台 - 开发实施计划

**版本**: v1.0
**日期**: 2026-01-06
**状态**: 🚀 待执行

---

## 📋 文档说明

本文档是项目的**开发执行手册**，提供Day-by-Day开发步骤。

**如需查看详细技术设计，请参考：**
- [IMPLEMENTATION-PLAN.md](./IMPLEMENTATION-PLAN.md) - 数据模型、API设计、实现细节
- [DATABASE.md](./DATABASE.md) - 完整数据库设计
- [API.md](./API.md) - 完整接口规范

**其他相关文档：**
- [PRD.md](./PRD.md) - 产品需求
- [ARCHITECTURE.md](./ARCHITECTURE.md) - 技术架构
- [GUIDE.md](./GUIDE.md) - 项目导航

---

## 📖 文档用途

| 文档 | 用途 | 使用场景 |
|------|------|----------|
| **IMPLEMENTATION-PLAN.md** | 技术设计评审 | 1. 方案设计阶段<br>2. 技术细节查阅<br>3. 代码实现参考 |
| **本文档** | 开发执行手册 | 1. 日常开发<br>2. 按步骤编码<br>3. 任务跟踪 |

---

## 🎯 项目现状

### ✅ 已完成
- 所有设计文档（PRD、数据库、架构、API）
- 基础设施搭建（Go/Vue脚手架）
- 技术栈选型确认

### ❌ 待开发
- 后端业务代码（0%）
- 前端业务代码（0%）
- 测试代码（0%）
- 部署配置（0%）

### 📊 整体进度：30%（基础设施完成）

---

## 🚀 Sprint 1: 用户认证 + 组织管理（3-4周）

### 目标
完成用户注册/登录、组织创建/管理、成员权限控制，为后续功能打下基础。

---

### Week 1: 后端基础架构（3-4天）

#### Day 1: 项目结构与配置

**任务清单：**
1. ✅ 创建应用入口 `backend/cmd/server/main.go`
2. ✅ 创建配置管理 `internal/config/config.go`
3. ✅ 创建数据库迁移工具
4. ✅ 创建统一响应格式

**详细步骤：**

**1. 创建 main.go**
```go
// backend/cmd/server/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jiangfire/cornerstone/backend/internal/config"
    "github.com/jiangfire/cornerstone/backend/internal/handlers"
    "github.com/jiangfire/cornerstone/backend/internal/middleware"
    "github.com/jiangfire/cornerstone/backend/pkg/db"
    "github.com/jiangfire/cornerstone/backend/pkg/log"
)

func main() {
    // 1. 加载配置（从环境变量）
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // 2. 初始化日志
    if err := log.InitLogger(cfg.Logger); err != nil {
        log.Fatalf("Failed to init logger: %v", err)
    }
    defer log.Sync()

    // 3. 初始化数据库
    dsn := cfg.Database.DSN()
    if err := db.InitDB(dsn, log.Logger()); err != nil {
        log.Fatalf("Failed to init database: %v", err)
    }

    // 4. 自动迁移
    if err := db.AutoMigrate(); err != nil {
        log.Fatalf("Failed to migrate: %v", err)
    }

    // 5. 创建Gin引擎
    r := gin.New()
    r.Use(gin.Recovery())
    r.Use(middleware.CORS())

    // 6. 注册路由
    handlers.RegisterRoutes(r)

    // 7. 启动服务
    srv := &http.Server{
        Addr:    ":" + cfg.Server.Port,
        Handler: r,
    }

    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    // 8. 优雅关闭
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("Server shutdown error: %v", err)
    }

    log.Info("Server exited")
}
```

**2. 创建配置管理**
```go
// internal/config/config.go
package config

import (
    "github.com/spf13/viper"
    "github.com/jiangfire/cornerstone/backend/pkg/db"
    "github.com/jiangfire/cornerstone/backend/pkg/log"
)

type Config struct {
    Server   ServerConfig   `mapstructure:"server"`
    Database DatabaseConfig `mapstructure:"database"`
    JWT      JWTConfig      `mapstructure:"jwt"`
    Logger   log.LoggerConfig `mapstructure:"logger"`
}

type ServerConfig struct {
    Port string `mapstructure:"port"`
    Mode string `mapstructure:"mode"` // debug | release
}

type DatabaseConfig struct {
    Host     string `mapstructure:"host"`
    Port     string `mapstructure:"port"`
    User     string `mapstructure:"user"`
    Password string `mapstructure:"password"`
    DBName   string `mapstructure:"dbname"`
    Timezone string `mapstructure:"timezone"`
}

func (c DatabaseConfig) DSN() string {
    return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=%s",
        c.Host, c.Port, c.User, c.Password, c.DBName, c.Timezone)
}

type JWTConfig struct {
    Secret        string `mapstructure:"secret"`
    AccessExpire  int64  `mapstructure:"access_expire"`
    RefreshExpire int64  `mapstructure:"refresh_expire"`
}

func Load(path string) (*Config, error) {
    viper.SetConfigFile(path)
    viper.SetConfigType("yaml")

    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

**3. 创建统一响应格式**
```go
// internal/types/response.go
package types

type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
    return Response{
        Code:    0,
        Message: "success",
        Data:    data,
    }
}

func Error(code int, message string) Response {
    return Response{
        Code:    code,
        Message: message,
    }
}

// 错误码定义
const (
    ErrCodeSuccess           = 0
    ErrCodeInvalidParams     = 1000
    ErrCodeUserExists        = 1001
    ErrCodeUserNotFound      = 1002
    ErrCodeInvalidPassword   = 1003
    ErrCodeTokenInvalid      = 1004
    ErrCodeTokenExpired      = 1005
    ErrCodeOrgNotFound       = 1101
    ErrCodePermissionDenied  = 1102
    ErrCodeMemberExists      = 1103
)

var ErrorMessages = map[int]string{
    ErrCodeSuccess:          "success",
    ErrCodeInvalidParams:    "参数错误",
    ErrCodeUserExists:       "用户已存在",
    ErrCodeUserNotFound:     "用户不存在",
    ErrCodeInvalidPassword:  "密码错误",
    ErrCodeTokenInvalid:     "token无效",
    ErrCodeTokenExpired:     "token已过期",
    ErrCodeOrgNotFound:      "组织不存在",
    ErrCodePermissionDenied: "权限不足",
    ErrCodeMemberExists:     "成员已存在",
}
```

**4. 更新数据库迁移**
```go
// internal/db/migrate.go
package db

import (
    "github.com/jiangfire/cornerstone/backend/internal/models"
    "gorm.io/gorm"
)

func AutoMigrate() error {
    return db.AutoMigrate(
        &models.User{},
        &models.Organization{},
        &models.OrganizationMember{},
    )
}
```

**交付物：**
- ✅ `backend/cmd/server/main.go`
- ✅ `internal/config/config.go`
- ✅ `internal/types/response.go`
- ✅ `internal/db/migrate.go`
- ✅ `.env.example` (环境变量示例文件)

---

#### Day 2: 数据模型 + JWT工具

**任务清单：**
1. ✅ 创建数据模型（User, Organization, OrganizationMember）
2. ✅ 创建JWT工具类
3. ✅ 创建密码哈希工具

**详细步骤：**

**1. 数据模型**
```go
// internal/models/user.go
package models

import "time"

type User struct {
    ID           string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
    Username     string    `gorm:"uniqueIndex;not null;size:50" json:"username"`
    UserCode     string    `gorm:"uniqueIndex;not null;size:20" json:"user_code"`
    PasswordHash string    `gorm:"not null" json:"-"`
    Email        string    `gorm:"index;size:100" json:"email"`
    Avatar       string    `gorm:"size:255" json:"avatar"`
    IsActive     bool      `gorm:"default:true" json:"is_active"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string {
    return "users"
}
```

```go
// internal/models/organization.go
package models

import "time"

type Organization struct {
    ID          string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
    Name        string    `gorm:"not null;size:100" json:"name"`
    Description string    `gorm:"size:500" json:"description"`
    OwnerID     string    `gorm:"not null;type:uuid" json:"owner_id"`
    Avatar      string    `gorm:"size:255" json:"avatar"`
    IsActive    bool      `gorm:"default:true" json:"is_active"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`

    Owner User `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

func (Organization) TableName() string {
    return "organizations"
}
```

```go
// internal/models/member.go
package models

import "time"

type OrganizationMember struct {
    OrganizationID string    `gorm:"primaryKey;type:uuid" json:"organization_id"`
    UserID         string    `gorm:"primaryKey;type:uuid" json:"user_id"`
    Role           string    `gorm:"type:varchar(20);not null" json:"role"`
    JoinedAt       time.Time `gorm:"default:now()" json:"joined_at"`

    User         User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
    Organization Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (OrganizationMember) TableName() string {
    return "organization_members"
}
```

**2. JWT工具类**
```go
// internal/pkg/jwt/jwt.go
package jwt

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    UserID   string `json:"user_id"`
    UserCode string `json:"user_code"`
    Username string `json:"username"`
    jwt.RegisteredClaims
}

type JWTUtil struct {
    secret        string
    accessExpire  time.Duration
    refreshExpire time.Duration
}

func New(secret string, accessExpire, refreshExpire int64) *JWTUtil {
    return &JWTUtil{
        secret:        secret,
        accessExpire:  time.Duration(accessExpire) * time.Second,
        refreshExpire: time.Duration(refreshExpire) * time.Second,
    }
}

// 生成Token
func (j *JWTUtil) GenerateTokens(userID, userCode, username string) (accessToken, refreshToken string, err error) {
    // Access Token
    accessClaims := &Claims{
        UserID:   userID,
        UserCode: userCode,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.accessExpire)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Subject:   userID,
        },
    }
    accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(j.secret))
    if err != nil {
        return "", "", err
    }

    // Refresh Token
    refreshClaims := &Claims{
        UserID:   userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.refreshExpire)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Subject:   userID,
        },
    }
    refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(j.secret))
    if err != nil {
        return "", "", err
    }

    return accessToken, refreshToken, nil
}

// 解析Token
func (j *JWTUtil) ParseToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(j.secret), nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, jwt.ErrTokenInvalid
}
```

**3. 密码工具**
```go
// internal/pkg/utils/password.go
package utils

import "golang.org/x/crypto/bcrypt"

const BcryptCost = 12

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
    return string(bytes), err
}

func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

**交付物：**
- ✅ `internal/models/user.go`
- ✅ `internal/models/organization.go`
- ✅ `internal/models/member.go`
- ✅ `internal/pkg/jwt/jwt.go`
- ✅ `internal/pkg/utils/password.go`

---

#### Day 3-4: Repository层 + Service层

**任务清单：**
1. ✅ UserRepository
2. ✅ OrganizationRepository
3. ✅ MemberRepository
4. ✅ AuthService
5. ✅ OrganizationService

**详细步骤：**

**1. Repository接口定义**
```go
// internal/repository/user.go
package repository

import (
    "github.com/jiangfire/cornerstone/backend/internal/models"
    "gorm.io/gorm"
)

type UserRepository interface {
    Create(user *models.User) (*models.User, error)
    GetByID(id string) (*models.User, error)
    GetByUsername(username string) (*models.User, error)
    GetByUserCode(userCode string) (*models.User, error)
    ExistsByUsername(username string) (bool, error)
    ExistsByUserCode(userCode string) (bool, error)
    Update(user *models.User) error
    UpdatePassword(id string, hash string) error
}

type userRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
    return &userRepository{db: db}
}

func (r *userRepository) Create(user *models.User) (*models.User, error) {
    err := r.db.Create(user).Error
    return user, err
}

func (r *userRepository) GetByID(id string) (*models.User, error) {
    var user models.User
    err := r.db.First(&user, "id = ?", id).Error
    return &user, err
}

func (r *userRepository) GetByUsername(username string) (*models.User, error) {
    var user models.User
    err := r.db.First(&user, "username = ?", username).Error
    return &user, err
}

func (r *userRepository) GetByUserCode(userCode string) (*models.User, error) {
    var user models.User
    err := r.db.First(&user, "user_code = ?", userCode).Error
    return &user, err
}

func (r *userRepository) ExistsByUsername(username string) (bool, error) {
    var count int64
    err := r.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
    return count > 0, err
}

func (r *userRepository) ExistsByUserCode(userCode string) (bool, error) {
    var count int64
    err := r.db.Model(&models.User{}).Where("user_code = ?", userCode).Count(&count).Error
    return count > 0, err
}

func (r *userRepository) Update(user *models.User) error {
    return r.db.Save(user).Error
}

func (r *userRepository) UpdatePassword(id string, hash string) error {
    return r.db.Model(&models.User{}).Where("id = ?", id).Update("password_hash", hash).Error
}
```

**2. Service层**
```go
// internal/service/auth.go
package service

import (
    "errors"
    "github.com/jiangfire/cornerstone/backend/internal/models"
    "github.com/jiangfire/cornerstone/backend/internal/pkg/jwt"
    "github.com/jiangfire/cornerstone/backend/internal/pkg/utils"
    "github.com/jiangfire/cornerstone/backend/internal/repository"
)

type RegisterInput struct {
    Username string `json:"username" binding:"required,min=3,max=50"`
    UserCode string `json:"user_code" binding:"required,min=4,max=20"`
    Password string `json:"password" binding:"required,min=8"`
    Email    string `json:"email"`
}

type LoginInput struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

type LoginResult struct {
    AccessToken  string       `json:"access_token"`
    RefreshToken string       `json:"refresh_token"`
    User         *models.User `json:"user"`
}

type AuthService struct {
    userRepo repository.UserRepository
    jwtUtil  *jwt.JWTUtil
}

func NewAuthService(userRepo repository.UserRepository, jwtUtil *jwt.JWTUtil) *AuthService {
    return &AuthService{userRepo: userRepo, jwtUtil: jwtUtil}
}

func (s *AuthService) Register(input RegisterInput) (*models.User, error) {
    // 检查用户名
    exists, err := s.userRepo.ExistsByUsername(input.Username)
    if err != nil {
        return nil, err
    }
    if exists {
        return nil, errors.New("用户已存在")
    }

    // 检查工号
    exists, err = s.userRepo.ExistsByUserCode(input.UserCode)
    if err != nil {
        return nil, err
    }
    if exists {
        return nil, errors.New("工号已存在")
    }

    // 密码哈希
    hash, err := utils.HashPassword(input.Password)
    if err != nil {
        return nil, err
    }

    // 创建用户
    user := &models.User{
        Username:     input.Username,
        UserCode:     input.UserCode,
        PasswordHash: hash,
        Email:        input.Email,
    }

    return s.userRepo.Create(user)
}

func (s *AuthService) Login(input LoginInput) (*LoginResult, error) {
    // 查找用户
    user, err := s.userRepo.GetByUsername(input.Username)
    if err != nil {
        return nil, errors.New("用户不存在")
    }

    // 验证密码
    if !utils.CheckPassword(input.Password, user.PasswordHash) {
        return nil, errors.New("密码错误")
    }

    // 生成Token
    accessToken, refreshToken, err := s.jwtUtil.GenerateTokens(user.ID, user.UserCode, user.Username)
    if err != nil {
        return nil, err
    }

    return &LoginResult{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        User:         user,
    }, nil
}

func (s *AuthService) GetProfile(userID string) (*models.User, error) {
    return s.userRepo.GetByID(userID)
}

func (s *AuthService) UpdateProfile(userID string, email, avatar string) error {
    user, err := s.userRepo.GetByID(userID)
    if err != nil {
        return err
    }

    user.Email = email
    user.Avatar = avatar

    return s.userRepo.Update(user)
}

func (s *AuthService) ChangePassword(userID, oldPassword, newPassword string) error {
    user, err := s.userRepo.GetByID(userID)
    if err != nil {
        return err
    }

    if !utils.CheckPassword(oldPassword, user.PasswordHash) {
        return errors.New("原密码错误")
    }

    newHash, err := utils.HashPassword(newPassword)
    if err != nil {
        return err
    }

    return s.userRepo.UpdatePassword(userID, newHash)
}
```

**交付物：**
- ✅ `internal/repository/user.go`
- ✅ `internal/repository/organization.go`
- ✅ `internal/repository/member.go`
- ✅ `internal/service/auth.go`
- ✅ `internal/service/organization.go`

---

### Week 2: API层 + 中间件（3-4天）

#### Day 5-6: Handler层 + 路由

**任务清单：**
1. ✅ AuthHandler
2. ✅ OrganizationHandler
3. ✅ 路由注册
4. ✅ JWT中间件
5. ✅ 权限中间件

**详细步骤：**

**1. Auth Handler**
```go
// internal/handlers/auth.go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/jiangfire/cornerstone/backend/internal/service"
    "github.com/jiangfire/cornerstone/backend/internal/types"
)

type AuthHandler struct {
    authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
    return &AuthHandler{authService: authService}
}

// Register POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
    var input service.RegisterInput
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, types.Error(types.ErrCodeInvalidParams, err.Error()))
        return
    }

    user, err := h.authService.Register(input)
    if err != nil {
        c.JSON(http.StatusBadRequest, types.Error(types.ErrCodeUserExists, err.Error()))
        return
    }

    c.JSON(http.StatusCreated, types.Success(gin.H{
        "id":         user.ID,
        "username":   user.Username,
        "user_code":  user.UserCode,
        "created_at": user.CreatedAt,
    }))
}

// Login POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
    var input service.LoginInput
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, types.Error(types.ErrCodeInvalidParams, err.Error()))
        return
    }

    result, err := h.authService.Login(input)
    if err != nil {
        code := types.ErrCodeInvalidPassword
        if err.Error() == "用户不存在" {
            code = types.ErrCodeUserNotFound
        }
        c.JSON(http.StatusUnauthorized, types.Error(code, err.Error()))
        return
    }

    c.JSON(http.StatusOK, types.Success(gin.H{
        "access_token":  result.AccessToken,
        "refresh_token": result.RefreshToken,
        "user": gin.H{
            "id":        result.User.ID,
            "username":  result.User.Username,
            "user_code": result.User.UserCode,
        },
    }))
}

// GetProfile GET /api/v1/auth/profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
    userID := c.GetString("user_id")
    user, err := h.authService.GetProfile(userID)
    if err != nil {
        c.JSON(http.StatusNotFound, types.Error(types.ErrCodeUserNotFound, "用户不存在"))
        return
    }

    c.JSON(http.StatusOK, types.Success(gin.H{
        "id":         user.ID,
        "username":   user.Username,
        "user_code":  user.UserCode,
        "email":      user.Email,
        "avatar":     user.Avatar,
        "created_at": user.CreatedAt,
    }))
}

// UpdateProfile PUT /api/v1/auth/profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
    userID := c.GetString("user_id")

    var input struct {
        Email  string `json:"email"`
        Avatar string `json:"avatar"`
    }
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, types.Error(types.ErrCodeInvalidParams, err.Error()))
        return
    }

    if err := h.authService.UpdateProfile(userID, input.Email, input.Avatar); err != nil {
        c.JSON(http.StatusInternalServerError, types.Error(types.ErrCodeUnknown, "更新失败"))
        return
    }

    c.JSON(http.StatusOK, types.Success(nil))
}

// ChangePassword PUT /api/v1/auth/password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
    userID := c.GetString("user_id")

    var input struct {
        OldPassword string `json:"old_password" binding:"required"`
        NewPassword string `json:"new_password" binding:"required,min=8"`
    }
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, types.Error(types.ErrCodeInvalidParams, err.Error()))
        return
    }

    if err := h.authService.ChangePassword(userID, input.OldPassword, input.NewPassword); err != nil {
        c.JSON(http.StatusBadRequest, types.Error(types.ErrCodeInvalidPassword, err.Error()))
        return
    }

    c.JSON(http.StatusOK, types.Success(nil))
}

// Logout POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
    // TODO: 加入PostgreSQL黑名单
    c.JSON(http.StatusOK, types.Success(nil))
}
```

**2. JWT中间件**
```go
// internal/middleware/auth.go
package middleware

import (
    "strings"
    "github.com/gin-gonic/gin"
    "github.com/jiangfire/cornerstone/backend/internal/pkg/jwt"
    "github.com/jiangfire/cornerstone/backend/internal/types"
)

func AuthMiddleware(jwtUtil *jwt.JWTUtil) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, types.Error(types.ErrCodeTokenInvalid, "缺少认证"))
            c.Abort()
            return
        }

        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := jwtUtil.ParseToken(tokenString)
        if err != nil {
            c.JSON(401, types.Error(types.ErrCodeTokenInvalid, "token无效"))
            c.Abort()
            return
        }

        // 设置上下文
        c.Set("user_id", claims.UserID)
        c.Set("user_code", claims.UserCode)
        c.Set("username", claims.Username)

        c.Next()
    }
}
```

**3. 路由注册**
```go
// internal/handlers/router.go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/jiangfire/cornerstone/backend/internal/config"
    "github.com/jiangfire/cornerstone/backend/internal/middleware"
    "github.com/jiangfire/cornerstone/backend/internal/pkg/jwt"
    "github.com/jiangfire/cornerstone/backend/internal/repository"
    "github.com/jiangfire/cornerstone/backend/internal/service"
    "github.com/jiangfire/cornerstone/backend/pkg/db"
)

func RegisterRoutes(r *gin.Engine) {
    cfg := config.Get() // 假设有全局配置获取函数
    jwtUtil := jwt.New(cfg.JWT.Secret, cfg.JWT.AccessExpire, cfg.JWT.RefreshExpire)

    // Repository
    userRepo := repository.NewUserRepository(db.DB())
    orgRepo := repository.NewOrganizationRepository(db.DB())
    memberRepo := repository.NewMemberRepository(db.DB())

    // Service
    authService := service.NewAuthService(userRepo, jwtUtil)
    orgService := service.NewOrganizationService(orgRepo, memberRepo, userRepo)

    // Handler
    authHandler := NewAuthHandler(authService)
    orgHandler := NewOrganizationHandler(orgService)

    // API v1
    api := r.Group("/api/v1")
    {
        // 公开路由
        auth := api.Group("/auth")
        {
            auth.POST("/register", authHandler.Register)
            auth.POST("/login", authHandler.Login)
        }

        // 需要认证的路由
        authed := api.Group("")
        authed.Use(middleware.AuthMiddleware(jwtUtil))
        {
            authed.GET("/auth/profile", authHandler.GetProfile)
            authed.PUT("/auth/profile", authHandler.UpdateProfile)
            authed.PUT("/auth/password", authHandler.ChangePassword)
            authed.POST("/auth/logout", authHandler.Logout)

            // 组织管理
            orgs := authed.Group("/organizations")
            {
                orgs.GET("", orgHandler.List)
                orgs.POST("", orgHandler.Create)

                org := orgs.Group("/:org_id")
                {
                    org.GET("", orgHandler.Get)
                    org.PUT("", middleware.OrgPermissionMiddleware("admin"), orgHandler.Update)
                    org.DELETE("", middleware.OrgPermissionMiddleware("owner"), orgHandler.Delete)

                    members := org.Group("/members")
                    {
                        members.GET("", orgHandler.ListMembers)
                        members.POST("", middleware.OrgPermissionMiddleware("admin"), orgHandler.AddMember)
                        members.PUT("/:user_id", middleware.OrgPermissionMiddleware("admin"), orgHandler.UpdateMemberRole)
                        members.DELETE("/:user_id", middleware.OrgPermissionMiddleware("admin"), orgHandler.RemoveMember)
                        members.DELETE("/leave", orgHandler.LeaveOrganization)
                    }
                }
            }
        }
    }
}
```

**交付物：**
- ✅ `internal/handlers/auth.go`
- ✅ `internal/handlers/organization.go`
- ✅ `internal/handlers/router.go`
- ✅ `internal/middleware/auth.go`
- ✅ `internal/middleware/permission.go`

---

### Week 3: 前端开发（3-4天）

#### Day 8-10: 前端API客户端 + 页面

**任务清单：**
1. ✅ API客户端封装
2. ✅ Pinia状态管理
3. ✅ 认证页面（登录/注册）
4. ✅ 组织管理页面

**详细步骤：**

**1. API客户端**
```typescript
// frontend/src/api/request.ts
import axios from 'axios'

const API_BASE = 'http://localhost:8080/api/v1'

const request = axios.create({
  baseURL: API_BASE,
  timeout: 10000,
})

// 请求拦截器
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器
request.interceptors.response.use(
  (response) => {
    const { code, data, message } = response.data
    if (code !== 0) {
      return Promise.reject(new Error(message))
    }
    return data
  },
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('access_token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default request
```

```typescript
// frontend/src/api/auth.ts
import request from './request'

export interface LoginParams {
  username: string
  password: string
}

export interface RegisterParams {
  username: string
  user_code: string
  password: string
  email?: string
}

export interface User {
  id: string
  username: string
  user_code: string
  email?: string
  avatar?: string
}

export const login = (params: LoginParams) => {
  return request.post('/auth/login', params)
}

export const register = (params: RegisterParams) => {
  return request.post('/auth/register', params)
}

export const getProfile = () => {
  return request.get('/auth/profile')
}

export const updateProfile = (data: { email?: string; avatar?: string }) => {
  return request.put('/auth/profile', data)
}

export const changePassword = (data: { old_password: string; new_password: string }) => {
  return request.put('/auth/password', data)
}

export const logout = () => {
  return request.post('/auth/logout')
}
```

**2. Pinia状态管理**
```typescript
// frontend/src/stores/auth.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login, register, getProfile, logout as apiLogout, User } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref(localStorage.getItem('access_token') || '')

  const isAuthenticated = computed(() => !!token.value)

  const setToken = (t: string) => {
    token.value = t
    localStorage.setItem('access_token', t)
  }

  const clearAuth = () => {
    user.value = null
    token.value = ''
    localStorage.removeItem('access_token')
  }

  const handleLogin = async (username: string, password: string) => {
    const data = await login({ username, password })
    setToken(data.access_token)
    user.value = data.user
    return data
  }

  const handleRegister = async (params: RegisterParams) => {
    return await register(params)
  }

  const fetchProfile = async () => {
    if (!token.value) return
    try {
      user.value = await getProfile()
    } catch (error) {
      clearAuth()
      throw error
    }
  }

  const handleLogout = async () => {
    try {
      await apiLogout()
    } finally {
      clearAuth()
    }
  }

  return {
    user,
    token,
    isAuthenticated,
    handleLogin,
    handleRegister,
    fetchProfile,
    handleLogout,
  }
})
```

**3. 登录页面**
```vue
<!-- frontend/src/views/auth/Login.vue -->
<template>
  <div class="login-container">
    <div class="login-card">
      <h2>登录</h2>
      <el-form :model="form" :rules="rules" @submit.prevent="handleSubmit">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" placeholder="请输入密码" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit" :loading="loading" style="width: 100%">
            登录
          </el-button>
        </el-form-item>
      </el-form>
      <div class="links">
        <router-link to="/register">没有账号？立即注册</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const form = reactive({
  username: '',
  password: '',
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

const loading = ref(false)

const handleSubmit = async () => {
  loading.value = true
  try {
    await authStore.handleLogin(form.username, form.password)
    ElMessage.success('登录成功')
    router.push('/')
  } catch (error: any) {
    ElMessage.error(error.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background: #f5f5f5;
}
.login-card {
  width: 400px;
  padding: 40px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}
.links {
  text-align: center;
  margin-top: 16px;
}
</style>
```

**4. 组织管理页面**
```vue
<!-- frontend/src/views/org/OrganizationList.vue -->
<template>
  <div class="org-list">
    <div class="header">
      <h2>我的组织</h2>
      <el-button type="primary" @click="showCreateDialog = true">创建组织</el-button>
    </div>

    <el-table :data="organizations" v-loading="loading">
      <el-table-column prop="name" label="组织名称" />
      <el-table-column prop="role" label="我的角色" />
      <el-table-column prop="member_count" label="成员数" />
      <el-table-column prop="created_at" label="创建时间" />
      <el-table-column label="操作">
        <template #default="{ row }">
          <el-button size="small" @click="viewOrganization(row.id)">查看</el-button>
          <el-button v-if="row.role === 'owner'" size="small" type="danger" @click="deleteOrganization(row.id)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 创建组织对话框 -->
    <el-dialog v-model="showCreateDialog" title="创建组织">
      <el-form :model="createForm" :rules="createRules">
        <el-form-item label="组织名称" prop="name">
          <el-input v-model="createForm.name" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="createForm.description" type="textarea" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listOrganizations, createOrganization, deleteOrganization } from '@/api/organization'

const router = useRouter()
const organizations = ref([])
const loading = ref(false)
const showCreateDialog = ref(false)
const creating = ref(false)

const createForm = ref({
  name: '',
  description: '',
})

const createRules = {
  name: [{ required: true, message: '请输入组织名称', trigger: 'blur' }],
}

const fetchOrganizations = async () => {
  loading.value = true
  try {
    const data = await listOrganizations()
    organizations.value = data.list
  } catch (error: any) {
    ElMessage.error(error.message)
  } finally {
    loading.value = false
  }
}

const handleCreate = async () => {
  creating.value = true
  try {
    await createOrganization(createForm.value)
    ElMessage.success('创建成功')
    showCreateDialog.value = false
    createForm.value = { name: '', description: '' }
    fetchOrganizations()
  } catch (error: any) {
    ElMessage.error(error.message)
  } finally {
    creating.value = false
  }
}

const viewOrganization = (id: string) => {
  router.push(`/organizations/${id}`)
}

const deleteOrganization = async (id: string) => {
  try {
    await ElMessageBox.confirm('确定要删除该组织吗？', '警告', {
      type: 'warning',
    })
    await deleteOrganization(id)
    ElMessage.success('删除成功')
    fetchOrganizations()
  } catch (error) {
    // 取消
  }
}

onMounted(() => {
  fetchOrganizations()
})
</script>

<style scoped>
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
</style>
```

**交付物：**
- ✅ `frontend/src/api/request.ts`
- ✅ `frontend/src/api/auth.ts`
- ✅ `frontend/src/api/organization.ts`
- ✅ `frontend/src/stores/auth.ts`
- ✅ `frontend/src/stores/organization.ts`
- ✅ `frontend/src/views/auth/Login.vue`
- ✅ `frontend/src/views/auth/Register.vue`
- ✅ `frontend/src/views/org/OrganizationList.vue`
- ✅ `frontend/src/views/org/OrganizationDetail.vue`

---

### Week 4: 测试 + 集成（3-4天）

#### Day 11-14: 测试与优化

**任务清单：**
1. ✅ 单元测试（Service层）
2. ✅ 集成测试（API层）
3. ✅ E2E测试（前端）
4. ✅ 性能测试
5. ✅ 文档完善

**测试示例：**

```go
// internal/service/auth_test.go
func TestAuthService_Register(t *testing.T) {
    // Mock
    mockRepo := new(MockUserRepository)
    mockRepo.On("ExistsByUsername", "test").Return(false, nil)
    mockRepo.On("ExistsByUserCode", "EMP001").Return(false, nil)
    mockRepo.On("Create", mock.Anything).Return(&models.User{ID: "usr_001"}, nil)

    jwtUtil := jwt.New("secret", 3600, 604800)
    service := NewAuthService(mockRepo, jwtUtil)

    // Test
    input := RegisterInput{
        Username: "test",
        UserCode: "EMP001",
        Password: "P@ssw0rd123",
    }
    user, err := service.Register(input)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "usr_001", user.ID)
}
```

---

## 📊 里程碑检查点

### Week 1 完成标准
- [ ] 后端项目结构完整
- [ ] 数据库表创建完成
- [ ] JWT工具可用
- [ ] 配置文件完整

### Week 2 完成标准
- [ ] 用户注册/登录API可用
- [ ] 组织管理API可用
- [ ] JWT中间件正常工作
- [ ] 权限中间件正常工作

### Week 3 完成标准
- [ ] 前端API客户端完成
- [ ] 登录/注册页面可用
- [ ] 组织管理页面可用
- [ ] 状态管理正常

### Week 4 完成标准
- [ ] 单元测试覆盖率 >80%
- [ ] 集成测试通过
- [ ] E2E测试通过
- [ ] 文档更新完成

---

## 🔧 环境变量配置（12-Factor App）

### 后端环境变量
```bash
# 服务器配置
PORT=8080
MODE=debug  # debug | release

# 数据库配置
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=cornerstone
DATABASE_TIMEZONE=Asia/Shanghai
DATABASE_SSL_MODE=disable
DATABASE_MAX_IDLE_CONNS=10
DATABASE_MAX_OPEN_CONNS=50

# JWT配置
JWT_SECRET=your-secret-key-change-in-production
JWT_ACCESS_EXPIRE=3600      # 1小时
JWT_REFRESH_EXPIRE=604800   # 7天

# 日志配置
LOG_PATH=./logs/app.log
LOG_LEVEL=info
LOG_MAX_SIZE=100
LOG_MAX_BACKUPS=10
LOG_MAX_AGE=7
LOG_COMPRESS=true

# 安全配置
BCRYPT_COST=12
PASSWORD_MIN_LENGTH=8
PLUGIN_TIMEOUT=5
UPLOAD_MAX_SIZE=100MB

# 文件存储
STORAGE_TYPE=local  # local | minio
STORAGE_PATH=./uploads

# MinIO配置（可选）
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=cornerstone
```

### 前端环境变量 (.env)
```bash
# 开发环境
VITE_API_BASE=http://localhost:8080/api/v1
VITE_APP_TITLE=硬件工程数据管理平台

# 生产环境（.env.production）
VITE_API_BASE=/api/v1
```

### Docker Compose 环境变量
```yaml
# docker-compose.yml
services:
  backend:
    environment:
      - PORT=8080
      - MODE=release
      - DATABASE_HOST=db
      - DATABASE_PORT=5432
      - DATABASE_USER=postgres
      - DATABASE_PASSWORD=${DB_PASSWORD}
      - DATABASE_NAME=cornerstone
      - JWT_SECRET=${JWT_SECRET}
      - PLUGIN_TIMEOUT=5
      - UPLOAD_MAX_SIZE=100MB
    depends_on:
      - db

  db:
    environment:
      - POSTGRES_DB=cornerstone
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=${DB_PASSWORD}
```

---

## 📝 开发规范

### 1. Git提交规范
```
feat(auth): add user registration endpoint
fix(api): fix pagination bug
docs: update API documentation
```

### 2. 代码风格
- 后端：Go标准格式 + golangci-lint
- 前端：ESLint + Prettier + TypeScript

### 3. 测试要求
- 单元测试：Service层必须覆盖
- 集成测试：关键API路径
- E2E测试：核心用户流程

---

## 🎯 下一步行动

### 立即开始（Day 1）
1. ✅ 创建 `backend/cmd/server/main.go`
2. ✅ 创建 `internal/config/config.go` (读取环境变量)
3. ✅ 配置环境变量 `.env` 文件
4. ✅ 启动PostgreSQL

### 验证清单
- [ ] Go mod tidy 通过
- [ ] 环境变量加载正常
- [ ] 数据库连接成功
- [ ] 日志输出正常
- [ ] 服务可以启动

---

## 📞 需要帮助？

如果在实施过程中遇到问题：
1. 查看 [ARCHITECTURE.md](./ARCHITECTURE.md) 了解系统设计
2. 查看 [API.md](./API.md) 了解接口规范
3. 查看 [DATABASE.md](./DATABASE.md) 了解数据库设计

---

**文档版本**: v1.0
**最后更新**: 2026-01-06
**状态**: 📋 待评审
