[English](TokenScopes.md) | [中文](TokenScopes.zh.md)

# Token 作用域

> 控制 Token 对数据库、表和字段的访问权限。

---

## 概述

Cornerstone 的权限系统基于 **Token + Scope** 模型：

- **Master Token**：拥有完整权限；在启动时自动生成，或通过 `MASTER_TOKEN` 预设
- **普通 Token**：通过 API 或 CLI 创建；必须配置 Scope 以限制其访问范围

Scope 是一个 JSON 对象，定义了 Token 可以访问哪些数据库和表，以及它在这些资源上的角色。

---

## Scope 格式

```json
{
  "databases": {
    "db_xxx": "admin",
    "db_yyy": "editor"
  },
  "tables": {
    "tbl_aaa": {
      "role": "viewer",
      "fields": {
        "fld_name": ["read"],
        "fld_email": ["read", "write"]
      }
    }
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `databases` | `map[string]string` | 数据库 ID -> 角色。角色可以是 `viewer`/`editor`/`admin` |
| `tables` | `map[string]TableScope` | 表 ID -> 表级权限配置 |
| `tables[table_id].role` | `string` | 该表上的角色 |
| `tables[table_id].fields` | `map[string][]string` | 字段级权限（可选）；字段 ID/名称 -> 操作列表 |

### 角色权限

| 角色 | read | write | delete | manage |
|------|:----:|:-----:|:------:|:------:|
| `viewer` | ✅ | ❌ | ❌ | ❌ |
| `editor` | ✅ | ✅ | ❌ | ❌ |
| `admin` | ✅ | ✅ | ✅ | ✅ |

> `manage` 包含更新/删除资源本身（例如，修改表结构、删除数据库）。

---

## 权限继承规则

1. **Master Token** 始终拥有完整权限
2. **表权限**可以独立设置；如未设置，则继承自所属数据库的权限
3. **字段权限**可以进一步细化访问控制；如未设置，则继承自表的权限
4. **操作匹配**不区分大小写；`read`/`READ`/`Read` 是等效的

### 继承示例

```json
{
  "databases": {
    "db_project": "editor"
  },
  "tables": {
    "tbl_users": {
      "role": "viewer"
    }
  }
}
```

- `db_project` 下的所有表：默认 `editor`（读取/写入）
- `tbl_users`：显式设置为 `viewer`（只读），覆盖数据库级别的 `editor`

---

## 创建带 Scope 的 Token

### CLI

```bash
# 直接写入 JSON scope（注意外层引号）
cornerstone token create "dev-team" \
  -s '{"databases":{"db_xxx":"editor"}}'

# 或从文件读取
cornerstone token create "dev-team" -s "$(cat scope.json)"
```

### REST API

```bash
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "Authorization: Bearer cs_master_token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dev-team",
    "scopes": "{\"databases\":{\"db_xxx\":\"editor\"}}"
  }'
```

> 在 API 中，`scopes` 字段是一个**字符串**（JSON 序列化的 Scope 对象），而不是对象。

---

## 字段级权限

要将 Token 限制为仅访问特定字段：

```json
{
  "tables": {
    "tbl_users": {
      "role": "viewer",
      "fields": {
        "fld_name": ["read"],
        "fld_email": ["read"]
      }
    }
  }
}
```

该 Token 对 `tbl_users` 的查询将只返回 `fld_name` 和 `fld_email`；所有其他字段均不可见。

字段键可以是**字段 ID**（例如 `fld_xxx`）或**字段名称**（例如 `name`）。

---

## 最佳实践

1. **最小权限原则**：仅授予 Token 完成其任务所需的最低权限
2. **数据库级默认值**：先在数据库级别分配默认角色，然后对敏感表进行降级
3. **字段级脱敏**：单独限制包含敏感信息的字段（例如手机号、身份证号）
4. **Token 轮换**：定期删除旧 Token 并创建新 Token
5. **过期时间**：为临时/特定场景的 Token 设置 `expires_at`，避免长期有效

---

## 故障排查

| 问题 | 原因 | 解决方案 |
|------|------|------|
| `permission denied: cannot access this database` | Token 没有该数据库的 scope | 检查 `scopes.databases` 是否包含目标数据库 ID |
| `permission denied: cannot access this table` | Token 没有该表的 scope，且数据库级权限不足 | 将其添加到 `scopes.tables` 或 `scopes.databases` 中 |
| `field 'xxx' is not in the allowed list` | Query DSL 请求了未授权的字段 | 检查该字段是否在 scope 的 `fields` 白名单中 |
| `master token required for this operation` | 该操作需要 Master Token（例如创建数据库） | 使用 Master Token，或将目标 Token 提升为 Master（不推荐） |

---

## 相关文档

- [Query DSL](Query.md) - 查询时的权限校验逻辑
- [REST API](README.md#rest-api) - Token 管理接口
