# 用户认证 + 组织管理 - 实现计划文档

**版本**: v1.0
**日期**: 2026-01-05
**状态**: 📋 待评审

---

## 📋 关联文档

本文档是**技术设计文档**，详细说明了用户认证和组织管理的实现方案。

**如需查看开发执行计划，请参考：**
- [PLAN.md](./PLAN.md) - 按Day分解的开发步骤（推荐开发时使用）
- [GUIDE.md](./GUIDE.md) - 项目快速导航

**其他相关文档：**
- [DATABASE.md](./DATABASE.md) - 完整数据库设计
- [ARCHITECTURE.md](./ARCHITECTURE.md) - 技术架构设计
- [API.md](./API.md) - 完整接口规范

---

## 📖 文档用途

| 文档 | 用途 | 使用场景 |
|------|------|----------|
| **本文档** | 技术设计评审 | 1. 方案设计阶段<br>2. 技术细节查阅<br>3. 代码实现参考 |
| **PLAN.md** | 开发执行手册 | 1. 日常开发<br>2. 按步骤编码<br>3. 任务跟踪 |

---

## 一、技术栈选型

| 组件 | 技术选型 | 说明 |
|------|----------|------|
| **语言** | Go 1.21 | 高性能、强类型 |
| **Web框架** | Gin | 轻量、高性能、社区成熟 |
| **ORM** | GORM | 功能完善、支持JSONB |
| **数据库** | PostgreSQL 15 | 支持JSONB、UUID、物化视图、缓存 |
| **JWT库** | golang-jwt/jwt v5 | 标准、安全 |
| **密码哈希** | bcrypt | 慢哈希、防彩虹表 |
| **UUID** | google/uuid | 标准UUID生成 |

---

## 二、数据模型设计

### 2.1 Users 表（用户表）

```go
type User struct {
    ID           string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
    Username     string    `gorm:"uniqueIndex;not null;size:50" json:"username"`      // 用户名（唯一）
    UserCode     string    `gorm:"uniqueIndex;not null;size:20" json:"user_code"`     // 工号（唯一）
    PasswordHash string    `gorm:"not null" json:"-"`                                 // 密码哈希（不返回）
    Role         string    `gorm:"not null;default:'user'" json:"role"`               // 角色：admin/user
    Email        string    `gorm:"index;size:100" json:"email"`                       // 邮箱（可选）
    Avatar       string    `gorm:"size:255" json:"avatar"`                            // 头像URL
    IsActive     bool      `gorm:"default:true" json:"is_active"`                     // 是否激活
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// 前缀：usr_
// 示例ID：usr_001f7a8b-3c2d-4e5f-6a7b-8c9d0e1f2a3b
```

**字段说明：**
- `username`: 用户登录名，支持中文，唯一
- `user_code`: 工号，唯一，用于企业身份标识
- `password_hash`: 使用bcrypt（cost=12）存储

**关键索引：**
```sql
CREATE UNIQUE INDEX idx_users_username ON users(username);
CREATE UNIQUE INDEX idx_users_user_code ON users(user_code);
CREATE INDEX idx_users_email ON users(email);
```

---

### 2.2 Organizations 表（组织表）

```go
type Organization struct {
    ID          string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
    Name        string    `gorm:"not null;size:100" json:"name"`                     // 组织名称
    Description string    `gorm:"size:500" json:"description"`                       // 描述
    OwnerID     string    `gorm:"not null;type:uuid" json:"owner_id"`                // 所有者ID
    Avatar      string    `gorm:"size:255" json:"avatar"`                            // 组织头像
    IsActive    bool      `gorm:"default:true" json:"is_active"`                     // 是否激活
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`

    // 关联
    Owner User `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

// 前缀：org_
// 示例ID：org_001f7a8b-3c2d-4e5f-6a7b-8c9d0e1f2a3b
```

**关键索引：**
```sql
CREATE INDEX idx_organizations_owner ON organizations(owner_id);
CREATE INDEX idx_organizations_name ON organizations(name);
```

---

### 2.3 OrganizationMembers 表（组织成员表）

```go
type OrganizationMember struct {
    OrganizationID string    `gorm:"primaryKey;type:uuid" json:"organization_id"`
    UserID         string    `gorm:"primaryKey;type:uuid" json:"user_id"`
    Role           string    `gorm:"type:varchar(20);not null" json:"role"`          // owner/admin/member
    JoinedAt       time.Time `gorm:"default:now()" json:"joined_at"`

    // 关联
    User         User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
    Organization Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

// 复合主键：(organization_id, user_id)
// 前缀：无
```

**角色枚举：**
- `owner`: 组织所有者（唯一），拥有所有权限
- `admin`: 组织管理员，可管理成员和数据库
- `member`: 普通成员，需要手动授权数据库

**关键索引：**
```sql
CREATE UNIQUE INDEX idx_org_members_composite ON organization_members(organization_id, user_id);
CREATE INDEX idx_org_members_user ON organization_members(user_id);
CREATE INDEX idx_org_members_role ON organization_members(role);
```

---

### 2.4 TokenBlacklist 表（Token黑名单）

```go
type TokenBlacklist struct {
    TokenHash string    `gorm:"primaryKey;type:varchar(64)" json:"-"`  // token的SHA256哈希
    ExpiredAt time.Time `gorm:"not null" json:"expired_at"`           // 过期时间
    CreatedAt time.Time `gorm:"default:now()" json:"created_at"`
}

// 前缀：无
// 用途：存储已登出但未过期的JWT token
```

**设计说明：**
- 使用SHA256哈希存储token，避免存储明文
- 依赖过期时间自动清理（定期任务）
- 主键查询性能优异

**关键索引：**
```sql
CREATE INDEX idx_blacklist_expired ON token_blacklist(expired_at)
WHERE expired_at > NOW();

-- 定期清理过期记录（每天凌晨执行）
-- DELETE FROM token_blacklist WHERE expired_at < NOW();
```

**替代Redis的理由：**
1. **简化架构**：单一数据库，无需额外服务
2. **数据一致性**：避免Redis与PostgreSQL数据不一致
3. **运维简单**：无需维护Redis服务
4. **性能足够**：主键查询 + 定期清理，满足MVP需求

---

## 三、API 接口设计

### 3.1 用户认证模块

#### 注册
```
POST /api/v1/auth/register
Content-Type: application/json

请求体：
{
  "username": "zhangsan",
  "user_code": "EMP001",
  "password": "P@ssw0rd123",
  "email": "zhangsan@company.com"
}

响应成功 (201):
{
  "code": 0,
  "message": "注册成功",
  "data": {
    "id": "usr_xxx",
    "username": "zhangsan",
    "user_code": "EMP001",
    "email": "zhangsan@company.com",
    "created_at": "2026-01-05T10:00:00Z"
  }
}

响应失败 (400):
{
  "code": 1001,
  "message": "用户已存在",
  "data": null
}
```

#### 登录
```
POST /api/v1/auth/login
Content-Type: application/json

请求体：
{
  "username": "zhangsan",
  "password": "P@ssw0rd123"
}

响应成功 (200):
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 3600,
    "user": {
      "id": "usr_xxx",
      "username": "zhangsan",
      "user_code": "EMP001"
    }
  }
}
```

#### 登出
```
POST /api/v1/auth/logout
Authorization: Bearer <access_token>

响应 (200):
{
  "code": 0,
  "message": "登出成功",
  "data": null
}
```

#### 刷新Token
```
POST /api/v1/auth/refresh
Authorization: Bearer <refresh_token>

响应 (200):
{
  "code": 0,
  "message": "刷新成功",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600
  }
}
```

#### 获取个人信息
```
GET /api/v1/auth/profile
Authorization: Bearer <access_token>

响应 (200):
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "usr_xxx",
    "username": "zhangsan",
    "user_code": "EMP001",
    "email": "zhangsan@company.com",
    "avatar": "https://...",
    "created_at": "2026-01-05T10:00:00Z"
  }
}
```

#### 更新个人信息
```
PUT /api/v1/auth/profile
Authorization: Bearer <access_token>

请求体：
{
  "email": "new@email.com",
  "avatar": "https://new-avatar.com"
}

响应 (200):
{
  "code": 0,
  "message": "更新成功",
  "data": {
    "id": "usr_xxx",
    "email": "new@email.com",
    "avatar": "https://new-avatar.com"
  }
}
```

#### 修改密码
```
PUT /api/v1/auth/password
Authorization: Bearer <access_token>

请求体：
{
  "old_password": "P@ssw0rd123",
  "new_password": "NewP@ssw0rd456"
}

响应 (200):
{
  "code": 0,
  "message": "密码修改成功",
  "data": null
}
```

---

### 3.2 组织管理模块

#### 获取我加入的组织列表
```
GET /api/v1/organizations?page=1&size=10
Authorization: Bearer <access_token>

响应 (200):
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      {
        "id": "org_xxx",
        "name": "硬件研发部",
        "description": "负责硬件产品开发",
        "role": "owner",           // 我在该组织中的角色
        "member_count": 15,
        "created_at": "2026-01-05T10:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "size": 10,
      "total": 1
    }
  }
}
```

#### 创建组织
```
POST /api/v1/organizations
Authorization: Bearer <access_token>

请求体：
{
  "name": "硬件测试组",
  "description": "负责硬件产品测试"
}

响应 (201):
{
  "code": 0,
  "message": "组织创建成功",
  "data": {
    "id": "org_yyy",
    "name": "硬件测试组",
    "owner_id": "usr_xxx",
    "role": "owner"
  }
}
```

#### 获取组织详情
```
GET /api/v1/organizations/:org_id
Authorization: Bearer <access_token>

响应 (200):
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "org_xxx",
    "name": "硬件研发部",
    "description": "负责硬件产品开发",
    "owner": {
      "id": "usr_xxx",
      "username": "zhangsan",
      "user_code": "EMP001"
    },
    "my_role": "owner",
    "member_count": 15,
    "created_at": "2026-01-05T10:00:00Z"
  }
}
```

#### 更新组织信息
```
PUT /api/v1/organizations/:org_id
Authorization: Bearer <access_token>
权限要求：owner 或 admin

请求体：
{
  "name": "硬件研发部（新）",
  "description": "负责硬件产品开发和测试"
}

响应 (200):
{
  "code": 0,
  "message": "更新成功",
  "data": {
    "id": "org_xxx",
    "name": "硬件研发部（新）",
    "description": "负责硬件产品开发和测试"
  }
}
```

#### 删除组织
```
DELETE /api/v1/organizations/:org_id
Authorization: Bearer <access_token>
权限要求：owner

响应 (200):
{
  "code": 0,
  "message": "组织已删除",
  "data": null
}
```

#### 获取组织成员列表
```
GET /api/v1/organizations/:org_id/members?page=1&size=20
Authorization: Bearer <access_token>
权限要求：owner/admin/member

响应 (200):
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      {
        "user_id": "usr_xxx",
        "username": "zhangsan",
        "user_code": "EMP001",
        "role": "owner",
        "joined_at": "2026-01-05T10:00:00Z"
      },
      {
        "user_id": "usr_yyy",
        "username": "lisi",
        "user_code": "EMP002",
        "role": "member",
        "joined_at": "2026-01-06T10:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "size": 20,
      "total": 2
    }
  }
}
```

#### 邀请/添加成员
```
POST /api/v1/organizations/:org_id/members
Authorization: Bearer <access_token>
权限要求：owner 或 admin

请求体：
{
  "user_code": "EMP003",  // 通过工号添加
  "role": "member"        // 默认 member，owner/admin 需要 owner 权限
}

响应 (201):
{
  "code": 0,
  "message": "成员添加成功",
  "data": {
    "organization_id": "org_xxx",
    "user_id": "usr_zzz",
    "role": "member"
  }
}
```

#### 修改成员角色
```
PUT /api/v1/organizations/:org_id/members/:user_id
Authorization: Bearer <access_token>
权限要求：owner 或 admin（不能修改owner）

请求体：
{
  "role": "admin"  // member/admin
}

响应 (200):
{
  "code": 0,
  "message": "角色更新成功",
  "data": {
    "user_id": "usr_yyy",
    "role": "admin"
  }
}
```

#### 移除成员
```
DELETE /api/v1/organizations/:org_id/members/:user_id
Authorization: Bearer <access_token>
权限要求：owner 或 admin（不能移除owner）

响应 (200):
{
  "code": 0,
  "message": "成员已移除",
  "data": null
}
```

#### 退出组织
```
DELETE /api/v1/organizations/:org_id/members/leave
Authorization: Bearer <access_token>

响应 (200):
{
  "code": 0,
  "message": "已退出组织",
  "data": null
}
```

---

## 四、核心实现细节

### 4.1 JWT Token 设计

**Token 结构：**
```go
type Claims struct {
    UserID   string `json:"user_id"`
    UserCode string `json:"user_code"`
    Username string `json:"username"`
    Role     string `json:"role"`  // 全局角色（预留）
    jwt.RegisteredClaims
}

// Access Token: 1小时过期
// Refresh Token: 7天过期
```

**生成逻辑：**
```go
func GenerateTokens(userID, userCode, username string) (accessToken, refreshToken string, err error) {
    // Access Token
    accessClaims := &Claims{
        UserID:   userID,
        UserCode: userCode,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Subject:   userID,
        },
    }
    accessToken, _ = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(secret))

    // Refresh Token
    refreshClaims := &Claims{
        UserID:   userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Subject:   userID,
        },
    }
    refreshToken, _ = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(secret))

    return accessToken, refreshToken, nil
}
```

**Token 黑名单（PostgreSQL）：**
```go
// 登出：将token加入黑名单
func AddToBlacklist(tokenString string) error {
    // 计算token的SHA256哈希
    hash := sha256.Sum256([]byte(tokenString))
    tokenHash := hex.EncodeToString(hash[:])

    // 解析token获取过期时间
    token, _, _ := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
    claims := token.Claims.(*Claims)

    // 存入PostgreSQL
    blacklist := &models.TokenBlacklist{
        TokenHash: tokenHash,
        ExpiredAt: claims.ExpiresAt.Time,
    }

    return db.Create(blacklist).Error
}

// 验证：检查token是否在黑名单
func IsBlacklisted(tokenString string) bool {
    hash := sha256.Sum256([]byte(tokenString))
    tokenHash := hex.EncodeToString(hash[:])

    var count int64
    db.Model(&models.TokenBlacklist{}).
        Where("token_hash = ? AND expired_at > NOW()", tokenHash).
        Count(&count)

    return count > 0
}

// 定期清理（每天凌晨执行）
func CleanupExpiredTokens() error {
    return db.Where("expired_at < NOW()").Delete(&models.TokenBlacklist{}).Error
}
```

**为什么不用Redis：**
- PostgreSQL主键查询性能足够快（<1ms）
- 避免额外的服务依赖
- 数据一致性保证
- 无需担心缓存失效问题

---

### 4.2 密码安全

```go
const BcryptCost = 12

// 密码哈希
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
    return string(bytes), err
}

// 密码验证
func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

---

### 4.3 权限校验中间件

```go
// AuthMiddleware - 验证JWT
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 从Header获取token
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"code": 401, "message": "缺少认证"})
            c.Abort()
            return
        }

        // 2. 解析token
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := ParseToken(tokenString)
        if err != nil {
            c.JSON(401, gin.H{"code": 401, "message": "token无效"})
            c.Abort()
            return
        }

        // 3. 检查黑名单
        if IsBlacklisted(tokenString) {
            c.JSON(401, gin.H{"code": 401, "message": "token已失效"})
            c.Abort()
            return
        }

        // 4. 设置上下文
        c.Set("user_id", claims.UserID)
        c.Set("user_code", claims.UserCode)
        c.Set("username", claims.Username)

        c.Next()
    }
}

// OrgPermissionMiddleware - 组织权限校验
func OrgPermissionMiddleware(minRole string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        orgID := c.Param("org_id")

        // 角色层级
        roleLevel := map[string]int{
            "member": 1,
            "admin":  2,
            "owner":  3,
        }

        // 查询用户在该组织中的角色（使用物化视图）
        var member struct {
            Role string
        }
        err := db.Raw(`
            SELECT role FROM user_database_permissions
            WHERE user_id = ? AND database_id = ?
        `, userID, orgID).Scan(&member).Error

        if err != nil {
            c.JSON(403, gin.H{"code": 403, "message": "无权访问该组织"})
            c.Abort()
            return
        }

        // 权限校验
        if roleLevel[member.Role] < roleLevel[minRole] {
            c.JSON(403, gin.H{"code": 403, "message": "权限不足"})
            c.Abort()
            return
        }

        c.Set("org_role", member.Role)
        c.Next()
    }
}
```

**权限校验优化：**
- 使用物化视图 `user_database_permissions` 替代Redis缓存
- PostgreSQL原生支持，无需额外服务
- 5分钟自动刷新，性能优异

**使用示例：**
```go
router := gin.Default()
auth := router.Group("/api/v1")
{
    auth.Use(AuthMiddleware())
    {
        // 用户认证
        auth.GET("/auth/profile", GetProfile)
        auth.PUT("/auth/profile", UpdateProfile)
        auth.PUT("/auth/password", ChangePassword)
        auth.POST("/auth/logout", Logout)

        // 组织管理
        orgs := auth.Group("/organizations")
        {
            orgs.GET("", ListOrganizations)
            orgs.POST("", CreateOrganization)

            org := orgs.Group("/:org_id")
            {
                org.GET("", GetOrganization)
                org.PUT("", OrgPermissionMiddleware("admin"), UpdateOrganization)
                org.DELETE("", OrgPermissionMiddleware("owner"), DeleteOrganization)

                // 成员管理
                members := org.Group("/members")
                {
                    members.GET("", ListMembers)
                    members.POST("", OrgPermissionMiddleware("admin"), AddMember)
                    members.PUT("/:user_id", OrgPermissionMiddleware("admin"), UpdateMemberRole)
                    members.DELETE("/:user_id", OrgPermissionMiddleware("admin"), RemoveMember)
                    members.DELETE("/leave", LeaveOrganization)
                }
            }
        }
    }
}
```

---

### 4.4 业务逻辑层

#### AuthService
```go
type AuthService struct {
    userRepo repository.UserRepository
}

// 注册
func (s *AuthService) Register(input RegisterInput) (*User, error) {
    // 1. 检查用户名/工号是否存在
    if exists, _ := s.userRepo.ExistsByUsername(input.Username); exists {
        return nil, ErrUserExists
    }
    if exists, _ := s.userRepo.ExistsByUserCode(input.UserCode); exists {
        return nil, ErrUserExists
    }

    // 2. 哈希密码
    hash, err := HashPassword(input.Password)
    if err != nil {
        return nil, err
    }

    // 3. 创建用户
    user := &User{
        Username:     input.Username,
        UserCode:     input.UserCode,
        PasswordHash: hash,
        Email:        input.Email,
    }

    return s.userRepo.Create(user)
}

// 登录
func (s *AuthService) Login(input LoginInput) (*LoginResult, error) {
    // 1. 查找用户
    user, err := s.userRepo.GetByUsername(input.Username)
    if err != nil {
        return nil, ErrUserNotFound
    }

    // 2. 验证密码
    if !CheckPassword(input.Password, user.PasswordHash) {
        return nil, ErrInvalidPassword
    }

    // 3. 生成Token
    accessToken, refreshToken, err := GenerateTokens(user.ID, user.UserCode, user.Username)
    if err != nil {
        return nil, err
    }

    // 4. 返回结果
    return &LoginResult{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        User:         user,
    }, nil
}

// 登出（使用PostgreSQL黑名单）
func (s *AuthService) Logout(tokenString string) error {
    // 调用全局函数加入黑名单
    return AddToBlacklist(tokenString)
}
```

#### OrganizationService
```go
type OrganizationService struct {
    orgRepo  repository.OrganizationRepository
    memberRepo repository.MemberRepository
    userRepo repository.UserRepository
}

// 创建组织
func (s *OrganizationService) Create(userID string, input CreateOrgInput) (*Organization, error) {
    // 1. 创建组织
    org := &Organization{
        Name:        input.Name,
        Description: input.Description,
        OwnerID:     userID,
    }

    org, err := s.orgRepo.Create(org)
    if err != nil {
        return nil, err
    }

    // 2. 添加所有者到成员表
    member := &OrganizationMember{
        OrganizationID: org.ID,
        UserID:         userID,
        Role:           "owner",
    }

    if err := s.memberRepo.Create(member); err != nil {
        // 回滚组织创建
        s.orgRepo.Delete(org.ID)
        return nil, err
    }

    return org, nil
}

// 邀请成员
func (s *OrganizationService) AddMember(userID, orgID, userCode string, role string) error {
    // 1. 验证邀请者权限（已在中间件处理）

    // 2. 查找被邀请用户
    invitee, err := s.userRepo.GetByUserCode(userCode)
    if err != nil {
        return ErrUserNotFound
    }

    // 3. 检查是否已在组织中
    exists, _ := s.memberRepo.Exists(orgID, invitee.ID)
    if exists {
        return errors.New("用户已在组织中")
    }

    // 4. 验证角色权限
    if role == "owner" {
        return errors.New("不能直接邀请owner")
    }

    // 5. 添加成员
    member := &OrganizationMember{
        OrganizationID: orgID,
        UserID:         invitee.ID,
        Role:           role,
    }

    return s.memberRepo.Create(member)
}

// 获取用户可访问的组织列表
func (s *OrganizationService) ListUserOrganizations(userID string) ([]OrganizationWithRole, error) {
    // 查询用户在所有组织中的角色
    members, err := s.memberRepo.GetByUserID(userID)
    if err != nil {
        return nil, err
    }

    var result []OrganizationWithRole
    for _, member := range members {
        org, err := s.orgRepo.GetByID(member.OrganizationID)
        if err != nil {
            continue
        }

        result = append(result, OrganizationWithRole{
            Organization: org,
            Role:         member.Role,
        })
    }

    return result, nil
}
```

---

### 4.5 Repository 层

```go
// UserRepository
type UserRepository interface {
    Create(user *User) (*User, error)
    GetByID(id string) (*User, error)
    GetByUsername(username string) (*User, error)
    GetByUserCode(userCode string) (*User, error)
    ExistsByUsername(username string) (bool, error)
    ExistsByUserCode(userCode string) (bool, error)
    Update(user *User) error
    UpdatePassword(id string, hash string) error
}

// OrganizationRepository
type OrganizationRepository interface {
    Create(org *Organization) (*Organization, error)
    GetByID(id string) (*Organization, error)
    Update(org *Organization) error
    Delete(id string) error
    Exists(id string) (bool, error)
    GetByOwnerID(ownerID string) ([]Organization, error)
}

// MemberRepository
type MemberRepository interface {
    Create(member *OrganizationMember) error
    Get(orgID, userID string) (*OrganizationMember, error)
    GetByOrgID(orgID string) ([]OrganizationMember, error)
    GetByUserID(userID string) ([]OrganizationMember, error)
    UpdateRole(orgID, userID, role string) error
    Delete(orgID, userID string) error
    Exists(orgID, userID string) (bool, error)
    CountByOrgID(orgID string) (int64, error)
}
```

---

## 五、数据库迁移

### 5.1 GORM 自动迁移

```go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &User{},
        &Organization{},
        &OrganizationMember{},
    )
}
```

### 5.2 手动优化索引

```sql
-- 用户表索引
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_users_is_active ON users(is_active) WHERE is_active = true;

-- 组织表索引
CREATE INDEX idx_organizations_owner_created ON organizations(owner_id, created_at DESC);
CREATE INDEX idx_organizations_is_active ON organizations(is_active) WHERE is_active = true;

-- 成员表索引（复合索引优化查询）
CREATE INDEX idx_org_members_user_org ON organization_members(user_id, organization_id);
CREATE INDEX idx_org_members_org_role ON organization_members(organization_id, role);

-- 部分索引（优化活跃用户查询）
CREATE INDEX idx_org_members_active ON organization_members(organization_id)
WHERE role IN ('owner', 'admin');
```

---

## 六、错误码定义

```go
const (
    // 通用错误
    ErrCodeSuccess       = 0
    ErrCodeUnknown       = 1
    ErrCodeInvalidParams = 1000

    // 用户认证 (1000-1099)
    ErrCodeUserExists      = 1001
    ErrCodeUserNotFound    = 1002
    ErrCodeInvalidPassword = 1003
    ErrCodeTokenInvalid    = 1004
    ErrCodeTokenExpired    = 1005

    // 组织管理 (1100-1199)
    ErrCodeOrgNotFound      = 1101
    ErrCodePermissionDenied = 1102
    ErrCodeMemberExists     = 1103
    ErrCodeCannotRemoveOwner = 1104

    // 数据库 (1200-1299)
    ErrCodeDatabaseError = 1201
)

var ErrorMessages = map[int]string{
    ErrCodeSuccess:          "success",
    ErrCodeUnknown:          "未知错误",
    ErrCodeInvalidParams:    "参数错误",
    ErrCodeUserExists:       "用户已存在",
    ErrCodeUserNotFound:     "用户不存在",
    ErrCodeInvalidPassword:  "密码错误",
    ErrCodeTokenInvalid:     "token无效",
    ErrCodeTokenExpired:     "token已过期",
    ErrCodeOrgNotFound:      "组织不存在",
    ErrCodePermissionDenied: "权限不足",
    ErrCodeMemberExists:     "成员已存在",
    ErrCodeCannotRemoveOwner: "不能移除组织所有者",
    ErrCodeDatabaseError:    "数据库错误",
}
```

---

## 七、测试策略

### 7.1 单元测试

```go
// service/auth_test.go
func TestAuthService_Register(t *testing.T) {
    // Mock repository
    mockRepo := new(MockUserRepository)
    mockRepo.On("ExistsByUsername", "zhangsan").Return(false, nil)
    mockRepo.On("ExistsByUserCode", "EMP001").Return(false, nil)
    mockRepo.On("Create", mock.Anything).Return(&User{ID: "usr_001"}, nil)

    service := NewAuthService(mockRepo, nil)

    input := RegisterInput{
        Username: "zhangsan",
        UserCode: "EMP001",
        Password: "P@ssw0rd123",
    }

    user, err := service.Register(input)
    assert.NoError(t, err)
    assert.Equal(t, "usr_001", user.ID)
}
```

### 7.2 集成测试

```go
// handler/auth_test.go
func TestRegisterAPI(t *testing.T) {
    // Setup
    router := SetupRouter()

    // Test
    w := httptest.NewRecorder()
    body := `{"username":"test","user_code":"EMP999","password":"P@ssw0rd123"}`
    req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
    router.ServeHTTP(w, req)

    // Assert
    assert.Equal(t, 201, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.Equal(t, float64(0), response["code"])
}
```

### 7.3 覆盖率目标

- Service层：> 90%
- Handler层：> 80%
- Repository层：> 70%

---

## 八、性能优化

### 8.1 查询优化

```go
// 避免N+1查询
func (r *memberRepository) GetByOrgID(orgID string) ([]OrganizationMember, error) {
    var members []OrganizationMember
    // 预加载User信息
    err := r.db.Where("organization_id = ?", orgID).
        Preload("User", "id,username,user_code").
        Find(&members).Error
    return members, err
}
```

### 8.2 缓存策略

```go
// 组织成员列表缓存（使用物化视图，无需Redis）
func (s *OrganizationService) ListMembers(orgID string) ([]MemberVO, error) {
    // 直接查询物化视图，性能优异
    // 物化视图会自动刷新（5分钟间隔）
    var members []MemberVO

    err := db.Raw(`
        SELECT
            m.user_id,
            u.username,
            u.user_code,
            m.role,
            m.joined_at
        FROM organization_members m
        JOIN users u ON m.user_id = u.id
        WHERE m.organization_id = ?
        ORDER BY m.joined_at DESC
    `, orgID).Scan(&members).Error

    return members, err
}
```

**缓存策略说明：**
- **物化视图**：权限数据通过物化视图自动缓存
- **索引优化**：复合索引确保查询性能
- **无需Redis**：PostgreSQL自身性能已足够
- **定期刷新**：5分钟自动更新物化视图

### 8.3 连接池配置

```go
sqlDB, _ := db.DB()
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(50)
sqlDB.SetConnMaxLifetime(time.Hour)
```

---

## 九、安全考虑

### 9.1 密码安全
- ✅ 使用bcrypt慢哈希（cost=12）
- ✅ 密码最小长度8位
- ✅ 强制复杂度要求（大小写+数字+特殊字符）

### 9.2 Token安全
- ✅ HTTPS传输
- ✅ 短期Access Token（1小时）
- ✅ 长期Refresh Token（7天）
- ✅ 登出加入黑名单
- ✅ Token刷新机制

### 9.3 SQL注入防护
- ✅ 使用GORM参数化查询
- ✅ 禁止原生SQL拼接

### 9.4 权限控制
- ✅ 中间件校验
- ✅ 最小权限原则
- ✅ 不能移除owner
- ✅ 不能直接设置owner角色

---

## 十、开发里程碑

### Week 1: 基础架构
- [ ] 项目结构初始化
- [ ] 数据库模型定义
- [ ] GORM迁移脚本
- [ ] JWT工具类
- [ ] 统一响应格式

### Week 2: 用户认证
- [ ] 用户注册API + 测试
- [ ] 用户登录API + 测试
- [ ] Token刷新API
- [ ] 个人信息管理
- [ ] 密码修改

### Week 3: 组织管理
- [ ] 组织创建/查询/更新/删除
- [ ] 成员列表/添加/移除
- [ ] 角色修改
- [ ] 退出组织

### Week 4: 权限与优化
- [ ] 权限中间件
- [ ] PostgreSQL物化视图优化（替代Redis）
- [ ] 集成测试
- [ ] 性能测试

---

## 十一、依赖清单

```go
// go.mod
module cornerstone

go 1.21

require (
    // Web框架
    github.com/gin-gonic/gin v1.9.1

    // 数据库
    gorm.io/gorm v1.25.5
    gorm.io/driver/postgres v1.5.4

    // JWT
    github.com/golang-jwt/jwt/v5 v5.1.0

    // 密码哈希
    golang.org/x/crypto v0.16.0

    // UUID
    github.com/google/uuid v1.5.0

    // 配置
    github.com/spf13/viper v1.17.0

    // 测试
    github.com/stretchr/testify v1.8.4
)
```

---

## 十二、环境变量配置（12-Factor App）

### 环境变量列表
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
DATABASE_SSL_MODE=disable
DATABASE_MAX_IDLE_CONNS=10
DATABASE_MAX_OPEN_CONNS=50

# JWT配置
JWT_SECRET=your-secret-key-change-this-in-production
JWT_ACCESS_EXPIRE=3600      # 1小时
JWT_REFRESH_EXPIRE=604800   # 7天

# 安全配置
BCRYPT_COST=12
PASSWORD_MIN_LENGTH=8

# 日志配置
LOG_PATH=./logs/app.log
LOG_LEVEL=info

# 插件配置
PLUGIN_TIMEOUT=5

# 文件上传配置
UPLOAD_MAX_SIZE=100MB
STORAGE_TYPE=local  # local | minio
```

### .env 文件示例
```bash
# backend/.env
PORT=8080
MODE=debug

DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=cornerstone

JWT_SECRET=your-secret-key-change-this-in-production
JWT_ACCESS_EXPIRE=3600
JWT_REFRESH_EXPIRE=604800

BCRYPT_COST=12
PLUGIN_TIMEOUT=5
UPLOAD_MAX_SIZE=100MB
```

**注意**：`.env` 文件不应提交到版本控制系统（已在 `.gitignore` 中排除）。

---

## 十三、部署建议

### Docker Compose
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: cornerstone
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      # 服务器
      - PORT=8080
      - MODE=release

      # 数据库（使用service名）
      - DATABASE_HOST=postgres
      - DATABASE_PORT=5432
      - DATABASE_USER=postgres
      - DATABASE_PASSWORD=${DB_PASSWORD}
      - DATABASE_NAME=cornerstone

      # JWT
      - JWT_SECRET=${JWT_SECRET}
      - JWT_ACCESS_EXPIRE=3600
      - JWT_REFRESH_EXPIRE=604800

      # 安全
      - BCRYPT_COST=12
      - PLUGIN_TIMEOUT=5
      - UPLOAD_MAX_SIZE=100MB
    depends_on:
      - postgres

volumes:
  postgres_data:
```

**环境变量文件 (.env)：**
```bash
# 生产环境
DB_PASSWORD=your-secure-db-password
JWT_SECRET=your-super-secret-jwt-key-change-this
```

---

## 十四、后续扩展

### Sprint 2 可扩展功能
1. **组织邀请邮件**
2. **组织仪表盘**
3. **操作日志**
4. **API限流**
5. **多因素认证（MFA）**
6. **组织转移所有权**

---

**文档版本**: v1.0
**最后更新**: 2026-01-05
**状态**: 📋 待评审

---

## 评审要点

请检查以下内容：

1. ✅ **数据模型**：字段设计是否合理？索引是否充分？
2. ✅ **API设计**：接口命名、参数、响应是否符合RESTful规范？
3. ✅ **权限模型**：owner/admin/member角色权限是否清晰？
4. ✅ **安全考虑**：密码、Token、SQL注入防护是否到位？
5. ✅ **性能优化**：缓存、索引、查询优化是否合理？
6. ✅ **错误处理**：错误码定义是否完整？
7. ✅ **测试策略**：单元测试和集成测试是否覆盖核心场景？

如有需要调整的地方，请告诉我！