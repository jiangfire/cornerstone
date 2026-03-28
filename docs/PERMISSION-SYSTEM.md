# Cornerstone 权限系统文档

## 📋 目录

- [概述](#概述)
- [权限模型](#权限模型)
- [角色定义](#角色定义)
- [权限矩阵](#权限矩阵)
- [实现细节](#实现细节)
- [使用指南](#使用指南)
- [测试报告](#测试报告)

---

## 概述

Cornerstone 采用三层权限模型，提供细粒度的访问控制：

- **L1: 数据库级权限** - 控制用户对整个数据库的访问
- **L2: 表级权限** - 继承数据库权限，控制表的操作
- **L3: 字段级权限** - 最细粒度，控制单个字段的读写删除

### 设计原则

1. **双重保护**: 前端UI控制 + 后端API验证
2. **权限继承**: 下级权限继承上级权限
3. **最小权限**: 默认最小权限，显式授权
4. **角色分离**: 清晰的角色层级和职责

---

## 权限模型

### 三层权限架构

```
┌─────────────────────────────────────┐
│   L1: 数据库级权限 (Database)        │
│   - Owner, Admin, Editor, Viewer    │
└──────────────┬──────────────────────┘
               │ 继承
┌──────────────▼──────────────────────┐
│   L2: 表级权限 (Table)               │
│   - 继承数据库角色权限               │
└──────────────┬──────────────────────┘
               │ 继承
┌──────────────▼──────────────────────┐
│   L3: 字段级权限 (Field)             │
│   - R (Read), W (Write), D (Delete) │
└─────────────────────────────────────┘
```

### 权限操作类型

| 操作 | 说明 | 适用层级 |
|------|------|----------|
| **R** (Read) | 读取/查看 | 字段级 |
| **W** (Write) | 写入/编辑 | 字段级 |
| **D** (Delete) | 删除 | 字段级 |
| **Create** | 创建 | 数据库、表、记录 |
| **Update** | 更新 | 数据库、表、记录 |
| **Delete** | 删除 | 数据库、表、记录 |

---

## 角色定义

### Owner (所有者)

**权限范围**: 完全控制权

- ✅ 所有读写删除操作
- ✅ 管理数据库成员
- ✅ 删除数据库
- ✅ 配置所有权限
- ⚠️ 只能将数据库分享给其他 Owner

**使用场景**: 数据库创建者，项目负责人

> 注：当前实现不允许通过“分享数据库”或“修改成员角色”接口创建额外的 `owner`；可授予的数据库角色为 `admin`、`editor`、`viewer`。

### Admin (管理员)

**权限范围**: 管理权限（不能删除数据库）

- ✅ 所有读写操作
- ✅ 管理表和记录
- ✅ 配置字段权限
- ✅ 可以分享给 Admin/Editor/Viewer
- ❌ 不能删除数据库

**使用场景**: 项目管理员，团队负责人

### Editor (编辑者)

**权限范围**: 编辑权限（不能删除）

- ✅ 读取所有数据
- ✅ 创建和编辑记录
- ✅ 创建表和字段
- ✅ 可配置字段的 R/W 权限
- ❌ 不能删除表、记录
- ❌ 不能配置字段的 D 权限

**使用场景**: 内容编辑者，数据录入员

### Viewer (查看者)

**权限范围**: 只读权限

- ✅ 查看所有数据
- ✅ 可配置字段的 R 权限
- ❌ 不能创建、编辑、删除任何内容
- ❌ 不能配置字段的 W/D 权限

**使用场景**: 数据查看者，报表用户

---

## 权限矩阵

### 数据库级权限

| 操作 | Owner | Admin | Editor | Viewer |
|------|-------|-------|--------|--------|
| 查看数据库 | ✅ | ✅ | ✅ | ✅ |
| 编辑数据库信息 | ✅ | ✅ | ❌ | ❌ |
| 删除数据库 | ✅ | ❌ | ❌ | ❌ |
| 分享数据库 | ✅ | ✅ | ❌ | ❌ |
| 管理成员 | ✅ | ✅ | ❌ | ❌ |

### 表级权限

| 操作 | Owner | Admin | Editor | Viewer |
|------|-------|-------|--------|--------|
| 查看表 | ✅ | ✅ | ✅ | ✅ |
| 创建表 | ✅ | ✅ | ✅ | ❌ |
| 编辑表信息 | ✅ | ✅ | ✅ | ❌ |
| 删除表 | ✅ | ✅ | ❌ | ❌ |

### 字段级权限

| 操作 | Owner | Admin | Editor | Viewer |
|------|-------|-------|--------|--------|
| 查看字段 | ✅ | ✅ | ✅ | ✅ |
| 创建字段 | ✅ | ✅ | ✅ | ❌ |
| 编辑字段 | ✅ | ✅ | ✅ | ❌ |
| 删除字段 | ✅ | ✅ | ❌ | ❌ |
| 配置 R 权限 | ✅ | ✅ | ✅ | ✅ |
| 配置 W 权限 | ✅ | ✅ | ✅ | ❌ |
| 配置 D 权限 | ✅ | ✅ | ❌ | ❌ |

### 记录级权限

| 操作 | Owner | Admin | Editor | Viewer |
|------|-------|-------|--------|--------|
| 查看记录 | ✅ | ✅ | ✅ | ✅ |
| 创建记录 | ✅ | ✅ | ✅ | ❌ |
| 编辑记录 | ✅ | ✅ | ✅ | ❌ |
| 删除记录 | ✅ | ✅ | ❌ | ❌ |
| 上传附件 | ✅ | ✅ | ✅ | ❌ |
| 删除附件 | ✅ | ✅ | ❌ | ❌ |

---

## 实现细节

### 后端实现

#### 1. 数据库模型

```go
// database_access 表
type DatabaseAccess struct {
    ID         string    `gorm:"type:varchar(50);primaryKey"`
    UserID     string    `gorm:"type:varchar(50);not null"`
    DatabaseID string    `gorm:"type:varchar(50);not null"`
    Role       string    `gorm:"type:varchar(50);not null"` // owner, admin, editor, viewer
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// field_permissions 表
type FieldPermission struct {
    ID         string    `gorm:"type:varchar(50);primaryKey"`
    TableID    string    `gorm:"type:varchar(50);not null"`
    FieldID    string    `gorm:"type:varchar(50);not null"`
    Role       string    `gorm:"type:varchar(50);not null"`
    CanRead    bool      `gorm:"default:true"`
    CanWrite   bool      `gorm:"default:false"`
    CanDelete  bool      `gorm:"default:false"`
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

#### 2. 权限验证中间件

```go
// 验证数据库访问权限
func CheckDatabaseAccess(dbID, userID string, requiredRole string) error {
    var access models.DatabaseAccess
    err := db.Where("database_id = ? AND user_id = ?", dbID, userID).First(&access).Error
    if err != nil {
        return errors.New("无权访问该数据库")
    }

    // 验证角色权限
    if !hasPermission(access.Role, requiredRole) {
        return errors.New("权限不足")
    }

    return nil
}
```

#### 3. 权限层级

```go
var roleHierarchy = map[string]int{
    "owner":  4,
    "admin":  3,
    "editor": 2,
    "viewer": 1,
}

func hasPermission(userRole, requiredRole string) bool {
    return roleHierarchy[userRole] >= roleHierarchy[requiredRole]
}
```

### 前端实现

#### 1. 权限状态管理

```typescript
// RecordsView.vue
const userRole = ref('')
const databaseId = ref('')

// 权限判断
const canCreate = computed(() => ['owner', 'admin', 'editor'].includes(userRole.value))
const canEdit = computed(() => ['owner', 'admin', 'editor'].includes(userRole.value))
const canDelete = computed(() => ['owner', 'admin'].includes(userRole.value))
```

#### 2. 获取用户角色

```typescript
// 从数据库详情获取用户角色
const loadTableInfo = async () => {
    const response = await tableAPI.get(tableId)
    if (response.success && response.data) {
        tableName.value = response.data.name || ''
        databaseId.value = response.data.database_id || ''

        // 获取数据库角色
        if (databaseId.value) {
            const dbResponse = await databaseAPI.getDetail(databaseId.value)
            if (dbResponse.success && dbResponse.data) {
                userRole.value = dbResponse.data.role || 'viewer'
            }
        }
    }
}
```

#### 3. UI权限控制

```vue
<!-- 根据权限显示/隐藏按钮 -->
<el-button v-if="canCreate" type="primary" @click="handleCreate">
    新建记录
</el-button>

<el-button v-if="canEdit" size="small" @click="handleEdit(row)">
    编辑
</el-button>

<el-button v-if="canDelete" size="small" type="danger" @click="handleDelete(row)">
    删除
</el-button>
```

#### 4. 修改的文件

- `frontend/src/views/RecordsView.vue` - 记录管理权限控制
- `frontend/src/views/DatabasesView.vue` - 数据库管理权限控制
- `frontend/src/views/TableView.vue` - 表管理权限控制

---

## 使用指南

### 1. 创建数据库并分享

```typescript
// 1. 创建数据库（自动成为 Owner）
const db = await databaseAPI.create({
    name: '项目数据库',
    description: '项目相关数据',
    isPublic: false
})

// 2. 分享给其他用户（仅 Owner 和 Admin 可以）
await databaseAPI.share(db.id, {
    user_id: 'usr_xxx',
    role: 'editor'  // admin, editor, viewer
})
```

### 2. 配置字段级权限

```typescript
// 获取字段权限配置
const permissions = await fieldAPI.getPermissions(tableId)

// 批量设置权限
await fieldAPI.batchSetPermissions(tableId, {
    permissions: [
        {
            field_id: 'field_1',
            role: 'viewer',
            can_read: true,
            can_write: false,
            can_delete: false
        },
        {
            field_id: 'field_2',
            role: 'editor',
            can_read: true,
            can_write: true,
            can_delete: false
        }
    ]
})
```

### 3. 权限检查示例

```typescript
// 前端检查
if (canEdit.value) {
    // 显示编辑按钮
}

// 后端会再次验证
try {
    await recordAPI.update(recordId, data)
} catch (error) {
    // 如果权限不足，后端返回 403 错误
    ElMessage.error('权限不足')
}
```

---

## 测试报告

### 测试概览

- **测试日期**: 2026-01-11
- **测试工具**: Playwright MCP
- **测试结果**: ✅ 全部通过

### 测试场景

#### 1. 后端权限验证 ✅

**测试步骤**:
1. 创建 Viewer 用户
2. 授予数据库 Viewer 权限
3. 尝试编辑记录

**结果**:
- ✅ 后端返回 400 错误
- ✅ 正确阻止未授权操作

#### 2. 前端UI权限控制 ✅

**测试步骤**:
1. 以 Viewer 身份登录
2. 访问数据记录页面
3. 检查按钮显示

**结果**:
- ✅ 不显示"新建记录"按钮
- ✅ 不显示"编辑"按钮
- ✅ 不显示"删除"按钮
- ✅ 只显示查看功能

#### 3. 字段级权限配置 ✅

**测试步骤**:
1. 配置字段权限矩阵
2. 为 Viewer 配置写入权限
3. 保存配置

**结果**:
- ✅ 权限配置界面正常
- ✅ 权限保存成功
- ✅ 批量操作功能可用

### 测试统计

- **总测试项**: 12 项
- **通过**: 12 项 ✅
- **失败**: 0 项
- **成功率**: 100%

### 发现并修复的问题

1. **问题**: Viewer 用户能看到编辑/删除按钮
   - **原因**: 前端未根据角色隐藏按钮
   - **修复**: 添加 `v-if` 权限判断
   - **状态**: ✅ 已修复

2. **问题**: 数据库分享限制过严
   - **现状**: Owner 只能分享给 Owner
   - **说明**: 这是业务逻辑设计，需要通过 Admin 分享给其他角色
   - **状态**: ⚠️ 按设计工作

---

## 最佳实践

### 1. 权限分配原则

- **最小权限**: 默认给予最小必要权限
- **按需授权**: 根据实际需要逐步提升权限
- **定期审查**: 定期检查和调整权限配置

### 2. 角色选择建议

| 场景 | 推荐角色 |
|------|----------|
| 项目负责人 | Owner |
| 团队管理员 | Admin |
| 内容编辑 | Editor |
| 数据查看 | Viewer |
| 外部合作 | Viewer |
| 临时访问 | Viewer |

### 3. 安全建议

1. **双重验证**: 前端UI控制 + 后端API验证
2. **审计日志**: 记录所有权限变更（已实现，含单条/批量设置）
3. **定期审查**: 定期检查用户权限
4. **最小暴露**: 只显示用户有权限的功能

---

## API 参考

### 数据库权限 API

```typescript
// 分享数据库
POST /api/databases/:id/share
Body: { user_id: string, role: 'admin' | 'editor' | 'viewer' }

// 获取数据库用户列表
GET /api/databases/:id/users

// 移除数据库用户
DELETE /api/databases/:id/users/:user_id

// 更新用户角色
PUT /api/databases/:id/users/:user_id/role
Body: { role: 'admin' | 'editor' | 'viewer' }
```

### 字段权限 API

```typescript
// 获取字段权限
GET /api/tables/:tableId/field-permissions

// 设置字段权限
PUT /api/tables/:tableId/field-permissions
Body: { field_id, role, can_read, can_write, can_delete }

// 批量设置权限
PUT /api/tables/:tableId/field-permissions/batch
Body: { permissions: [...] }
```

---

## 常见问题

### Q: Owner 可以分享哪些数据库角色？

A: Owner 和 Admin 都可以授予 `admin`、`editor`、`viewer`；当前不支持通过分享或改角色接口创建额外的 `owner`，避免出现多个所有者。

### Q: 如何修改用户角色？

A: 使用 `PUT /api/databases/:id/users/:user_id/role` API，请求体只需要 `role`。当前只有 Owner 可以修改数据库成员角色。

### Q: 字段权限如何继承？

A: 字段权限继承数据库角色权限。如果没有配置字段级权限，则使用默认权限：
- Owner/Admin: R/W/D 全部权限
- Editor: R/W 权限
- Viewer: R 权限

### Q: 如何实现更细粒度的权限控制？

A: 可以通过字段级权限配置实现。在字段权限配置界面，可以为每个角色单独配置每个字段的 R/W/D 权限。

---

## 更新日志

### 2026-01-11

- ✅ 完成权限系统测试
- ✅ 修复前端权限UI显示问题
- ✅ 添加权限系统文档
- ✅ 验证三层权限模型

---

## 相关文档

- [API 文档](./API.md)
- [开发指南](./DEVELOPER-GUIDE.md)
- [E2E 测试报告](./E2E-TEST-REPORT.md)
- [项目状态](./PROJECT-STATUS.md)
