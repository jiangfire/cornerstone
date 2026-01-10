# API 接口设计文档

**版本**: v1.0
**更新日期**: 2026-01-05
**技术栈**: Go 1.21 + Gin/Fiber + GORM
**认证方式**: JWT (Bearer Token)

---

## 一、接口规范

### 1.1 基础信息

**基础 URL**: `http://localhost:8080/api/v1`

**响应格式** (统一):
```json
{
  "code": 0,
  "message": "success",
  "data": {},
  "trace_id": "abc123..."
}
```

**错误码**:
| Code | 说明 | HTTP 状态码 |
|------|------|-------------|
| 0    | 成功 | 200 |
| 1001 | 未登录/Token失效 | 401 |
| 1002 | 权限不足 | 403 |
| 1003 | 参数错误 | 400 |
| 1004 | 资源不存在 | 404 |
| 1005 | 系统错误 | 500 |
| 1006 | 数据库错误 | 500 |
| 1007 | 业务冲突 | 409 |

### 1.2 分页规范

**请求参数**:
```go
type PaginationReq struct {
    Cursor  string `form:"cursor"`  // 游标（Base64编码的ID）
    Limit   int    `form:"limit,default=20"`  // 每页数量，最大100
    SortBy  string `form:"sort_by,default=created_at"`  // 排序字段
    Order   string `form:"order,default=desc"`  // asc/desc
}
```

**响应格式**:
```json
{
  "code": 0,
  "data": {
    "list": [],
    "pagination": {
      "has_more": true,
      "next_cursor": "eyJpZCI6IjEyMyJ9",
      "total": 100
    }
  }
}
```

---

## 二、认证与授权模块

### 2.1 用户注册

**POST** `/auth/register`

**请求**:
```json
{
  "username": "zhangsan",
  "password": "P@ssw0rd123",
  "user_code": "ZS001"
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "user_id": "usr_1234567890",
    "username": "zhangsan",
    "user_code": "ZS001",
    "created_at": "2026-01-05T10:00:00Z"
  }
}
```

**说明**:
- `user_code` 用于组织内唯一标识
- 密码需包含大小写、数字、特殊字符

---

### 2.2 用户登录

**POST** `/auth/login`

**请求**:
```json
{
  "username": "zhangsan",
  "password": "P@ssw0rd123"
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 3600,
    "user": {
      "user_id": "usr_1234567890",
      "username": "zhangsan",
      "user_code": "ZS001"
    }
  }
}
```

---

### 2.3 刷新 Token

**POST** `/auth/refresh`

**请求**:
```json
{
  "refresh_token": "..."
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "access_token": "new_access_token",
    "expires_in": 3600
  }
}
```

---

### 2.4 获取当前用户信息

**GET** `/auth/me`

**Headers**:
```
Authorization: Bearer <access_token>
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "user_id": "usr_1234567890",
    "username": "zhangsan",
    "user_code": "ZS001",
    "created_at": "2026-01-05T10:00:00Z",
    "orgs": [
      {
        "org_id": "org_123",
        "org_name": "研发部",
        "role": "admin"
      }
    ]
  }
}
```

---

## 三、数据库管理模块

### 3.1 创建数据库

**POST** `/databases`

**权限**: 需要登录

**请求**:
```json
{
  "db_name": "project_alpha",
  "display_name": "Alpha项目数据库",
  "type": "personal",  // personal | organization
  "description": "用于Alpha项目的硬件数据管理"
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "database_id": "db_1234567890",
    "db_name": "project_alpha",
    "display_name": "Alpha项目数据库",
    "owner_id": "usr_1234567890",
    "type": "personal",
    "created_at": "2026-01-05T10:00:00Z"
  }
}
```

---

### 3.2 获取数据库列表

**GET** `/databases`

**请求参数**:
```go
type DatabaseListReq struct {
    Type string `form:"type"`  // personal | organization | all
    PaginationReq
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "database_id": "db_1234567890",
        "db_name": "project_alpha",
        "display_name": "Alpha项目数据库",
        "type": "personal",
        "table_count": 5,
        "record_count": 1250,
        "permission": "owner",  // owner | admin | write | read
        "created_at": "2026-01-05T10:00:00Z"
      }
    ],
    "pagination": {
      "has_more": false,
      "next_cursor": null
    }
  }
}
```

---

### 3.3 获取数据库详情

**GET** `/databases/:database_id`

**权限**: 需要对该数据库有读权限

**响应**:
```json
{
  "code": 0,
  "data": {
    "database_id": "db_1234567890",
    "db_name": "project_alpha",
    "display_name": "Alpha项目数据库",
    "description": "用于Alpha项目的硬件数据管理",
    "type": "personal",
    "owner_id": "usr_1234567890",
    "owner_name": "zhangsan",
    "table_count": 5,
    "record_count": 1250,
    "created_at": "2026-01-05T10:00:00Z",
    "updated_at": "2026-01-05T10:00:00Z"
  }
}
```

---

### 3.4 更新数据库

**PUT** `/databases/:database_id`

**权限**: 需要对该数据库有管理员权限

**请求**:
```json
{
  "display_name": "Alpha项目数据库 v2",
  "description": "更新后的描述"
}
```

**响应**: 同详情接口

---

### 3.5 删除数据库

**DELETE** `/databases/:database_id`

**权限**: 需要对该数据库有所有者权限

**响应**:
```json
{
  "code": 0,
  "message": "数据库已删除"
}
```

**说明**: 软删除，实际数据保留7天后物理删除

---

### 3.6 数据库访问权限管理

#### 3.6.1 获取数据库权限列表

**GET** `/databases/:database_id/access`

**权限**: 需要管理员权限

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "access_id": "acc_123",
      "user_id": "usr_456",
      "username": "lisi",
      "user_code": "LS-002",
      "permission": "write",  // read | write | admin
      "granted_by": "usr_1234567890",
      "granted_at": "2026-01-05T10:00:00Z"
    }
  ]
}
```

---

#### 3.6.2 授予数据库访问权限

**POST** `/databases/:database_id/access`

**权限**: 需要管理员权限

**请求**:
```json
{
  "user_code": "LS-002",
  "permission": "write"
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "access_id": "acc_123",
    "granted": true
  }
}
```

---

#### 3.6.3 撤销数据库访问权限

**DELETE** `/databases/:database_id/access/:access_id`

**权限**: 需要管理员权限

**响应**:
```json
{
  "code": 0,
  "message": "权限已撤销"
}
```

---

## 四、表和字段管理模块

### 4.1 创建表

**POST** `/databases/:database_id/tables`

**权限**: 需要对该数据库有写权限

**请求**:
```json
{
  "table_name": "hardware_specs",
  "display_name": "硬件规格表",
  "fields": [
    {
      "field_name": "model",
      "display_name": "型号",
      "field_type": "text",
      "required": true,
      "unique": true
    },
    {
      "field_name": "cpu",
      "display_name": "CPU",
      "field_type": "text",
      "required": true
    },
    {
      "field_name": "memory",
      "display_name": "内存",
      "field_type": "number",
      "required": true
    },
    {
      "field_name": "price",
      "display_name": "价格",
      "field_type": "number",
      "required": false,
      "decimal": true
    },
    {
      "field_name": "specs",
      "display_name": "详细规格",
      "field_type": "json",
      "required": false
    }
  ]
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "table_id": "tbl_1234567890",
    "table_name": "hardware_specs",
    "display_name": "硬件规格表",
    "field_count": 5,
    "created_at": "2026-01-05T10:00:00Z"
  }
}
```

---

### 4.2 获取表列表

**GET** `/databases/:database_id/tables`

**响应**:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "table_id": "tbl_1234567890",
        "table_name": "hardware_specs",
        "display_name": "硬件规格表",
        "field_count": 5,
        "record_count": 1250,
        "created_at": "2026-01-05T10:00:00Z"
      }
    ]
  }
}
```

---

### 4.3 获取表结构详情

**GET** `/tables/:table_id`

**权限**: 需要对所属数据库有读权限

**响应**:
```json
{
  "code": 0,
  "data": {
    "table_id": "tbl_1234567890",
    "table_name": "hardware_specs",
    "display_name": "硬件规格表",
    "fields": [
      {
        "field_id": "fld_123",
        "field_name": "model",
        "display_name": "型号",
        "field_type": "text",
        "required": true,
        "unique": true,
        "created_at": "2026-01-05T10:00:00Z"
      }
    ],
    "indexes": [
      {
        "index_id": "idx_123",
        "index_name": "idx_model",
        "fields": ["model"],
        "type": "unique",
        "created_at": "2026-01-05T10:00:00Z"
      }
    ]
  }
}
```

---

### 4.4 添加字段

**POST** `/tables/:table_id/fields`

**权限**: 需要对所属数据库有写权限

**请求**:
```json
{
  "field_name": "manufacturer",
  "display_name": "制造商",
  "field_type": "text",
  "required": false
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "field_id": "fld_789",
    "field_name": "manufacturer",
    "display_name": "制造商"
  }
}
```

---

### 4.5 修改字段

**PUT** `/tables/:table_id/fields/:field_id`

**权限**: 需要对所属数据库有写权限

**请求**:
```json
{
  "display_name": "制造商（厂商）",
  "required": true
}
```

**说明**:
- 不能修改 `field_name` 和 `field_type`
- 如果字段已有数据，`required` 只能从 false 改为 true（需确保所有记录都有值）

---

### 4.6 删除字段

**DELETE** `/tables/:table_id/fields/:field_id`

**权限**: 需要对所属数据库有写权限

**响应**:
```json
{
  "code": 0,
  "message": "字段已删除"
}
```

**说明**:
- 如果字段已有数据，需要先清空数据才能删除
- 操作不可逆

---

## 五、数据操作模块

### 5.1 批量导入记录

**POST** `/tables/:table_id/records/bulk`

**权限**: 需要对所属数据库有写权限

**请求**:
```json
{
  "records": [
    {
      "model": "Xeon-W3400",
      "cpu": "Intel Xeon W3400",
      "memory": 128,
      "price": 45000.00,
      "specs": {
        "core_count": 56,
        "thread_count": 112,
        "base_freq": 1.9,
        "max_freq": 4.8
      }
    },
    {
      "model": "Ryzen-Threadripper-7995WX",
      "cpu": "AMD Ryzen Threadripper 7995WX",
      "memory": 256,
      "price": 68000.00,
      "specs": {
        "core_count": 96,
        "thread_count": 192,
        "base_freq": 2.5,
        "max_freq": 5.1
      }
    }
  ]
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "success_count": 2,
    "failed_count": 0,
    "records": [
      {
        "record_id": "rec_1234567890",
        "model": "Xeon-W3400",
        "created_at": "2026-01-05T10:00:00Z"
      },
      {
        "record_id": "rec_1234567891",
        "model": "Ryzen-Threadripper-7995WX",
        "created_at": "2026-01-05T10:00:00Z"
      }
    ]
  }
}
```

---

### 5.2 查询记录列表

**GET** `/tables/:table_id/records`

**权限**: 需要对所属数据库有读权限

**请求参数**:
```go
type RecordListReq struct {
    PaginationReq
    // 筛选条件（支持多条件组合）
    Filter string `form:"filter"`  // JSON格式: {\"cpu\": {\"$like\": \"%Xeon%\"}, \"memory\": {\"$gte\": 128}}
    // 字段投影
    Fields string `form:"fields\"`  // JSON格式: [\"model\", \"cpu\", \"price\"]
}
```

**Filter 支持的操作符**:
- `$eq`: 等于
- `$neq`: 不等于
- `$gt`: 大于
- `$gte`: 大于等于
- `$lt`: 小于
- `$lte`: 小于等于
- `$like`: 模糊匹配
- `$in`: 包含
- `$nin`: 不包含
- `$null`: 为空
- `$nnull`: 不为空

**示例请求**:
```
GET /tables/tbl_123/records?limit=20&filter={\"cpu\":{\"$like\":\"%Xeon%\"},\"memory\":{\"$gte\":128}}&fields=[\"model\",\"cpu\",\"price\"]
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "record_id": "rec_1234567890",
        "model": "Xeon-W3400",
        "cpu": "Intel Xeon W3400",
        "price": 45000.00,
        "created_at": "2026-01-05T10:00:00Z",
        "updated_at": "2026-01-05T10:00:00Z",
        "version": 1
      }
    ],
    "pagination": {
      "has_more": true,
      "next_cursor": "eyJpZCI6InJlY18xMjM0NTY3ODkxIn0="
    }
  }
}
```

---

### 5.3 获取单条记录

**GET** `/records/:record_id`

**权限**: 需要对所属数据库有读权限

**响应**:
```json
{
  "code": 0,
  "data": {
    "record_id": "rec_1234567890",
    "table_id": "tbl_1234567890",
    "table_name": "hardware_specs",
    "data": {
      "model": "Xeon-W3400",
      "cpu": "Intel Xeon W3400",
      "memory": 128,
      "price": 45000.00,
      "specs": {
        "core_count": 56,
        "thread_count": 112
      }
    },
    "created_by": "usr_1234567890",
    "created_at": "2026-01-05T10:00:00Z",
    "updated_by": "usr_1234567890",
    "updated_at": "2026-01-05T10:00:00Z",
    "version": 1
  }
}
```

---

### 5.4 更新记录

**PUT** `/records/:record_id`

**权限**: 需要对所属数据库有写权限

**请求**:
```json
{
  "data": {
    "price": 48000.00,
    "specs": {
      "core_count": 56,
      "thread_count": 112,
      "base_freq": 1.9,
      "max_freq": 4.8,
      "tdp": 350
    }
  },
  "version": 1
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "record_id": "rec_1234567890",
    "version": 2,
    "updated_at": "2026-01-05T10:05:00Z"
  }
}
```

**说明**:
- 乐观锁：必须传入当前 `version`，版本不匹配返回错误
- 支持部分更新，只传需要修改的字段

---

### 5.5 删除记录

**DELETE** `/records/:record_id`

**权限**: 需要对所属数据库有写权限

**请求**:
```json
{
  "version": 2
}
```

**响应**:
```json
{
  "code": 0,
  "message": "记录已删除"
}
```

**说明**: 软删除，保留7天后物理删除

---

### 5.6 编辑锁定（防止并发编辑）

#### 5.6.1 申请编辑锁

**POST** `/records/:record_id/lock`

**权限**: 需要对所属数据库有写权限

**请求**:
```json
{
  "ttl": 300  // 锁定时间（秒），默认300秒
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "lock_id": "lock_123",
    "locked_by": "usr_1234567890",
    "locked_at": "2026-01-05T10:00:00Z",
    "expires_at": "2026-01-05T10:05:00Z"
  }
}
```

**说明**:
- 如果记录已被他人锁定，返回错误
- 需要定期心跳保持锁定

---

#### 5.6.2 释放编辑锁

**DELETE** `/records/:record_id/lock`

**响应**:
```json
{
  "code": 0,
  "message": "锁已释放"
}
```

---

## 六、文件管理模块

### 6.1 上传文件

**POST** `/files/upload`

**权限**: 需要登录

**请求**: Multipart/form-data

```
Content-Type: multipart/form-data

file: <binary file>
record_id: rec_1234567890 (可选，关联到记录)
description: "硬件照片"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "file_id": "file_1234567890",
    "filename": "cpu_photo.jpg",
    "file_size": 204800,
    "file_type": "image/jpeg",
    "file_url": "/api/v1/files/file_1234567890/download",
    "uploaded_at": "2026-01-05T10:00:00Z"
  }
}
```

**说明**:
- 支持单文件/多文件上传
- 文件大小限制：100MB
- 支持的格式：图片、PDF、Excel、CSV

---

### 6.2 获取文件列表

**GET** `/files`

**请求参数**:
```go
type FileListReq struct {
    RecordID string `form:"record_id"`  // 按记录筛选
    PaginationReq
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "file_id": "file_1234567890",
        "filename": "cpu_photo.jpg",
        "file_size": 204800,
        "file_type": "image/jpeg",
        "record_id": "rec_1234567890",
        "uploaded_by": "usr_1234567890",
        "uploaded_at": "2026-01-05T10:00:00Z"
      }
    ]
  }
}
```

---

### 6.3 下载文件

**GET** `/files/:file_id/download`

**权限**: 需要对所属数据库有读权限

**响应**: 文件二进制流

---

### 6.4 删除文件

**DELETE** `/files/:file_id`

**权限**: 需要对所属数据库有写权限

**响应**:
```json
{
  "code": 0,
  "message": "文件已删除"
}
```

---

## 七、插件系统模块

### 7.1 注册插件

**POST** `/plugins`

**权限**: 系统管理员

**请求**:
```json
{
  "plugin_name": "数据校验插件",
  "plugin_type": "python",  // python | go | shell
  "entry_point": "/opt/plugins/validate.py",
  "description": "校验硬件规格数据的完整性",
  "config_schema": {
    "required_fields": ["model", "cpu", "memory"],
    "max_price": 100000
  }
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "plugin_id": "plg_1234567890",
    "plugin_name": "数据校验插件",
    "status": "active",
    "created_at": "2026-01-05T10:00:00Z"
  }
}
```

---

### 7.2 插件绑定到数据库

**POST** `/plugins/:plugin_id/bind`

**权限**: 系统管理员

**请求**:
```json
{
  "database_id": "db_1234567890",
  "table_name": "hardware_specs",
  "trigger": "before_save",  // before_save | after_save | before_delete | after_delete
  "config": {
    "required_fields": ["model", "cpu"]
  }
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "binding_id": "bind_123",
    "status": "active"
  }
}
```

---

### 7.3 执行插件（手动触发）

**POST** `/plugins/:plugin_id/execute`

**权限**: 需要对所属数据库有写权限

**请求**:
```json
{
  "database_id": "db_1234567890",
  "table_name": "hardware_specs",
  "record_ids": ["rec_123", "rec_456"],
  "config": {
    "strict_mode": true
  }
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "execution_id": "exec_1234567890",
    "status": "completed",  // pending | running | completed | failed
    "result": {
      "success": true,
      "message": "校验通过",
      "details": {
        "total": 2,
        "passed": 2,
        "failed": 0
      }
    },
    "started_at": "2026-01-05T10:00:00Z",
    "completed_at": "2026-01-05T10:00:05Z"
  }
}
```

---

### 7.4 获取插件执行日志

**GET** `/plugins/:plugin_id/logs`

**权限**: 需要对所属数据库有读权限

**请求参数**:
```go
type PluginLogReq struct {
    ExecutionID string `form:"execution_id"`
    PaginationReq
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "log_id": "log_123",
        "execution_id": "exec_1234567890",
        "level": "info",  // info | warn | error
        "message": "开始校验记录 rec_123",
        "timestamp": "2026-01-05T10:00:01Z"
      }
    ]
  }
}
```

---

## 八、系统管理模块

### 8.1 用户管理

#### 8.1.1 获取用户列表

**GET** `/admin/users`

**权限**: 系统管理员

**响应**:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "user_id": "usr_1234567890",
        "username": "zhangsan",
        "user_code": "ZS-001",
        "status": "active",
        "created_at": "2026-01-05T10:00:00Z",
        "last_login": "2026-01-05T09:30:00Z"
      }
    ]
  }
}
```

---

#### 8.1.2 禁用/启用用户

**PUT** `/admin/users/:user_id/status`

**权限**: 系统管理员

**请求**:
```json
{
  "status": "inactive"
}
```

**响应**:
```json
{
  "code": 0,
  "message": "用户状态已更新"
}
```

---

### 8.2 组织管理

#### 8.2.1 创建组织

**POST** `/organizations`

**请求**:
```json
{
  "org_name": "研发部",
  "description": "硬件产品研发团队"
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "org_id": "org_1234567890",
    "org_name": "研发部",
    "owner_id": "usr_1234567890"
  }
}
```

**说明**: 创建者自动成为组织所有者

---

#### 8.2.2 组织成员管理

**POST** `/organizations/:org_id/members`

**权限**: 组织管理员

**请求**:
```json
{
  "user_code": "LS-002",
  "role": "member"  // member | admin
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "member_id": "mbr_123",
    "joined": true
  }
}
```

---

### 8.3 操作日志

**GET** `/audit/logs`

**权限**: 系统管理员或数据库所有者

**请求参数**:
```go
type AuditLogReq struct {
    DatabaseID string `form:"database_id"`
    Operation  string `form:"operation"`  // create | update | delete | grant
    StartTime  string `form:"start_time"`  // ISO8601
    EndTime    string `form:"end_time"`
    PaginationReq
}
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "log_id": "audit_123",
        "user_id": "usr_1234567890",
        "username": "zhangsan",
        "operation": "update",
        "resource_type": "record",
        "resource_id": "rec_123",
        "details": {
          "table": "hardware_specs",
          "changes": {"price": {"old": 45000, "new": 48000}}
        },
        "ip_address": "192.168.1.100",
        "timestamp": "2026-01-05T10:05:00Z"
      }
    ]
  }
}
```

---

## 九、Go 语言代码示例

### 9.1 项目结构

```
cornerstone/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── handler/
│   │   ├── model/
│   │   ├── service/
│   │   ├── middleware/
│   │   ├── repository/
│   │   └── pkg/
│   ├── pkg/
│   │   ├── database/
│   │   ├── logger/
│   │   └── utils/
│   ├── uploads/
│   ├── go.mod
│   └── go.sum
```

---

### 9.2 主程序入口

**cmd/server/main.go**
```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cornerstone/internal/config"
	"cornerstone/internal/handler"
	"cornerstone/internal/repository"
	"cornerstone/internal/service"
	"cornerstone/pkg/database"
	"cornerstone/pkg/logger"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化日志
	logger.Init(cfg.Log.Level, cfg.Log.Path)
	defer logger.Sync()

	// 初始化数据库
	db, err := database.NewPostgres(cfg.Database.DSN)
	if err != nil {
		logger.Fatal("failed to connect database", logger.Error(err))
	}
	defer db.Close()

	// 自动迁移
	if err := database.AutoMigrate(db); err != nil {
		logger.Fatal("failed to migrate database", logger.Error(err))
	}

	// 初始化依赖
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	// 创建路由
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.GinLogger())

	// 注册路由
	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", userHandler.Register)
			auth.POST("/login", userHandler.Login)
			auth.POST("/refresh", userHandler.RefreshToken)
			auth.Use(handler.AuthMiddleware()).GET("/me", userHandler.GetMe)
		}

		// 其他路由...
	}

	// 启动服务器
	srv := &http.Server{
		Addr:    cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("server starting", logger.String("addr", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", logger.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", logger.Error(err))
	}

	logger.Info("server exited")
}
```

---

### 9.3 中间件 - JWT 认证

**internal/middleware/auth.go**
```go
package middleware

import (
	"net/http"
	"strings"

	"cornerstone/internal/config"
	"cornerstone/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    1001,
				"message": "missing authorization header",
			})
			return
		}

		// Bearer <token>
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    1001,
				"message": "invalid authorization format",
			})
			return
		}

		tokenString := parts[1]
		claims, err := ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    1001,
				"message": "invalid or expired token",
			})
			return
		}

		// 将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("user_code", claims.UserCode)

		c.Next()
	}
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	UserCode string `json:"user_code"`
	jwt.RegisteredClaims
}

func GenerateToken(userID, username, userCode string) (string, error) {
	cfg := config.Load()
	claims := Claims{
		UserID:   userID,
		Username: username,
		UserCode: userCode,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "cornerstone",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

func ParseToken(tokenString string) (*Claims, error) {
	cfg := config.Load()
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWT.Secret), nil
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

---

### 9.4 数据库操作 - 记录查询

**internal/repository/record.go**
```go
package repository

import (
	"encoding/json"
	"fmt"
	"strings"

	"cornerstone/internal/model"
	"cornerstone/pkg/logger"

	"gorm.io/gorm"
)

type RecordRepository interface {
	Create(record *model.Record) error
	GetByID(id string) (*model.Record, error)
	List(tableID string, filter map[string]interface{}, limit int, cursor string) ([]model.Record, string, error)
	Update(record *model.Record) error
	Delete(id string) error
}

type recordRepository struct {
	db *gorm.DB
}

func NewRecordRepository(db *gorm.DB) RecordRepository {
	return &recordRepository{db: db}
}

func (r *recordRepository) Create(record *model.Record) error {
	return r.db.Create(record).Error
}

func (r *recordRepository) GetByID(id string) (*model.Record, error) {
	var record model.Record
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *recordRepository) List(tableID string, filter map[string]interface{}, limit int, cursor string) ([]model.Record, string, error) {
	query := r.db.Where("table_id = ? AND deleted_at IS NULL", tableID)

	// 处理游标
	if cursor != "" {
		cursorID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		query = query.Where("id < ?", cursorID)
	}

	// 处理筛选条件
	if len(filter) > 0 {
		for key, value := range filter {
			switch v := value.(type) {
			case map[string]interface{}:
				for op, val := range v {
					switch op {
					case "$eq":
						query = query.Where(fmt.Sprintf("data->>'%s' = ?", key), val)
					case "$neq":
						query = query.Where(fmt.Sprintf("data->>'%s' != ?", key), val)
					case "$gt":
						query = query.Where(fmt.Sprintf("CAST(data->>'%s' AS NUMERIC) > ?", key), val)
					case "$gte":
						query = query.Where(fmt.Sprintf("CAST(data->>'%s' AS NUMERIC) >= ?", key), val)
					case "$lt":
						query = query.Where(fmt.Sprintf("CAST(data->>'%s' AS NUMERIC) < ?", key), val)
					case "$lte":
						query = query.Where(fmt.Sprintf("CAST(data->>'%s' AS NUMERIC) <= ?", key), val)
					case "$like":
						query = query.Where(fmt.Sprintf("data->>'%s' LIKE ?", key), "%"+val.(string)+"%")
					case "$in":
						query = query.Where(fmt.Sprintf("data->>'%s' IN (?)", key), val)
					case "$nin":
						query = query.Where(fmt.Sprintf("data->>'%s' NOT IN (?)", key), val)
					case "$null":
						query = query.Where(fmt.Sprintf("data->>'%s' IS NULL", key))
					case "$nnull":
						query = query.Where(fmt.Sprintf("data->>'%s' IS NOT NULL", key))
					}
				}
			default:
				query = query.Where(fmt.Sprintf("data->>'%s' = ?", key), v)
			}
		}
	}

	// 排序和限制
	query = query.Order("id DESC").Limit(limit + 1)

	var records []model.Record
	if err := query.Find(&records).Error; err != nil {
		return nil, "", err
	}

	// 生成下一个游标
	var nextCursor string
	if len(records) > limit {
		nextCursor = encodeCursor(records[limit].ID)
		records = records[:limit]
	}

	return records, nextCursor, nil
}

func (r *recordRepository) Update(record *model.Record) error {
	return r.db.Save(record).Error
}

func (r *recordRepository) Delete(id string) error {
	return r.db.Delete(&model.Record{}, "id = ?", id).Error
}

func encodeCursor(id string) string {
	data := map[string]string{"id": id}
	jsonData, _ := json.Marshal(data)
	return strings.TrimRight(base64.StdEncoding.EncodeToString(jsonData), "=")
}

func decodeCursor(cursor string) (string, error) {
	// 补全可能缺失的 =
	if l := len(cursor) % 4; l != 0 {
		cursor += strings.Repeat("=", 4-l)
	}
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	var data map[string]string
	if err := json.Unmarshal(decoded, &data); err != nil {
		return "", err
	}
	return data["id"], nil
}
```

---

### 9.5 服务层 - 数据库权限检查

**internal/service/database.go**
```go
package service

import (
	"errors"
	"fmt"

	"cornerstone/internal/model"
	"cornerstone/internal/repository"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrDatabaseNotFound = errors.New("database not found")
)

type DatabaseService interface {
	CheckPermission(userID, databaseID string, requiredPerm model.Permission) error
	CreateDatabase(userID string, req CreateDatabaseReq) (*model.Database, error)
}

type databaseService struct {
	dbRepo     repository.DatabaseRepository
	accessRepo repository.DatabaseAccessRepository
}

func NewDatabaseService(dbRepo repository.DatabaseRepository, accessRepo repository.DatabaseAccessRepository) DatabaseService {
	return &databaseService{
		dbRepo:     dbRepo,
		accessRepo: accessRepo,
	}
}

func (s *databaseService) CheckPermission(userID, databaseID string, requiredPerm model.Permission) error {
	// 获取数据库
	db, err := s.dbRepo.GetByID(databaseID)
	if err != nil {
		return ErrDatabaseNotFound
	}

	// 如果是所有者，直接通过
	if db.OwnerID == userID {
		return nil
	}

	// 获取访问权限
	access, err := s.accessRepo.GetByUserAndDatabase(userID, databaseID)
	if err != nil {
		return ErrPermissionDenied
	}

	// 检查权限级别
	if !hasSufficientPermission(access.Permission, requiredPerm) {
		return fmt.Errorf("%w: 需要 %v，当前只有 %v", ErrPermissionDenied, requiredPerm, access.Permission)
	}

	return nil
}

func hasSufficientPermission(userPerm, requiredPerm model.Permission) bool {
	permLevels := map[model.Permission]int{
		model.PermissionRead:  1,
		model.PermissionWrite: 2,
		model.PermissionAdmin: 3,
	}

	return permLevels[userPerm] >= permLevels[requiredPerm]
}

type CreateDatabaseReq struct {
	DBName      string `json:"db_name" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Type        string `json:"type" binding:"required,oneof=personal organization"`
	Description string `json:"description"`
}

func (s *databaseService) CreateDatabase(userID string, req CreateDatabaseReq) (*model.Database, error) {
	database := &model.Database{
		ID:          generateID("db_"),
		DBName:      req.DBName,
		DisplayName: req.DisplayName,
		OwnerID:     userID,
		Type:        model.DatabaseType(req.Type),
		Description: req.Description,
	}

	if err := s.dbRepo.Create(database); err != nil {
		return nil, err
	}

	// 自动授予所有者完全权限
	access := &model.DatabaseAccess{
		ID:         generateID("acc_"),
		UserID:     userID,
		DatabaseID: database.ID,
		Permission: model.PermissionAdmin,
	}

	if err := s.accessRepo.Create(access); err != nil {
		return nil, err
	}

	return database, nil
}
```

---

### 9.6 处理器 - 记录操作

**internal/handler/record.go**
```go
package handler

import (
	"net/http"
	"strconv"

	"cornerstone/internal/model"
	"cornerstone/internal/service"
	"cornerstone/pkg/logger"

	"github.com/gin-gonic/gin"
)

type RecordHandler struct {
	recordService service.RecordService
}

func NewRecordHandler(recordService service.RecordService) *RecordHandler {
	return &RecordHandler{recordService: recordService}
}

// CreateBatch 批量创建记录
func (h *RecordHandler) CreateBatch(c *gin.Context) {
	tableID := c.Param("table_id")
	userID := c.GetString("user_id")

	var req struct {
		Records []map[string]interface{} `json:"records" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1003,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	results, err := h.recordService.CreateBatch(userID, tableID, req.Records)
	if err != nil {
		logger.Error("failed to create records", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1006,
			"message": "创建失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"success_count": len(results),
			"failed_count":  0,
			"records":       results,
		},
	})
}

// List 查询记录列表
func (h *RecordHandler) List(c *gin.Context) {
	tableID := c.Param("table_id")
	userID := c.GetString("user_id")

	// 解析分页参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit > 100 {
		limit = 100
	}
	cursor := c.Query("cursor")

	// 解析筛选条件
	filterStr := c.Query("filter")
	var filter map[string]interface{}
	if filterStr != "" {
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    1003,
				"message": "筛选条件格式错误",
			})
			return
		}
	}

	// 检查权限
	if err := h.recordService.CheckPermission(userID, tableID, model.PermissionRead); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    1002,
			"message": err.Error(),
		})
		return
	}

	records, nextCursor, err := h.recordService.List(tableID, filter, limit, cursor)
	if err != nil {
		logger.Error("failed to list records", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1006,
			"message": "查询失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list": records,
			"pagination": gin.H{
				"has_more":    nextCursor != "",
				"next_cursor": nextCursor,
			},
		},
	})
}

// Update 更新记录
func (h *RecordHandler) Update(c *gin.Context) {
	recordID := c.Param("record_id")
	userID := c.GetString("user_id")

	var req struct {
		Data    map[string]interface{} `json:"data" binding:"required"`
		Version int                    `json:"version" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1003,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 检查编辑锁
	locked, lockInfo, err := h.recordService.CheckEditLock(recordID)
	if err != nil {
		logger.Error("failed to check edit lock", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1006,
			"message": "系统错误",
		})
		return
	}

	if locked && lockInfo.LockedBy != userID {
		c.JSON(http.StatusConflict, gin.H{
			"code":    1007,
			"message": "记录正在被 " + lockInfo.LockedBy + " 编辑中",
			"data": gin.H{
				"locked_by":    lockInfo.LockedBy,
				"locked_at":    lockInfo.LockedAt,
				"expires_at":   lockInfo.ExpiresAt,
			},
		})
		return
	}

	// 执行更新
	updated, err := h.recordService.Update(userID, recordID, req.Data, req.Version)
	if err != nil {
		if errors.Is(err, service.ErrVersionConflict) {
			c.JSON(http.StatusConflict, gin.H{
				"code":    1007,
				"message": "版本冲突，请刷新后重试",
			})
			return
		}
		logger.Error("failed to update record", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1006,
			"message": "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"record_id":  updated.ID,
			"version":    updated.Version,
			"updated_at": updated.UpdatedAt,
		},
	})
}
```

---

### 9.7 插件执行器

**internal/service/plugin.go**
```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"cornerstone/internal/model"
	"cornerstone/internal/repository"
	"cornerstone/pkg/logger"
)

type PluginExecutor interface {
	Execute(plugin *model.Plugin, req ExecuteRequest) (*model.PluginExecution, error)
}

type pluginExecutor struct {
	execRepo repository.PluginExecutionRepository
	logRepo  repository.PluginLogRepository
}

type ExecuteRequest struct {
	DatabaseID string                 `json:"database_id"`
	TableName  string                 `json:"table_name"`
	RecordIDs  []string               `json:"record_ids"`
	Config     map[string]interface{} `json:"config"`
}

func (e *pluginExecutor) Execute(plugin *model.Plugin, req ExecuteRequest) (*model.PluginExecution, error) {
	// 创建执行记录
	execution := &model.PluginExecution{
		ID:         generateID("exec_"),
		PluginID:   plugin.ID,
		Status:     model.ExecutionStatusRunning,
		StartTime:  time.Now(),
	}
	if err := e.execRepo.Create(execution); err != nil {
		return nil, err
	}

	// 准备输入数据
	inputData := map[string]interface{}{
		"database_id": req.DatabaseID,
		"table_name":  req.TableName,
		"record_ids":  req.RecordIDs,
		"config":      req.Config,
	}
	inputJSON, _ := json.Marshal(inputData)

	// 执行插件
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, plugin.EntryPoint)
	cmd.Stdin = bytes.NewReader(inputJSON)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 设置环境变量
	cmd.Env = append(os.Environ(),
		"PLUGIN_ID="+plugin.ID,
		"DATABASE_ID="+req.DatabaseID,
		"TABLE_NAME="+req.TableName,
	)

	// 记录开始日志
	e.logRepo.Create(&model.PluginLog{
		PluginID:      plugin.ID,
		ExecutionID:   execution.ID,
		Level:         model.LogLevelInfo,
		Message:       fmt.Sprintf("插件开始执行，输入: %s", string(inputJSON)),
		Timestamp:     time.Now(),
	})

	// 执行
	err := cmd.Run()

	// 记录输出
	if stdout.Len() > 0 {
		e.logRepo.Create(&model.PluginLog{
			PluginID:    plugin.ID,
			ExecutionID: execution.ID,
			Level:       model.LogLevelInfo,
			Message:     stdout.String(),
			Timestamp:   time.Now(),
		})
	}

	if stderr.Len() > 0 {
		e.logRepo.Create(&model.PluginLog{
			PluginID:    plugin.ID,
			ExecutionID: execution.ID,
			Level:       model.LogLevelError,
			Message:     stderr.String(),
			Timestamp:   time.Now(),
		})
	}

	// 更新执行状态
	execution.EndTime = time.Now()
	if err != nil {
		execution.Status = model.ExecutionStatusFailed
		execution.Result = map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		e.logRepo.Create(&model.PluginLog{
			PluginID:    plugin.ID,
			ExecutionID: execution.ID,
			Level:       model.LogLevelError,
			Message:     fmt.Sprintf("执行失败: %v", err),
			Timestamp:   time.Now(),
		})
	} else {
		execution.Status = model.ExecutionStatusCompleted
		execution.Result = map[string]interface{}{
			"success": true,
			"message": "执行成功",
		}
		e.logRepo.Create(&model.PluginLog{
			PluginID:    plugin.ID,
			ExecutionID: execution.ID,
			Level:       model.LogLevelInfo,
			Message:     "插件执行完成",
			Timestamp:   time.Now(),
		})
	}

	e.execRepo.Update(execution)
	return execution, nil
}
```

---

## 十、错误处理规范

### 10.1 统一错误响应

**internal/pkg/app/response.go**
```go
package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
		TraceID: c.GetString("trace_id"),
	})
}

func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		TraceID: c.GetString("trace_id"),
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, 1003, message)
}

func Unauthorized(c *gin.Context) {
	Error(c, 1001, "未登录或Token已失效")
}

func Forbidden(c *gin.Context, message string) {
	Error(c, 1002, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, 1004, message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, 1005, message)
}
```

---

### 10.2 业务错误定义

**internal/pkg/app/errors.go**
```go
package app

import "errors"

var (
	ErrInvalidParams      = errors.New("参数错误")
	ErrRecordNotFound     = errors.New("记录不存在")
	ErrPermissionDenied   = errors.New("权限不足")
	ErrVersionConflict    = errors.New("版本冲突")
	ErrDatabaseError      = errors.New("数据库错误")
	ErrLockConflict       = errors.New("记录被锁定")
	ErrPluginExecution    = errors.New("插件执行失败")
	ErrUserAlreadyExists  = errors.New("用户已存在")
	ErrInvalidCredentials = errors.New("用户名或密码错误")
)
```

---

### 10.3 错误码映射

**internal/pkg/app/error_code.go**
```go
package app

const (
	CodeSuccess = 0

	// 认证相关
	CodeUnauthorized     = 1001
	CodePermissionDenied = 1002

	// 请求相关
	CodeInvalidParams = 1003
	CodeNotFound      = 1004

	// 系统相关
	CodeInternalError = 1005
	CodeDatabaseError = 1006
	CodeConflict      = 1007
)

var ErrorMessages = map[int]string{
	CodeUnauthorized:     "未登录或Token已失效",
	CodePermissionDenied: "权限不足",
	CodeInvalidParams:    "参数错误",
	CodeNotFound:         "资源不存在",
	CodeInternalError:    "系统错误",
	CodeDatabaseError:    "数据库错误",
	CodeConflict:         "业务冲突",
}
```

---

## 十一、性能优化建议

### 11.1 查询优化

1. **使用 GIN 索引**
```sql
CREATE INDEX idx_records_data_gin ON records USING GIN (data);
```

2. **复合索引**
```sql
CREATE INDEX idx_access_user_database ON database_access (user_id, database_id, permission);
```

3. **部分索引**
```sql
CREATE INDEX idx_plugins_active ON plugins (id) WHERE status = 'active';
```

---

### 11.2 缓存策略

```go
// 使用 PostgreSQL 物化视图缓存权限检查结果
func (s *databaseService) CheckPermission(userID, databaseID string, requiredPerm model.Permission) error {
    // 查询物化视图（已预计算权限）
    // 物化视图每5分钟自动刷新一次
    var perm model.Permission
    err := s.db.Raw(`
        SELECT permission FROM user_database_permissions
        WHERE user_id = ? AND database_id = ?`,
        userID, databaseID).Scan(&perm).Error

    if err != nil {
        // 回退到实时查询
        return s.checkPermissionRealtime(userID, databaseID, requiredPerm)
    }

    if !hasSufficientPermission(perm, requiredPerm) {
        return ErrPermissionDenied
    }
    return nil
}

// 实时权限检查（物化视图未命中时）
func (s *databaseService) checkPermissionRealtime(userID, databaseID string, requiredPerm model.Permission) error {
    // 1. 检查数据库所有者
    var db model.Database
    if err := s.db.First(&db, "id = ?", databaseID).Error; err != nil {
        return err
    }
    if db.OwnerID == userID {
        return nil
    }

    // 2. 检查数据库访问权限
    var access model.DatabaseAccess
    err := s.db.Where("user_id = ? AND database_id = ?", userID, databaseID).First(&access).Error
    if err != nil {
        return ErrPermissionDenied
    }

    // 3. 检查权限级别
    if !hasSufficientPermission(access.Permission, requiredPerm) {
        return ErrPermissionDenied
    }

    return nil
}
```

**说明**：
- 使用 PostgreSQL **物化视图**替代 Redis 缓存
- 物化视图 `user_database_permissions` 每5分钟自动刷新
- 首次查询未命中时，回退到实时查询并自动更新物化视图
- PostgreSQL 主键查询 <1ms，性能足够

---

### 11.3 批量操作

```go
// 使用事务批量操作
func (r *recordRepository) CreateBatch(records []*model.Record) error {
    return r.db.Transaction(func(tx *gorm.DB) error {
        for _, record := range records {
            if err := tx.Create(record).Error; err != nil {
                return err
            }
        }
        return nil
    })
}
```

---

## 十二、测试示例

### 12.1 单元测试

```go
package service_test

import (
	"testing"

	"cornerstone/internal/model"
	"cornerstone/internal/service"
	"cornerstone/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDatabaseService_CheckPermission(t *testing.T) {
	mockDBRepo := new(mocks.DatabaseRepository)
	mockAccessRepo := new(mocks.DatabaseAccessRepository)

	svc := service.NewDatabaseService(mockDBRepo, mockAccessRepo)

	// 测试所有者权限
	mockDBRepo.On("GetByID", "db_123").Return(&model.Database{
		ID:      "db_123",
		OwnerID: "user_123",
	}, nil)

	err := svc.CheckPermission("user_123", "db_123", model.PermissionRead)
	assert.NoError(t, err)

	// 测试权限不足
	mockDBRepo.On("GetByID", "db_456").Return(&model.Database{
		ID:      "db_456",
		OwnerID: "user_789",
	}, nil)

	mockAccessRepo.On("GetByUserAndDatabase", "user_123", "db_456").Return(&model.DatabaseAccess{
		Permission: model.PermissionRead,
	}, nil)

	err = svc.CheckPermission("user_123", "db_456", model.PermissionWrite)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}
```

---

### 12.2 API 测试

```go
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cornerstone/internal/handler"
	"cornerstone/internal/service"
	"cornerstone/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecordHandler_CreateBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(mocks.RecordService)
	h := handler.NewRecordHandler(mockService)

	router := gin.New()
	router.POST("/tables/:table_id/records", h.CreateBatch)

	// 准备请求
	records := []map[string]interface{}{
		{"model": "Test1", "cpu": "Intel"},
		{"model": "Test2", "cpu": "AMD"},
	}
	body, _ := json.Marshal(map[string]interface{}{
		"records": records,
	})

	req := httptest.NewRequest("POST", "/tables/tbl_123/records", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_token")

	// 模拟用户上下文
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
}
```

---

## 十三、API 版本管理

### 13.1 版本策略

- URL 路径版本：`/api/v1/...`
- 向后兼容：至少保留 2 个版本
- 提前 3 个月公告，6 个月后移除

### 13.2 升级示例

**v1 (当前)**
```
GET /api/v1/tables/:table_id/records
```

**v2 (未来)**
```
GET /api/v2/tables/:table_id/records
```

---

## 十四、部署配置

### 14.1 Docker 配置

**backend/Dockerfile**
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

CMD ["./main"]
```

---

### 14.2 Docker Compose

**docker-compose.yml**
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: cornerstone
      POSTGRES_USER: cornerstone
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U cornerstone"]
      interval: 10s
      timeout: 5s
      retries: 5

  backend:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      - DB_DSN=postgres://cornerstone:${DB_PASSWORD}@postgres:5432/cornerstone?sslmode=disable
      - JWT_SECRET=${JWT_SECRET}
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./uploads:/app/uploads

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    depends_on:
      - backend

volumes:
  postgres_data:
```

---

## 十五、监控与日志

### 15.1 日志格式

```json
{
  "timestamp": "2026-01-05T10:00:00Z",
  "level": "INFO",
  "trace_id": "abc123",
  "user_id": "usr_123",
  "module": "record_service",
  "operation": "update",
  "message": "记录更新成功",
  "duration_ms": 45,
  "details": {
    "record_id": "rec_123",
    "version": 2
  }
}
```

---

### 15.2 性能监控指标

- API 响应时间（P50, P95, P99）
- 数据库查询时间
- 插件执行时间
- 并发连接数
- 错误率

---

## 附录 A: 状态码速查表

| 模块 | 接口 | 方法 | 认证 | 权限 |
|------|------|------|------|------|
| 认证 | /auth/register | POST | ❌ | - |
| 认证 | /auth/login | POST | ❌ | - |
| 认证 | /auth/refresh | POST | ❌ | - |
| 认证 | /auth/me | GET | ✅ | - |
| 数据库 | /databases | POST | ✅ | - |
| 数据库 | /databases | GET | ✅ | - |
| 数据库 | /databases/:id | GET | ✅ | read |
| 数据库 | /databases/:id | PUT | ✅ | admin |
| 数据库 | /databases/:id | DELETE | ✅ | owner |
| 权限 | /databases/:id/access | GET | ✅ | admin |
| 权限 | /databases/:id/access | POST | ✅ | admin |
| 权限 | /databases/:id/access/:aid | DELETE | ✅ | admin |
| 表 | /databases/:id/tables | POST | ✅ | write |
| 表 | /databases/:id/tables | GET | ✅ | read |
| 表 | /tables/:id | GET | ✅ | read |
| 字段 | /tables/:id/fields | POST | ✅ | write |
| 字段 | /tables/:id/fields/:fid | PUT | ✅ | write |
| 字段 | /tables/:id/fields/:fid | DELETE | ✅ | write |
| 记录 | /tables/:id/records/bulk | POST | ✅ | write |
| 记录 | /tables/:id/records | GET | ✅ | read |
| 记录 | /records/:id | GET | ✅ | read |
| 记录 | /records/:id | PUT | ✅ | write |
| 记录 | /records/:id | DELETE | ✅ | write |
| 锁 | /records/:id/lock | POST | ✅ | write |
| 锁 | /records/:id/lock | DELETE | ✅ | write |
| 文件 | /files/upload | POST | ✅ | write |
| 文件 | /files | GET | ✅ | read |
| 文件 | /files/:id/download | GET | ✅ | read |
| 文件 | /files/:id | DELETE | ✅ | write |
| 插件 | /plugins | POST | ✅ | admin |
| 插件 | /plugins/:id/bind | POST | ✅ | admin |
| 插件 | /plugins/:id/execute | POST | ✅ | write |
| 插件 | /plugins/:id/logs | GET | ✅ | read |
| 管理 | /admin/users | GET | ✅ | sysadmin |
| 管理 | /admin/users/:id/status | PUT | ✅ | sysadmin |
| 组织 | /organizations | POST | ✅ | - |
| 组织 | /organizations/:id/members | POST | ✅ | orgadmin |
| 审计 | /audit/logs | GET | ✅ | sysadmin |

---

## 附录 B: Go 依赖包推荐

```go
module cornerstone

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/golang-jwt/jwt/v5 v5.0.0
    github.com/google/uuid v1.4.0
    github.com/spf13/viper v1.17.0
    github.com/stretchr/testify v1.8.4
    gorm.io/driver/postgres v1.5.4
    gorm.io/gorm v1.25.5
    github.com/sirupsen/logrus v1.9.3
    github.com/gin-contrib/cors v1.5.0
    github.com/gin-contrib/requestid v1.0.0
)
```

---

**文档结束**

**版本历史**:
- v1.0 (2026-01-05): 初始版本，包含完整的 API 设计
