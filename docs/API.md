# Cornerstone API 文档

**版本**: v1.7 | **最后更新**: 2026-01-11 | **基础地址**: `http://localhost:8080`

---

## 概述

Cornerstone 提供完整的 RESTful API 接口，支持用户认证、组织管理、数据库管理、表/字段/记录的完整 CRUD 操作。

**技术栈**: Go + Gin + GORM + PostgreSQL

---

## 快速开始

### 环境要求
- Go 1.21+
- PostgreSQL 15+ (推荐) 或 SQLite 3.35+
- Node.js 18+ (前端开发)

### 启动后端服务
```bash
cd backend
go run ./cmd/server/main.go
```

服务将运行在 `http://localhost:8080`

---

## 认证机制

### JWT 认证
- **Token 格式**: `Bearer <JWT_TOKEN>`
- **过期时间**: 24 小时
- **Claims**: `user_id`, `username`, `role`

### 认证头格式
```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

---

## API 端点

### 基础端点

#### 健康检查
```http
GET /health
```

**响应** (200 OK):
```json
{
  "status": "healthy",
  "service": "cornerstone-backend",
  "time": "2026-01-10T19:00:00Z"
}
```

---

### 认证模块

#### 注册
```http
POST /api/auth/register
```

**请求体**:
```json
{
  "username": "字符串 (3-50字符, 字母数字下划线连字符)",
  "email": "字符串 (有效邮箱格式)",
  "password": "字符串 (6-50字符, 必须包含字母和数字)"
}
```

**响应** (201 Created):
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "usr_20260110190000_123456",
      "username": "john_doe",
      "email": "john@example.com",
      "created_at": "2026-01-10T19:00:00Z"
    }
  }
}
```

#### 登录
```http
POST /api/auth/login
```

**请求体**:
```json
{
  "username": "字符串 (用户名或邮箱)",
  "password": "字符串"
}
```

**响应** (200 OK):
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "usr_20260110190000_123456",
      "username": "john_doe",
      "email": "john@example.com"
    }
  }
}
```

#### 登出
```http
POST /api/auth/logout
```

**认证**: 必需

**响应** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "登出成功"
  }
}
```

#### 获取用户信息
```http
GET /api/users/me
```

**认证**: 必需

---

### 组织管理

#### 创建组织
```http
POST /api/organizations
```

**认证**: 必需

**请求体**:
```json
{
  "name": "字符串 (2-100字符)",
  "description": "字符串 (最多500字符, 可选)"
}
```

**响应** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "org_20260110190000_123456",
    "name": "Engineering Team",
    "description": "Hardware engineering department",
    "owner_id": "usr_20260110190000_123456",
    "created_at": "2026-01-10T19:00:00Z"
  }
}
```

#### 列出组织
```http
GET /api/organizations
```

**认证**: 必需

#### 获取组织详情
```http
GET /api/organizations/:id
```

**认证**: 必需 (用户必须是组织成员)

#### 更新组织
```http
PUT /api/organizations/:id
```

**认证**: 必需 (所有者或管理员)

#### 删除组织
```http
DELETE /api/organizations/:id
```

**认证**: 必需 (所有者)

#### 组织成员管理
```http
GET /api/organizations/:id/members
POST /api/organizations/:id/members
DELETE /api/organizations/:id/members/:member_id
PUT /api/organizations/:id/members/:member_id/role
```

---

### 数据库管理

#### 创建数据库
```http
POST /api/databases
```

**认证**: 必需

**请求体**:
```json
{
  "name": "字符串 (2-255字符)",
  "description": "字符串 (最多500字符, 可选)",
  "is_public": false,
  "is_personal": true
}
```

**响应** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "db_20260110190000_123456",
    "name": "Project Database",
    "description": "Project data storage",
    "owner_id": "usr_20260110190000_123456",
    "is_public": false,
    "is_personal": true,
    "created_at": "2026-01-10T19:00:00Z"
  }
}
```

#### 数据库操作
```http
GET /api/databases
GET /api/databases/:id
PUT /api/databases/:id
DELETE /api/databases/:id
POST /api/databases/:id/share
GET /api/databases/:id/users
DELETE /api/databases/:id/users/:user_id
PUT /api/databases/:id/users/:user_id/role
```

---

### 表管理

#### 创建表
```http
POST /api/tables
```

**认证**: 必需 (数据库所有者、管理员或编辑者)

**请求体**:
```json
{
  "database_id": "db_20260110190000_123456",
  "name": "字符串 (2-255字符)",
  "description": "字符串 (最多500字符, 可选)"
}
```

**响应** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "tbl_20260110190000_123456",
    "database_id": "db_20260110190000_123456",
    "name": "Customer Table",
    "description": "Customer information",
    "created_at": "2026-01-10T19:00:00Z"
  }
}
```

#### 表操作
```http
GET /api/databases/:id/tables
GET /api/tables/:id
PUT /api/tables/:id
DELETE /api/tables/:id
```

---

### 字段管理

#### 创建字段
```http
POST /api/fields
```

**认证**: 必需 (数据库所有者、管理员或编辑者)

**请求体**:
```json
{
  "table_id": "tbl_20260110190000_123456",
  "name": "字符串 (1-255字符)",
  "type": "string | number | boolean | date | datetime | single_select | multi_select",
  "required": false,
  "config": {
    "max_length": 255,
    "validation": "regex pattern"
  }
}
```

**字段类型说明**:
- `string`: 文本
- `number`: 数字
- `boolean`: 真/假
- `date`: 日期字符串 (YYYY-MM-DD)
- `datetime`: 日期时间字符串 (ISO 8601)
- `single_select`: 从选项中单选
- `multi_select`: 从选项中多选

**响应** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "fld_20260110190000_123456",
    "table_id": "tbl_20260110190000_123456",
    "name": "customer_name",
    "type": "string",
    "required": true,
    "created_at": "2026-01-10T19:00:00Z"
  }
}
```

#### 字段操作
```http
GET /api/tables/:id/fields
GET /api/fields/:id
PUT /api/fields/:id
DELETE /api/fields/:id
```

---

### 字段权限管理 ⭐ 新增

#### 获取表字段权限配置
```http
GET /api/tables/:tableId/field-permissions
```

**认证**: 必需 (需要表访问权限)

**响应** (200 OK):
```json
{
  "success": true,
  "data": {
    "permissions": [
      {
        "id": "flp_20260111190000_123456",
        "table_id": "tbl_20260110190000_123456",
        "field_id": "fld_20260110190000_123456",
        "role": "editor",
        "can_read": true,
        "can_write": true,
        "can_delete": false
      }
    ]
  }
}
```

#### 设置字段权限
```http
PUT /api/tables/:tableId/field-permissions
```

**认证**: 必需 (数据库所有者或管理员)

**请求体**:
```json
{
  "field_id": "fld_20260110190000_123456",
  "role": "editor",
  "can_read": true,
  "can_write": true,
  "can_delete": false
}
```

**角色权限限制**:
- `owner` / `admin`: 始终拥有全部权限 (R/W/D)，不可修改
- `editor`: 可配置 R/W，不能配置 D
- `viewer`: 只能配置 R

#### 批量设置字段权限
```http
PUT /api/tables/:tableId/field-permissions/batch
```

**请求体**:
```json
{
  "permissions": [
    {
      "field_id": "fld_20260110190000_123456",
      "role": "editor",
      "can_read": true,
      "can_write": true,
      "can_delete": false
    }
  ]
}
```

---

### 记录管理

#### 创建记录
```http
POST /api/records
```

**认证**: 必需 (数据库所有者、管理员或编辑者)

**请求体**:
```json
{
  "table_id": "tbl_20260110190000_123456",
  "data": {
    "customer_name": "张三",
    "age": 25,
    "is_active": true
  }
}
```

**响应** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "rec_20260110190000_123456",
    "table_id": "tbl_20260110190000_123456",
    "data": {
      "customer_name": "张三",
      "age": 25,
      "is_active": true
    },
    "version": 1,
    "created_at": "2026-01-10T19:00:00Z"
  }
}
```

#### 列出记录
```http
GET /api/records
```

**认证**: 必需

**查询参数**:
- `table_id` (必需): 要查询的表ID
- `limit` (可选, 默认: 20, 最大: 100): 分页限制
- `offset` (可选, 默认: 0): 分页偏移
- `filter` (可选): JSON字符串用于过滤

**示例**: `GET /api/records?table_id=tbl_xxx&limit=20&offset=0&filter={"age":25}`

#### 记录操作
```http
GET /api/records/:id
PUT /api/records/:id
DELETE /api/records/:id
```

#### 批量创建记录
```http
POST /api/records/batch
```

**查询参数**:
- `count` (可选, 默认: 1, 最大: 100): 要创建的记录数量

---

## 响应格式

### 成功响应
```json
{
  "success": true,
  "data": {
    // 响应数据
  }
}
```

### 错误响应
```json
{
  "success": false,
  "message": "错误描述",
  "code": 400
}
```

### HTTP 状态码
| 状态码 | 说明 |
|--------|------|
| `200 OK` | 成功 |
| `201 Created` | 资源创建成功 |
| `400 Bad Request` | 无效输入/验证错误 |
| `401 Unauthorized` | 缺少或无效的身份验证 |
| `403 Forbidden` | 权限不足 |
| `404 Not Found` | 资源未找到 |
| `500 Internal Server Error` | 服务器错误 |

---

## 数据模型

### 核心表结构 (共14张)

| 表名 | 前缀 | 说明 |
|------|------|------|
| users | `usr_` | 用户表 |
| organizations | `org_` | 组织表 |
| organization_members | `mem_` | 组织成员 |
| databases | `db_` | 数据库 |
| database_access | `acc_` | 数据库权限 |
| tables | `tbl_` | 表定义 |
| fields | `fld_` | 字段定义 |
| field_permissions | `flp_` | 字段权限 ⭐ 新增 |
| records | `rec_` | 数据记录 |
| files | `fil_` | 文件附件 |
| plugins | `plg_` | 插件定义 |
| plugin_bindings | `pbd_` | 插件绑定 |
| token_blacklist | - | JWT黑名单 |
| user_database_permissions | - | 物化视图 |

### ID 格式
所有 ID 使用 UUID 风格格式带前缀：`usr_001`, `db_001`, `tbl_001`, `fld_001`, `rec_001`

---

## 权限模型

### 三层权限架构
```
L1: 数据库级权限 (owner/admin/editor/viewer)
L2: 表级权限 (继承自数据库)
L3: 字段级权限 (owner/admin/editor/viewer + R/W/D)
```

### 权限优先级
```
字段级权限配置 > 表级默认权限 > 数据库级权限
```

### 角色说明

#### 数据库级角色
| 角色 | 权限 |
|------|------|
| owner | 完全控制 |
| admin | 编辑权限 + 用户管理 |
| editor | 编辑数据 |
| viewer | 只读 |

#### 字段级权限
| 角色 | 可配置权限 |
|------|-----------|
| owner / admin | 始终拥有全部权限 (R/W/D)，不可修改 |
| editor | 可配置 R/W，不能配置 D |
| viewer | 只能配置 R |

---

## 配置

### 环境变量
```bash
# 数据库
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=disable
DB_MAX_OPEN=10
DB_MAX_IDLE=5
DB_MAX_LIFETIME=3600

# 服务器
SERVER_MODE=debug  # debug | release
PORT=8080

# JWT
JWT_SECRET=your-secret-key-here
JWT_EXPIRATION=24  # 小时

# 日志
LOG_LEVEL=info
LOG_OUTPUT=logs/app.log
LOG_ERROR=logs/error.log
LOG_MAX_SIZE=100
LOG_MAX_AGE=7
LOG_MAX_BACKUPS=5
```

### 默认值
- 数据库: PostgreSQL
- 端口: 8080
- JWT 过期时间: 24小时
- 日志级别: info

---

## 使用示例

### curl 示例

#### 注册
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"Test123456"}'
```

#### 登录
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"Test123456"}'
```

#### 使用 Token 认证请求
```bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
curl -X GET http://localhost:8080/api/organizations \
  -H "Authorization: Bearer $TOKEN"
```

### Go 客户端
```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
)

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

func main() {
    reqBody := LoginRequest{
        Username: "testuser",
        Password: "Test123456",
    }
    jsonBody, _ := json.Marshal(reqBody)
    resp, _ := http.Post(
        "http://localhost:8080/api/auth/login",
        "application/json",
        bytes.NewBuffer(jsonBody),
    )
    defer resp.Body.Close()
    // 处理响应...
}
```

### JavaScript 客户端
```typescript
const login = async (credentials: { username: string; password: string }) => {
  const response = await fetch('http://localhost:8080/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(credentials),
  });
  return response.json();
};
```

### Python 客户端
```python
import requests

response = requests.post(
    'http://localhost:8080/api/auth/login',
    json={'username': 'testuser', 'password': 'Test123456'}
)
data = response.json()
token = data['data']['token']

# 使用 token
headers = {'Authorization': f'Bearer {token}'}
response = requests.get('http://localhost:8080/api/organizations', headers=headers)
```

---

## 重要说明

1. **时间戳格式**: 所有时间戳均为 ISO 8601 格式 (UTC)
2. **软删除**: 所有删除操作均为软删除 (GORM DeletedAt)
3. **中文字符**: 完全支持 Unicode/中文字符
4. **JSONB 存储**: 记录数据使用 JSONB 存储实现动态字段
5. **乐观锁**: 更新记录时支持版本号并发控制

---

## 安全特性

- **认证**: JWT tokens 使用 HS256 签名
- **授权**: 基于角色的访问控制 (RBAC)
- **输入验证**: 所有端点进行服务器端验证
- **密码安全**: 使用 bcrypt 哈希 (cost: 10)
- **并发保护**: 乐观锁防止数据冲突

---

## 相关文档

- [开发指南](./DEVELOPER-GUIDE.md) - 完整开发指南
- [测试报告](./E2E-TEST-REPORT.md) - E2E 测试报告
- [项目状态](./PROJECT-STATUS.md) - 项目进度状态
