# Cornerstone P0 任务测试指南

## 📋 已完成的 P0 任务

### ✅ 后端实现
1. **用户认证系统** (`internal/services/auth.go`)
   - 用户注册（用户名+邮箱唯一性检查）
   - 用户登录（支持用户名或邮箱登录）
   - 密码哈希（bcrypt）
   - JWT Token 生成与验证
   - JWT 黑名单（登出功能）

2. **组织管理系统** (`internal/services/organization.go`)
   - 组织 CRUD（创建、查询、更新、删除）
   - 组织成员管理（添加、移除、角色更新）
   - 权限控制（owner、admin、member）
   - 成员列表查询

3. **API 路由** (`cmd/server/main.go`)
   - `/api/auth/register` - 用户注册
   - `/api/auth/login` - 用户登录
   - `/api/auth/logout` - 用户登出
   - `/api/users/me` - 获取当前用户信息
   - `/api/organizations` - 组织管理（CRUD）
   - `/api/organizations/:id/members` - 组织成员管理

4. **数据库模型** (`internal/models/models.go`)
   - 13 张表定义完整
   - UUID 主键生成（usr_, org_, mem_, db_ 等前缀）
   - 软删除支持
   - 自动时间戳

## 🚀 快速启动测试

### 前置条件
1. 安装 Go 1.25.4+
2. 安装 PostgreSQL 15+
3. 创建数据库：
   ```sql
   CREATE DATABASE cornerstone;
   ```

### 步骤 1: 配置环境变量

```bash
cd backend
cp .env.example .env
# 编辑 .env 文件，设置数据库连接和 JWT_SECRET
```

**关键配置**：
```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/cornerstone?sslmode=disable
JWT_SECRET=your-secret-key-change-this-in-production-12345
PORT=8080
```

### 步骤 2: 启动后端服务

```bash
cd backend
# 首次运行会自动创建表结构
go run ./cmd/server/main.go
```

**预期输出**：
```
INFO Starting Cornerstone server...
INFO Server starting on :8080
INFO Database connected successfully
INFO Database migration completed
```

### 步骤 3: 运行测试脚本

在新的终端窗口中：

**Windows (使用 Git Bash 或 WSL)**:
```bash
cd backend
bash test-auth.sh
```

**或手动测试（使用 PowerShell）**:

1. **健康检查**:
```powershell
Invoke-RestMethod -Uri http://localhost:8080/health
```

2. **用户注册**:
```powershell
$body = @{
    username = "testuser"
    email = "test@example.com"
    password = "password123"
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8080/api/auth/register `
    -Method POST `
    -ContentType "application/json" `
    -Body $body
```

3. **用户登录**:
```powershell
$body = @{
    username = "testuser"
    password = "password123"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri http://localhost:8080/api/auth/login `
    -Method POST `
    -ContentType "application/json" `
    -Body $body

$token = $response.data.token
Write-Host "Token: $($token.Substring(0,50))..."
```

4. **获取用户信息**:
```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/users/me `
    -Headers @{ Authorization = "Bearer $token" }
```

5. **创建组织**:
```powershell
$body = @{
    name = "测试组织"
    description = "这是一个测试组织"
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8080/api/organizations `
    -Method POST `
    -Headers @{ Authorization = "Bearer $token" } `
    -ContentType "application/json" `
    -Body $body
```

6. **获取组织列表**:
```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/organizations `
    -Headers @{ Authorization = "Bearer $token" }
```

7. **用户登出**:
```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/auth/logout `
    -Method POST `
    -Headers @{ Authorization = "Bearer $token" }
```

## 🔍 验证数据库

使用 PostgreSQL 客户端查看表结构：

```sql
-- 查看所有表
\dt

-- 查看用户表
SELECT id, username, email, created_at FROM users;

-- 查看组织表
SELECT id, name, owner_id, created_at FROM organizations;

-- 查看组织成员
SELECT * FROM organization_members;

-- 查看 JWT 黑名单
SELECT * FROM token_blacklist;
```

## ✅ 预期测试结果

### 1. 注册测试
```json
{
  "success": true,
  "message": "操作成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": "usr_20250109...",
      "username": "testuser",
      "email": "test@example.com",
      "created_at": "2025-01-09T..."
    }
  }
}
```

### 2. 登录测试
```json
{
  "success": true,
  "message": "操作成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": "usr_20250109...",
      "username": "testuser",
      "email": "test@example.com"
    }
  }
}
```

### 3. 组织创建测试
```json
{
  "success": true,
  "message": "操作成功",
  "data": {
    "id": "org_20250109...",
    "name": "测试组织",
    "description": "这是一个测试组织",
    "owner_id": "usr_20250109...",
    "created_at": "2025-01-09T..."
  }
}
```

## 📊 当前系统状态

### 已实现功能 ✅
- [x] 用户注册（用户名+邮箱唯一性检查）
- [x] 用户登录（支持用户名或邮箱）
- [x] JWT Token 认证
- [x] Token 黑名单（登出）
- [x] 组织 CRUD
- [x] 组织成员管理
- [x] 权限中间件
- [x] 密码加密（bcrypt）
- [x] 数据库迁移（13张表）
- [x] 统一响应格式
- [x] 错误处理

### 待实现功能 ⏳
- [ ] 数据库管理 API
- [ ] 表/字段管理 API
- [ ] 数据记录 CRUD
- [ ] 文件上传/下载
- [ ] 插件系统
- [ ] 前端与真实 API 对接

## 🐛 常见问题

### 1. 数据库连接失败
```
Error: failed to connect to database
```
**解决方案**：检查 PostgreSQL 是否运行，检查 DATABASE_URL 是否正确

### 2. JWT_SECRET 验证失败
```
Error: 配置验证失败: JWT_SECRET 必须设置且不能使用默认值
```
**解决方案**：在 .env 文件中设置一个强密码作为 JWT_SECRET

### 3. 端口已被占用
```
Error: bind: address already in use
```
**解决方案**：修改 .env 中的 PORT 配置，或关闭占用 8080 端口的程序

### 4. Token 认证失败
```
401 Unauthorized
```
**解决方案**：
- 确认 Token 格式：`Authorization: Bearer <token>`
- 检查 Token 是否过期
- 验证 JWT_SECRET 是否与登录时一致

## 📝 下一步计划

### P1 任务（重要）
1. 实现数据库管理 API
2. 实现表/字段管理 API
3. 前端对接真实 API
4. 添加输入验证增强

### P2 任务（优化）
1. 添加单元测试
2. 性能基准测试
3. API 文档更新
4. Docker 配置

## 🎯 测试检查清单

- [ ] 用户能够成功注册
- [ ] 用户能够成功登录
- [ ] 注册重复用户名时返回错误
- [ ] 注册重复邮箱时返回错误
- [ ] 错误密码无法登录
- [ ] JWT Token 能够正确认证
- [ ] 用户信息获取正确
- [ ] 用户能够创建组织
- [ ] 组织列表正确显示
- [ ] 组织所有者能够添加成员
- [ ] 组织管理员能够添加成员
- [ ] 普通成员无法添加成员
- [ ] Token 登出后无法使用
- [ ] 数据库表正确创建

---

**文档生成时间**: 2025-01-09
**后端版本**: v1.0.0-p0
**状态**: P0 任务已完成，可开始测试
