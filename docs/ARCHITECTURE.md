# 硬件工程数据管理平台 - 技术架构设计

**版本**：1.0
**日期**：2024年01月03日
**状态**：设计完成

---

## 1. 系统架构概览

### 1.1 整体架构图 (PlantUML)

```plantuml
@startuml
!theme plain
skinparam componentStyle rectangle
skinparam roundcorner 5
skinparam defaultTextAlignment center

package "用户层 (User Layer)" {
    [Web Browser] as browser
    [Mobile App] as mobile
}

package "网关层 (Gateway Layer)" {
    component [Nginx / API Gateway] as gateway
}

package "应用层 (Application Layer)" {
    component [前端服务\n(React)] as frontend
    component [后端API服务\n(FastAPI)] as backend
    component [插件服务\n(Python子进程)] as plugin
}

package "数据层 (Data Layer)" {
    database [PostgreSQL\n主数据库] as postgres
    component [文件系统\n/ MinIO] as storage
}

browser --> gateway : HTTP/HTTPS
mobile --> gateway : HTTP/HTTPS

gateway --> frontend : 静态资源
gateway --> backend : API请求

frontend --> backend : RESTful API

backend --> postgres : CRUD
backend --> plugin : 子进程调用
backend --> storage : 文件上传/下载

plugin --> postgres : 数据读写
plugin --> storage : 文件操作

@enduml
```

### 1.2 插件执行流程图

```plantuml
@startuml
!theme plain
skinparam activity {
  BackgroundColor<<Success>> LightGreen
  BackgroundColor<<Error>> LightCoral
}

start
:主进程接收请求\n(记录保存/删除);
:权限校验;
if (权限通过?) then (是)
  :查询启用插件列表;
  if (有插件?) then (是)
    :构造Payload;
    :启动子进程;
    :发送JSON到stdin;

    fork
      :子进程执行插件;
    fork again
      :主进程监控超时\n(5秒);
    end fork

    if (执行成功?) then (是)
      :接收stdout结果;
      :更新数据;
      <<Success>>
    else (否)
      :捕获异常/超时;
      :记录错误日志;
      :阻止操作;
      <<Error>>
    endif
  else (否)
    :直接保存数据;
  endif
else (否)
  :返回403无权访问;
  <<Error>>
endif

stop
@enduml
```

### 1.3 数据库ER关系图

```plantuml
@startuml
!theme plain
hide circle

' 用户与组织
entity Users {
  * id : VARCHAR(50)
  --
  username : VARCHAR(50)
  user_code : VARCHAR(50)
  password_hash : VARCHAR(255)
  role : VARCHAR(20)
}

entity Organizations {
  * id : VARCHAR(50)
  --
  name : VARCHAR(255)
  owner_id : VARCHAR(50)
}

entity Organization_Members {
  * id : VARCHAR(50)
  --
  organization_id : VARCHAR(50)
  user_id : VARCHAR(50)
  role : VARCHAR(20)
}

' 数据库与权限
entity Databases {
  * id : VARCHAR(50)
  --
  db_name : VARCHAR(255)
  owner_id : VARCHAR(50)
  organization_id : VARCHAR(50)
  visibility : VARCHAR(20)
}

entity Database_Access {
  * id : VARCHAR(50)
  --
  database_id : VARCHAR(50)
  user_id : VARCHAR(50)
  role : VARCHAR(20)
  source : VARCHAR(20)
}

' 数据结构
entity Tables {
  * id : VARCHAR(50)
  --
  database_id : VARCHAR(50)
  name : VARCHAR(255)
}

entity Fields {
  * id : VARCHAR(50)
  --
  table_id : VARCHAR(50)
  name : VARCHAR(255)
  type : VARCHAR(50)
  config : JSONB
}

entity Records {
  * id : VARCHAR(50)
  --
  table_id : VARCHAR(50)
  data : JSONB
  version : INTEGER
}

entity Files {
  * id : VARCHAR(50)
  --
  record_id : VARCHAR(50)
  original_name : VARCHAR(500)
  stored_name : VARCHAR(255)
}

' 插件系统
entity Plugins {
  * id : VARCHAR(50)
  --
  name : VARCHAR(255)
  database_id : VARCHAR(50)
  event_type : VARCHAR(100)
  is_enabled : BOOLEAN
}

entity Plugin_Logs {
  * id : SERIAL
  --
  plugin_id : VARCHAR(50)
  status : VARCHAR(20)
  execution_time_ms : INTEGER
}

entity Edit_Locks {
  * record_id : VARCHAR(50)
  --
  user_id : VARCHAR(50)
  locked_at : TIMESTAMP
}

' 关系定义
Users ||--o{ Organization_Members : "加入"
Organizations ||--o{ Organization_Members : "包含"
Users ||--o{ Organizations : "创建"

Organizations ||--o{ Databases : "拥有"
Users ||--o{ Databases : "创建"

Databases ||--o{ Database_Access : "授权"
Users ||--o{ Database_Access : "被授权"

Databases ||--o{ Tables : "包含"
Users ||--o{ Tables : "创建"

Tables ||--o{ Fields : "定义"
Tables ||--o{ Records : "包含"
Users ||--o{ Records : "创建"

Records ||--o{ Files : "关联"
Users ||--o{ Files : "上传"

Databases ||--o{ Plugins : "绑定"
Users ||--o{ Plugins : "创建"

Plugins ||--o{ Plugin_Logs : "产生"
Records ||--o{ Edit_Locks : "锁定"
Users ||--o{ Edit_Locks : "持有"

@enduml
```

### 1.4 部署架构图

```plantuml
@startuml
!theme plain
skinparam componentStyle rectangle

node "Web Server" {
    component [Nginx] as nginx
}

node "Application Server" {
    component [Frontend\nVue 3 + Vite] as frontend
    component [Backend\nGo (Gin/Fiber)] as backend
    component [Plugin Runner\nGo子进程] as plugin
}

node "Data" {
    database [PostgreSQL\nPort: 5432] as postgres
}

node "Storage" {
    component [MinIO\nPort: 9000] as minio
}

node "Monitoring" {
    component [Prometheus] as prometheus
    component [Grafana] as grafana
}

' 外部用户
actor [User] as user

user --> nginx : HTTP/HTTPS
nginx --> frontend : /
nginx --> backend : /api/*

frontend --> backend : API调用
backend --> postgres : 数据持久化
backend --> minio : 文件存储
backend --> plugin : 子进程调用

backend --> prometheus : 指标暴露
prometheus --> grafana : 数据展示

@enduml
```

### 1.5 时序图 - 记录保存流程

```plantuml
@startuml
!theme plain
autonumber

actor 用户 as user
participant "前端" as fe
participant "后端API" as be
participant "权限校验" as auth
participant "插件系统" as plugin
participant "数据库" as db
participant "文件存储" as storage

user -> fe: 填写表单并提交
fe -> be: POST /api/records\n{data, table_id}

be -> auth: check_permission(user, db, "editor")
auth --> be: 权限通过

be -> plugin: 触发 before_record_save
plugin -> plugin: 子进程执行
plugin -> db: 读取配置
plugin --> be: 返回修改后数据

be -> db: BEGIN TRANSACTION
be -> db: INSERT/UPDATE records
be -> db: UPDATE version

alt 文件上传
    be -> storage: 上传文件
    storage --> be: 返回文件信息
    be -> db: 更新文件关联
end

be -> db: COMMIT
be -> plugin: 触发 after_record_save
plugin -> plugin: 记录日志
plugin --> be: 完成

be --> fe: 200 OK
fe --> user: 显示成功消息

@enduml
```

### 1.6 权限继承流程图

```plantuml
@startuml
!theme plain
skinparam decision {
  BackgroundColor LightBlue
  BorderColor DarkBlue
}

start
:用户请求访问数据库;

if (系统管理员?) then (是)
  :授予访问权限;
  stop
endif

if (数据库所有者?) then (是)
  :授予owner权限;
  stop
endif

if (直接授权?) then (是)
  :授予指定角色;
  stop
endif

if (组织数据库?) then (是)
  :查询组织角色;

  if (组织所有者?) then (是)
    :授予owner权限;
    stop
  endif

  if (组织管理员?) then (是)
    :授予editor权限;
    stop
  endif

  if (组织成员?) then (是)
    :需要database_access配置;
    if (有配置?) then (是)
      :授予配置角色;
      stop
    else (否)
      :拒绝访问;
      stop
    endif
  endif
endif

:拒绝访问;
stop

@enduml
```

---

## 1.2 架构说明

### 1.2.1 分层设计

**用户层**：
- Web浏览器：主要使用场景，支持Chrome、Firefox、Edge
- 移动端：未来扩展，响应式设计

**网关层**：
- Nginx：反向代理、负载均衡、SSL终止
- API Gateway：请求路由、限流、认证

**应用层**：
- 前端服务：Vue 3 + TypeScript + Vite，提供类Excel界面
- 后端服务：Go (Gin/Fiber)，提供高性能RESTful API
- 插件服务：Go子进程，隔离执行用户代码

**数据层**：
- PostgreSQL：主数据库，存储元数据和业务数据
- 文件系统/MinIO：附件存储

### 1.2.2 插件隔离机制

插件通过子进程方式运行，实现完全隔离：
- **进程隔离**：每个插件独立进程
- **通信机制**：stdin/stdout JSON通信
- **超时控制**：5秒强制终止
- **异常隔离**：插件崩溃不影响主进程
- **资源限制**：可配置CPU/内存限制
- **插件语言**：支持Go/Python（通过子进程调用）

### 1.2.3 权限模型

**双重数据库模式**：
1. **个人数据库**：用户创建，手动共享
2. **组织数据库**：组织创建，自动继承

**权限继承优先级**：
1. 系统管理员
2. 数据库所有者
3. 直接授权（database_access）
4. 组织继承（组织所有者/管理员）

### 1.2.4 性能优化

**数据库层面**：
- GIN索引：JSONB字段查询
- 复合索引：双向查询优化
- 物化视图：权限缓存

**应用层面**：
- 游标分页：避免offset性能问题
- 异步处理：插件执行不阻塞主流程

**文件层面**：
- 分片上传：大文件支持
- CDN加速：静态资源

---

## 2. 技术栈选择

### 2.1 前端技术栈

| 技术 | 选型 | 理由 |
|------|------|------|
| **框架** | Vue 3 + TypeScript | 渐进式框架，学习曲线平缓，适合快速开发 |
| **状态管理** | Pinia | Vue官方推荐，类型安全，轻量级 |
| **UI 组件库** | Element Plus | 企业级组件库，表单/表格完善，文档友好 |
| **网格组件** | Element Plus Table / AG Grid | 类Excel体验，支持虚拟滚动 |
| **文件上传** | Axios + 自定义上传 | 支持分片上传，进度显示 |
| **构建工具** | Vite | 快速冷启动，HMR 体验好 |
| **路由** | Vue Router | 官方路由，支持嵌套路由 |

### 2.2 后端技术栈

| 技术 | 选型 | 理由 |
|------|------|------|
| **语言** | Go 1.21 | 高性能，编译型语言，部署简单 |
| **框架** | Gin / Fiber | 高性能Web框架，轻量级，路由优秀 |
| **ORM** | GORM | 成熟的Go ORM，支持JSON/关联查询 |
| **认证** | JWT + bcrypt | 标准安全，无状态 |
| **数据库驱动** | pgx | PostgreSQL原生驱动，性能优秀 |
| **配置管理** | Viper | 配置文件管理，支持多种格式 |

### 2.3 数据存储

| 组件 | 选型 | 用途 |
|------|------|------|
| **主数据库** | PostgreSQL 15 | 结构化数据，JSONB支持 |
| **文件存储** | 本地文件系统 / MinIO | 附件存储，成本可控 |

---

## 3. 核心模块设计

### 3.1 认证与授权模块

```go
// 认证流程
1. 用户登录 → 验证用户名/密码 → 生成JWT Token
2. Token 包含：user_id, role, exp
3. 每次请求携带 Token → 验证并解析 → 获取用户信息

// 授权流程
1. 检查系统管理员权限
2. 检查数据库所有者
3. 检查直接授权 (database_access)
4. 检查组织继承权限 (organization_members)
5. 返回权限结果
```

**关键接口**：
```go
// 认证
POST /api/auth/login
POST /api/auth/register

// 权限检查（中间件）
func RequirePermission(role string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user_id := c.GetString("user_id")
        database_id := c.Param("database_id")

        if !checkPermission(user_id, database_id, role) {
            c.JSON(403, gin.H{"error": "无权访问"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### 3.2 数据建模模块

**动态表结构设计**：
```go
// 元数据存储
type Table struct {
    ID           string `gorm:"primaryKey"`
    DatabaseID   string
    Name         string
}

type Field struct {
    ID           string `gorm:"primaryKey"`
    TableID      string
    Name         string
    Type         string
    Config       datatypes.JSON  // GORM JSON类型
}

// 业务数据存储（JSONB）
type Record struct {
    ID           string `gorm:"primaryKey"`
    TableID      string
    Data         datatypes.JSON  // JSONB存储
    CreatedBy    string
    Version      int
}

// 示例数据 (JSON)
{
  "fld_001": "2024-01-15",
  "fld_002": 25.5,
  "fld_003": "正常"
}
```

**查询优化**：
```sql
-- GIN索引支持JSONB字段查询
CREATE INDEX idx_records_data_gin ON records USING gin (data);

-- 查询示例
SELECT * FROM records
WHERE table_id = 'tbl_001'
  AND data @> '{"fld_003": "正常"}';
```

### 3.3 插件系统模块

**执行架构**：
```
主进程 (Go/Gin)
    ↓ 触发事件
构造 payload
    ↓
启动子进程 (exec.Command)
    ↓
Go/Python 插件执行
    ├─ stdin: JSON payload
    ├─ stdout: JSON result
    └─ stderr: 错误信息
    ↓
超时控制 (5秒)
    ↓
记录日志
    ↓
返回结果
```

**插件示例（Go）**：
```go
// 插件代码 (user_plugin.go)
package main

import (
    "encoding/json"
    "os"
)

type Payload struct {
    Event string `json:"event"`
    Record Record `json:"record"`
}

type Record struct {
    Data map[string]interface{} `json:"data"`
}

func main() {
    // 从 stdin 读取 payload
    decoder := json.NewDecoder(os.Stdin)
    var payload Payload
    decoder.Decode(&payload)

    if payload.Event == "before_record_save" {
        // 自动计算字段
        data := payload.Record.Data
        if fld002, ok := data["fld_002"].(float64); ok {
            data["calculated_field"] = fld002 * 2
        }

        // 输出到 stdout
        encoder := json.NewEncoder(os.Stdout)
        encoder.Encode(payload)
    }
}
```

**插件示例（Python）**：
```python
# 插件代码 (user_plugin.py)
def plugin_entrypoint(event_type: str, payload: dict) -> dict:
    if event_type == "before_record_save":
        # 自动计算字段
        data = payload["record"]["data"]
        data["calculated_field"] = data["fld_002"] * 2
        return {"record": payload["record"]}
    return None
```

**安全机制**：
- 子进程沙箱隔离
- 5秒超时限制
- 异常捕获不影响主进程
- 所有执行记录落日志

### 3.4 文件管理模块

**存储路径设计**：
```
uploads/
  └── {database_id}/
      └── {table_id}/
          └── {record_id}/
              └── {stored_name}
                  - original_name: 用户上传时的文件名
                  - stored_name: 系统生成的唯一文件名 (UUID)
```

**上传流程**：
1. 前端选择文件
2. 请求上传地址（包含 record_id）
3. 分片上传（大文件支持）
4. 写入文件系统
5. 记录文件元数据到 `files` 表
6. 关联到 record 的 JSONB 字段

### 3.5 编辑锁模块

**乐观锁机制**：
```go
// 1. 用户A打开记录
GET /api/records/rec_001 → 返回 version=1

// 2. 用户A编辑，用户B也编辑
// 用户A提交：
PUT /api/records/rec_001
Body: {data: {...}, version: 1}
→ 检查 version=1 → 更新成功 → version=2

// 用户B提交（同时）：
PUT /api/records/rec_001
Body: {data: {...}, version: 1}
→ 检查 version=2 ≠ 1 → 返回冲突错误
```

**编辑锁（防止同时编辑）**：
```go
// 用户A打开编辑界面
POST /api/edit-locks
Body: {record_id: "rec_001"}
→ 写入 edit_locks 表

// 用户B尝试编辑
GET /api/edit-locks/rec_001
→ 返回：正在被用户A编辑

// 用户A关闭或超时
DELETE /api/edit-locks/rec_001
或 自动清理（10分钟超时）
```

---

## 4. 数据库设计

### 4.1 核心表结构

详见 `database-er-v3.0.md`

### 4.2 索引策略

```sql
-- 1. 用户表
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_user_code ON users(user_code);

-- 2. 组织成员表（复合索引）
CREATE INDEX idx_org_members_user_org ON organization_members(user_id, organization_id);

-- 3. 数据库表
CREATE INDEX idx_databases_owner_id ON databases(owner_id);
CREATE INDEX idx_databases_organization_id ON databases(organization_id);

-- 4. 权限表（双向查询）
CREATE INDEX idx_database_access_db_user ON database_access(database_id, user_id);
CREATE INDEX idx_database_access_user_db ON database_access(user_id, database_id);

-- 5. 记录表（核心查询）
CREATE INDEX idx_records_table_id ON records(table_id);
CREATE INDEX idx_records_created_at ON records(created_at DESC);
CREATE INDEX idx_records_data_gin ON records USING gin (data);

-- 6. 部分索引（优化）
CREATE INDEX idx_plugins_enabled ON plugins(is_enabled) WHERE is_enabled = true;
CREATE INDEX idx_edit_locks_recent ON edit_locks(locked_at)
    WHERE locked_at > NOW() - INTERVAL '10 minutes';
```

### 4.3 物化视图（权限缓存）

```sql
CREATE MATERIALIZED VIEW user_database_permissions AS
SELECT
    da.user_id,
    da.database_id,
    da.role,
    da.source,
    d.owner_id,
    d.organization_id,
    CASE
        WHEN da.user_id = d.owner_id THEN 'owner'
        ELSE da.role
    END AS effective_role
FROM database_access da
JOIN databases d ON da.database_id = d.id;

CREATE INDEX idx_user_db_perm ON user_database_permissions(user_id, database_id);

-- 定期刷新（建议每5分钟）
REFRESH MATERIALIZED VIEW CONCURRENTLY user_database_permissions;
```

---

## 5. API 接口设计

### 5.1 认证相关

```http
### 用户注册
POST /api/auth/register
Content-Type: application/json

{
  "username": "zhangsan",
  "user_code": "E001",
  "password": "SecurePass123"
}

### 用户登录
POST /api/auth/login
Content-Type: application/json

{
  "username": "zhangsan",
  "password": "SecurePass123"
}

Response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "usr_001",
    "username": "zhangsan",
    "role": "user"
  }
}
```

**Go Handler 示例**：
```go
// handler/auth.go
func Login(c *gin.Context) {
    var req struct {
        Username string `json:"username" binding:"required"`
        Password string `json:"password" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "参数错误"})
        return
    }

    // 查询用户
    var user models.User
    if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
        c.JSON(401, gin.H{"error": "用户名或密码错误"})
        return
    }

    // 验证密码
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        c.JSON(401, gin.H{"error": "用户名或密码错误"})
        return
    }

    // 生成JWT
    token, err := util.GenerateToken(user.ID, user.Role)
    if err != nil {
        c.JSON(500, gin.H{"error": "生成token失败"})
        return
    }

    c.JSON(200, gin.H{
        "token": token,
        "user": gin.H{
            "id":       user.ID,
            "username": user.Username,
            "role":     user.Role,
        },
    })
}
```

### 5.2 组织管理

```http
### 创建组织
POST /api/organizations
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "硬件研发部",
  "description": "负责硬件产品研发"
}

### 添加组织成员
POST /api/organizations/{org_id}/members
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_id": "usr_002",
  "role": "member"
}
```

**Go Handler 示例**：
```go
// handler/organization.go
func CreateOrganization(c *gin.Context) {
    userID := c.GetString("user_id")

    var req struct {
        Name        string `json:"name" binding:"required"`
        Description string `json:"description"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "参数错误"})
        return
    }

    org := models.Organization{
        ID:          util.NewUUID(),
        Name:        req.Name,
        OwnerID:     userID,
        Description: req.Description,
    }

    if err := db.Create(&org).Error; err != nil {
        c.JSON(500, gin.H{"error": "创建组织失败"})
        return
    }

    // 自动添加创建者为组织所有者
    member := models.OrganizationMember{
        ID:             util.NewUUID(),
        OrganizationID: org.ID,
        UserID:         userID,
        Role:           "owner",
    }
    db.Create(&member)

    c.JSON(200, org)
}
```

### 5.3 数据库管理

```http
### 创建数据库（个人）
POST /api/databases
Authorization: Bearer <token>
Content-Type: application/json

{
  "db_name": "我的实验数据",
  "description": "2024年Q1实验记录",
  "visibility": "private"
}

### 创建数据库（组织）
POST /api/databases
Authorization: Bearer <token>
Content-Type: application/json

{
  "db_name": "部门实验数据",
  "description": "团队共享",
  "organization_id": "org_001",
  "visibility": "organization"
}

### 获取用户可访问的数据库列表
GET /api/databases
Authorization: Bearer <token>

Response:
{
  "databases": [
    {
      "id": "db_001",
      "db_name": "我的实验数据",
      "role": "owner",
      "type": "personal"
    },
    {
      "id": "db_002",
      "db_name": "部门实验数据",
      "role": "editor",
      "type": "organization",
      "organization": "硬件研发部"
    }
  ]
}
```

### 5.4 表与字段管理

```http
### 创建表
POST /api/tables
Authorization: Bearer <token>
Content-Type: application/json

{
  "database_id": "db_001",
  "name": "实验记录表"
}

### 添加字段
POST /api/fields
Authorization: Bearer <token>
Content-Type: application/json

{
  "table_id": "tbl_001",
  "name": "实验日期",
  "type": "date"
}

### 添加单选字段
POST /api/fields
Authorization: Bearer <token>
Content-Type: application/json

{
  "table_id": "tbl_001",
  "name": "状态",
  "type": "single_select",
  "config": {
    "options": ["正常", "异常", "待定"]
  }
}
```

### 5.5 数据操作

```http
### 获取记录列表（支持分页、筛选）
GET /api/records?table_id=tbl_001&limit=50&offset=0&filter={"fld_003":"正常"}
Authorization: Bearer <token>

Response:
{
  "records": [...],
  "total": 150,
  "has_more": true
}

### 创建记录
POST /api/records
Authorization: Bearer <token>
Content-Type: application/json

{
  "table_id": "tbl_001",
  "data": {
    "fld_001": "2024-01-15",
    "fld_002": 25.5,
    "fld_003": "正常"
  }
}

### 更新记录（乐观锁）
PUT /api/records/{record_id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "data": {
    "fld_001": "2024-01-15",
    "fld_002": 28.0,
    "fld_003": "异常"
  },
  "version": 2
}

### 删除记录
DELETE /api/records/{record_id}
Authorization: Bearer <token>
```

### 5.6 文件上传

```http
### 上传文件
POST /api/files/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data

Body:
- record_id: "rec_001"
- file: (binary)

Response:
{
  "file_id": "file_001",
  "original_name": "实验结果.xlsx",
  "stored_name": "a1b2c3d4-...",
  "size": 102400
}
```

### 5.7 插件管理

```http
### 上传插件
POST /api/plugins
Authorization: Bearer <token>
Content-Type: multipart/form-data

Body:
- name: "自动计算字段"
- database_id: "db_001"
- event_type: "before_record_save"
- file: (Python script)

### 查看插件日志
GET /api/plugins/{plugin_id}/logs
Authorization: Bearer <token>

Response:
{
  "logs": [
    {
      "status": "success",
      "execution_time_ms": 120,
      "created_at": "2024-01-15T10:00:00"
    }
  ]
}
```

---

## 6. 性能优化策略

### 6.1 查询优化

```python
# 1. 使用物化视图缓存权限
def get_user_databases(user_id):
    # 而非多次查询
    return db.execute(
        "SELECT * FROM user_database_permissions WHERE user_id = ?",
        [user_id]
    )

# 2. 分页使用游标而非 offset
GET /api/records?cursor=eyJjcmVhdGVkX2F0IjoiMjAyNC0wMS0xNSJ9&limit=50

# 3. JSONB查询使用GIN索引
SELECT * FROM records
WHERE table_id = ?
  AND data @> ?  -- 使用GIN索引
```

### 6.2 缓存策略

```python
# 缓存策略（使用PostgreSQL物化视图）
1. 权限缓存: 物化视图自动刷新（5分钟）
2. 数据库元数据: 复合索引优化查询
3. 插件配置: 数据库索引查询

# 性能优化
- 物化视图：权限数据自动缓存
- 复合索引：双向查询优化
- GIN索引：JSONB字段查询
```

### 6.3 数据库优化

```sql
-- 1. 连接池配置
max_connections = 100
shared_buffers = 256MB

-- 2. 定期清理
-- 插件日志保留30天
DELETE FROM plugin_logs WHERE created_at < NOW() - INTERVAL '30 days';

-- 编辑锁自动清理
DELETE FROM edit_locks WHERE locked_at < NOW() - INTERVAL '10 minutes';

-- 3. 分区表（未来扩展）
CREATE TABLE records_partitioned (
    LIKE records INCLUDING ALL
) PARTITION BY RANGE (created_at);
```

---

## 7. 安全设计

### 7.1 认证安全

- **密码存储**: bcrypt (cost=12)
- **Token**: JWT (RS256), 2小时过期
- **刷新Token**: 7天过期
- **密码策略**: 最小8位，包含大小写+数字

### 7.2 授权安全

```go
// 所有API必须验证权限
func RequirePermission(role string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user_id := c.GetString("user_id")
        database_id := c.Param("database_id")

        if !checkPermission(user_id, database_id, role) {
            c.JSON(403, gin.H{"error": "无权访问"})
            c.Abort()
            return
        }
        c.Next()
    }
}

// 使用示例
func GetRecords(c *gin.Context) {
    // 权限已在中间件验证
    // ...
}

// 路由配置
router.GET("/api/databases/:database_id/records",
    AuthMiddleware(),
    RequirePermission("viewer"),
    GetRecords)
```

### 7.3 数据安全

- **SQL注入**: 使用GORM参数化查询
- **XSS**: 前端转义，CSP策略
- **CSRF**: SameSite Cookie + CSRF Token
- **文件上传**: 类型检查，大小限制，隔离存储

### 7.4 插件安全

- **沙箱隔离**: 子进程独立运行
- **超时控制**: 5秒强制终止
- **异常隔离**: 插件崩溃不影响主进程
- **资源限制**: CPU/内存限制（可选）

---

## 8. 部署架构

### 8.1 Docker Compose 部署

```yaml
version: '3.8'

services:
  # 前端服务
  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    environment:
      - VITE_API_URL=http://localhost:8000
    depends_on:
      - backend

  # 后端服务
  backend:
    build: ./backend
    ports:
      - "8000:8000"
    environment:
      - DATABASE_URL=postgresql://user:pass@db:5432/cornerstone
      - PLUGIN_DIR=/app/plugins
    volumes:
      - ./plugins:/app/plugins
      - ./uploads:/app/uploads
    depends_on:
      - db

  # 数据库
  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=cornerstone
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  # 可选：MinIO (对象存储)
  minio:
    image: minio/minio
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    command: server /data

volumes:
  pgdata:
```

### 8.2 环境配置

```bash
# .env
DATABASE_URL=postgresql://user:pass@localhost:5432/cornerstone
SECRET_KEY=your-secret-key
PLUGIN_TIMEOUT=5
UPLOAD_MAX_SIZE=100MB
```

---

## 9. 监控与日志

### 9.1 日志规范

```go
// 应用日志
logger.Info("用户登录", zap.String("user_id", userID), zap.String("ip", ip))
logger.Error("插件执行失败",
    zap.String("plugin_id", pluginID),
    zap.Error(err),
    zap.Int("execution_time", timeMs))

// 访问日志
logger.Info("API请求",
    zap.String("method", "POST"),
    zap.String("path", "/api/records"),
    zap.Int("status", 200),
    zap.Int("duration_ms", 45))
```

### 9.2 监控指标

- **系统指标**: CPU、内存、磁盘
- **数据库**: 连接数、慢查询、缓存命中率
- **应用**: API响应时间、错误率、插件执行时间
- **业务**: 用户活跃度、数据量增长

### 9.3 告警规则

- API错误率 > 5%
- 平均响应时间 > 2秒
- 数据库连接池耗尽
- 磁盘使用率 > 80%

---

## 10. 开发与测试

### 10.1 开发环境

```bash
# 前端 (Vue 3 + Vite)
cd frontend
npm install
npm run dev
# 访问: http://localhost:3000

# 后端 (Go)
cd backend
go mod download
go run main.go
# API: http://localhost:8000

# 数据库
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=pass postgres:15
```

### 10.2 测试策略

```go
// 单元测试
- 认证逻辑 (TestAuth)
- 权限校验 (TestPermission)
- 数据验证 (TestValidation)

// 集成测试
- API端到端测试 (TestAPI)
- 插件执行流程 (TestPlugin)
- 文件上传下载 (TestFile)

// 性能测试
- 单表10,000条数据查询
- 并发编辑测试
- 插件超时测试
```

### 10.3 CI/CD

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run Go tests
        run: go test ./...
      - name: Frontend tests
        run: |
          cd frontend
          npm install
          npm run test

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Build and push Docker images
        run: |
          docker-compose build
          docker-compose push
      - name: Deploy to server
        run: |
          docker-compose up -d
```

---

## 11. 扩展性设计

### 11.1 未来扩展方向

1. **多级组织架构**
   - 支持部门嵌套
   - 权限逐级继承

2. **数据版本历史**
   - 记录每次修改
   - 支持回滚

3. **API导出**
   - RESTful API
   - GraphQL 支持
   - 数据导出（CSV/Excel）

4. **插件市场**
   - 插件上传/分享
   - 插件评分系统

5. **数据分析**
   - 仪表盘
   - 图表展示
   - 数据聚合

### 11.2 技术债务

- MVP阶段暂不支持字段修改/删除
- 文件不支持预览和版本管理
- 插件不支持异步队列
- 无数据备份/恢复功能

---

## 12. 总结

### 12.1 核心优势

✅ **多租户支持**：个人 + 组织数据库模式
✅ **权限继承**：组织角色自动继承权限
✅ **插件系统**：Python子进程隔离执行
✅ **性能优化**：GIN索引 + 物化视图
✅ **安全可靠**：JWT认证 + 乐观锁
✅ **易部署**：Docker Compose一键启动

### 12.2 技术亮点

1. **JSONB动态字段**：无需修改表结构即可扩展字段
2. **双重数据库模式**：支持个人使用和团队协作
3. **插件沙箱**：子进程隔离，超时控制，异常隔离
4. **权限缓存**：物化视图提升权限查询性能
5. **乐观锁**：防止并发编辑冲突

### 12.3 下一步

1. 确认技术架构文档
2. 输出详细API接口文档
3. 开发环境搭建
4. Sprint 1 开发（核心数据建模）

---

**文档维护**：
- 2026-01-05 v1.0: 移除Redis，纯PostgreSQL架构
