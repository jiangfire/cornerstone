# Cornerstone API 文档

**最后校对**: 2026-03-28  
**基准代码**: `backend/cmd/server/main.go`、`backend/internal/handlers/*`、`backend/pkg/query/*`  
**基础地址**: `http://localhost:8080`  
**路由真值**: 以 [backend/cmd/server/main.go](../backend/cmd/server/main.go) 为准

## 1. 总览

Cornerstone 当前提供四类接口：

| 类别 | 路径前缀 | 认证方式 | 用途 |
|---|---|---|---|
| 公共接口 | `/health`、`/api/auth/*` | 无 / JWT | 健康检查、注册、登录 |
| 业务接口 | `/api/*` | JWT | 用户、组织、数据库、记录、治理、查询 DSL |
| 系统集成接口 | `/api/integrations/*` | Integration Token（系统间令牌） | 接收入站治理事件 |
| HTTP MCP 接口 | `/mcp` | JWT + Origin 校验 | 通过 MCP 查询和创建数据库，并通过 SSE 接收业务变更通知 |

补充说明：

- `/api/v1/*` 已做兼容转发到 `/api/*`。
- 成功响应统一为 `{"success": true, "data": ...}`。
- 失败响应统一为 `{"success": false, "message": "...", "code": 4xx/5xx}`。

## 2. 认证约定

### 2.1 JWT 认证

除 `/health`、`/api/auth/register`、`/api/auth/login` 外，其余业务接口默认要求：

```http
Authorization: Bearer <jwt-token>
```

相关接口：

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/auth/register` | 注册 |
| `POST` | `/api/auth/login` | 登录 |
| `POST` | `/api/auth/logout` | 登出并拉黑当前 token |
| `GET` | `/api/users/me` | 获取当前用户 |

### 2.2 集成令牌认证

`/api/integrations/events` 不走 JWT，而是使用系统间令牌：

```http
X-Source-System: fuckcmdb
Authorization: Bearer <integration-token>
```

鉴权优先级：

1. `INTEGRATION_SHARED_TOKEN`
2. `INTEGRATION_TOKENS` 中按 `sourceSystem=token` 匹配

### 2.3 HTTP MCP 认证

`/mcp` 使用 JWT 登录态，而不是系统集成 token：

```http
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

补充说明：

- `OPTIONS /mcp` 不要求 JWT，用于预检。
- 若请求携带 `Origin`，服务会校验其是否与当前 Host 一致，或命中 `MCP_ALLOWED_ORIGINS`。
- `/mcp` 返回 **JSON-RPC / MCP 响应**，不使用业务 API 的统一 `{success,data}` 包装。

## 3. 响应格式

### 3.1 成功响应

```json
{
  "success": true,
  "data": {}
}
```

### 3.2 失败响应

```json
{
  "success": false,
  "message": "错误描述",
  "code": 400
}
```

常见状态码：

| 状态码 | 含义 |
|---|---|
| `200` | 成功 |
| `400` | 参数错误或业务校验失败 |
| `401` | 未认证或认证失败 |
| `403` | 有 token 但权限不足 |
| `404` | 资源不存在 |
| `500` | 服务端错误 |

## 4. 路由清单

### 4.1 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/health` | 服务健康检查，返回 `status/service/version/time` |

### 4.2 用户与认证

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/auth/register` | 注册 |
| `POST` | `/api/auth/login` | 登录 |
| `POST` | `/api/auth/logout` | 登出 |
| `GET` | `/api/users/me` | 当前用户资料 |
| `PUT` | `/api/users/me` | 更新个人资料 |
| `PUT` | `/api/users/me/password` | 修改密码 |
| `DELETE` | `/api/users/me` | 注销账号 |
| `GET` | `/api/users` | 列出用户，可按组织/数据库过滤 |
| `GET` | `/api/users/search` | 搜索用户 |

### 4.3 组织

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/organizations` | 创建组织 |
| `GET` | `/api/organizations` | 组织列表 |
| `GET` | `/api/organizations/:id` | 组织详情 |
| `PUT` | `/api/organizations/:id` | 更新组织 |
| `DELETE` | `/api/organizations/:id` | 删除组织 |
| `GET` | `/api/organizations/:id/members` | 成员列表 |
| `POST` | `/api/organizations/:id/members` | 添加成员，`role` 仅允许 `admin`/`member` |
| `DELETE` | `/api/organizations/:id/members/:member_id` | 移除成员 |
| `PUT` | `/api/organizations/:id/members/:member_id/role` | 更新成员角色，`role` 仅允许 `admin`/`member` |

### 4.4 数据库、表、字段、字段权限

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/databases` | 创建数据库 |
| `GET` | `/api/databases` | 数据库列表 |
| `GET` | `/api/databases/:id` | 数据库详情 |
| `PUT` | `/api/databases/:id` | 更新数据库 |
| `DELETE` | `/api/databases/:id` | 删除数据库 |
| `POST` | `/api/databases/:id/share` | 分享数据库，`role` 仅允许 `admin`/`editor`/`viewer` |
| `GET` | `/api/databases/:id/users` | 数据库成员 |
| `DELETE` | `/api/databases/:id/users/:user_id` | 移除数据库成员 |
| `PUT` | `/api/databases/:id/users/:user_id/role` | 更新数据库角色，body 仅需 `role`，且仅允许 `admin`/`editor`/`viewer` |
| `POST` | `/api/tables` | 创建表 |
| `GET` | `/api/databases/:id/tables` | 数据库下表列表 |
| `GET` | `/api/tables/:id` | 表详情 |
| `PUT` | `/api/tables/:id` | 更新表 |
| `DELETE` | `/api/tables/:id` | 删除表 |
| `POST` | `/api/fields` | 创建字段 |
| `GET` | `/api/tables/:id/fields` | 表字段列表 |
| `GET` | `/api/fields/:id` | 字段详情 |
| `PUT` | `/api/fields/:id` | 更新字段 |
| `DELETE` | `/api/fields/:id` | 删除字段 |
| `GET` | `/api/tables/:id/field-permissions` | 获取字段权限矩阵 |
| `PUT` | `/api/tables/:id/field-permissions` | 设置单条字段权限 |
| `PUT` | `/api/tables/:id/field-permissions/batch` | 批量设置字段权限 |

### 4.5 记录与文件

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/records` | 创建记录 |
| `GET` | `/api/records` | 记录列表 |
| `GET` | `/api/records/export` | 导出记录 |
| `GET` | `/api/records/:id` | 记录详情 |
| `PUT` | `/api/records/:id` | 更新记录 |
| `DELETE` | `/api/records/:id` | 删除记录 |
| `POST` | `/api/records/batch` | 批量创建记录 |
| `POST` | `/api/files/upload` | 上传附件 |
| `GET` | `/api/files/:id` | 获取文件元数据 |
| `GET` | `/api/files/:id/download` | 下载文件 |
| `DELETE` | `/api/files/:id` | 删除文件 |
| `GET` | `/api/records/:id/files` | 记录附件列表 |

### 4.6 插件、统计、系统设置

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/plugins` | 创建插件 |
| `GET` | `/api/plugins` | 插件列表 |
| `GET` | `/api/plugins/:id` | 插件详情 |
| `PUT` | `/api/plugins/:id` | 更新插件 |
| `DELETE` | `/api/plugins/:id` | 删除插件 |
| `POST` | `/api/plugins/:id/bind` | 绑定插件到表 |
| `DELETE` | `/api/plugins/:id/unbind` | 解绑插件 |
| `GET` | `/api/plugins/:id/bindings` | 插件绑定列表 |
| `POST` | `/api/plugins/:id/execute` | 手动执行插件 |
| `GET` | `/api/plugins/:id/executions` | 插件执行记录 |
| `GET` | `/api/stats/summary` | 统计汇总 |
| `GET` | `/api/stats/activities` | 最近活动 |
| `GET` | `/api/settings` | 读取系统设置 |
| `PUT` | `/api/settings` | 更新系统设置 |

### 4.7 Query DSL（查询 DSL，面向受控查询的 JSON 查询语言）

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET`/`POST` | `/api/query` | 执行完整 DSL 查询 |
| `GET` | `/api/query/simple` | 简化查询接口 |
| `POST` | `/api/query/batch` | 批量查询 |
| `POST` | `/api/query/explain` | 返回权限收口后的 SQL |
| `POST` | `/api/query/validate` | 只做校验，不执行 |
| `GET` | `/api/query/tables` | 当前用户可访问的 DSL 表清单 |
| `GET` | `/api/query/schema/:table` | 指定表的可访问字段清单 |

关键限制：

- 不是任意 SQL 执行器，只允许访问白名单表与字段。
- `users.password` 明确禁止查询。
- `database_access`、`field_permissions` 需要数据库 `owner/admin` 权限。
- `records`、`tables`、`fields`、`files`、`plugin_bindings`、`plugin_executions` 会自动按当前用户权限补充过滤条件。
- 默认限制来自 `backend/pkg/query/model.go`：
  - 最多 `3` 个 `JOIN`
  - 最大页大小 `1000`
  - 最大嵌套深度 `5`
  - 最大字段数 `100`

完整 DSL 示例：

```json
{
  "from": "tables",
  "select": ["id", "database_id", "name", "created_at"],
  "where": {
    "and": [
      {
        "field": "database_id",
        "op": "eq",
        "value": "db_xxx"
      }
    ]
  },
  "orderBy": [
    {
      "field": "created_at",
      "dir": "desc"
    }
  ],
  "page": 1,
  "size": 20
}
```

简化查询示例：

```http
GET /api/query/simple?table=tables&filter={"database_id":"db_xxx"}&sort=-created_at&page=1&size=20
```

### 4.8 治理域

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/governance/tasks` | 创建治理任务 |
| `GET` | `/api/governance/tasks` | 查询治理任务列表 |
| `GET` | `/api/governance/tasks/:id` | 治理任务详情 |
| `PUT` | `/api/governance/tasks/:id` | 更新治理任务 |
| `POST` | `/api/governance/tasks/:id/evidences` | 添加整改证据 |
| `POST` | `/api/governance/tasks/:id/comments` | 添加评论 |
| `POST` | `/api/governance/reviews` | 发起治理审核 |
| `GET` | `/api/governance/reviews/:id` | 查询治理审核详情 |
| `POST` | `/api/governance/reviews/:id/approve` | 审核通过 |
| `POST` | `/api/governance/reviews/:id/reject` | 审核驳回 |
| `POST` | `/api/governance/reviews/:id/apply` | 触发或重试审核回写 |

当前已实现的治理行为：

- 入站事件可自动建单。
- 自动建单对当前登录用户可见，并可进入详情、评论、补证据、发起审核。
- 已通过审核可触发回写；支持 `term_binding`、`classification`、`dq_rule` 三类审核回写。
- 回写通过 outbox（出站待发送队列）执行，支持状态流转、重试和终止。

治理任务详情返回结构：

```json
{
  "task": {},
  "reviews": [],
  "evidences": [],
  "comments": [],
  "external_links": []
}
```

创建治理审核示例：

```json
{
  "task_id": "gvt_xxx",
  "review_type": "term_binding",
  "reviewer_id": "usr_xxx",
  "proposal_source": "manual",
  "proposal_payload": "{\"summary\":\"确认 panel_id 的术语绑定建议\"}"
}
```

审核结论示例：

```json
{
  "decision_payload": "{\"decision\":\"approved\",\"note\":\"确认建议可落地\"}"
}
```

### 4.9 集成事件

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/integrations/events` | 接收入站治理事件 |

请求头：

```http
X-Source-System: fuckcmdb
Authorization: Bearer <integration-token>
Content-Type: application/json
```

请求体示例：

```json
{
  "event_id": "evt_20260324_001",
  "event_type": "metadata.schema.changed",
  "occurred_at": "2026-03-24T09:00:00Z",
  "resource_type": "column",
  "resource_id": "col_panel_id",
  "actor_id": "system",
  "trace_id": "trace_123",
  "payload": {
    "change_type": "column_added",
    "display_name": "panel_id",
    "summary": "新增字段需要治理确认"
  }
}
```

当前支持自动建单的事件类型：

| 事件类型 | 生成任务类型 |
|---|---|
| `dq.alert.triggered` | `dq_issue` |
| `dq.rule.failed` | `dq_issue` |
| `metadata.schema.changed` | `schema_change` |
| `ai.recommendation.generated` + `recommendation_type=term_binding` | `term_review` |
| `ai.recommendation.generated` + `recommendation_type=classification` | `classification_review` |

幂等性说明：

- 幂等键为 `source_system + event_id`。
- 同一来源系统下，相同 `event_id` 重放不会重复建单，而是返回既有入站事件结果。
- 不同来源系统可使用相同 `event_id`，不会互相冲突。

### 4.10 HTTP MCP

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/mcp` | 执行 MCP JSON-RPC 请求 |
| `GET` | `/mcp` | 当 `Accept: text/event-stream` 时建立并维持 SSE 流 |
| `OPTIONS` | `/mcp` | 预检请求 |

当前已提供的 MCP tools：

| Tool | 说明 |
|---|---|
| `query_data` | 执行受权限约束的 Query DSL 查询 |
| `create_database` | 创建数据库 |
| `list_databases` | 列出当前用户可访问数据库 |
| `get_table_schema` | 获取 Query DSL 表字段清单 |

初始化示例：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-03-26"
  }
}
```

查询工具调用示例：

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "query_data",
    "arguments": {
      "query": {
        "from": "users",
        "select": ["id", "username", "email"],
        "page": 1,
        "size": 20
      }
    }
  }
}
```

建库工具调用示例：

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "create_database",
    "arguments": {
      "name": "MCP Created DB",
      "description": "created via HTTP MCP",
      "is_public": false,
      "is_personal": true
    }
  }
}
```

SSE 调用说明：

- `POST /mcp` 默认返回普通 JSON-RPC 响应。
- 若请求头包含 `Accept: text/event-stream`，服务会按 SSE 输出该次请求的响应事件。
- `GET /mcp` 也支持 SSE；若未提供 `Accept: text/event-stream`，当前返回 `406`。
- `GET /mcp` 建链后会持续保持连接，并定期发送 keepalive 注释帧；间隔由 `MCP_SSE_KEEPALIVE_SEC` 控制。
- `GET /mcp` 会输出 SSE `retry:` 提示，建议客户端按 `MCP_SSE_RETRY_MS` 重连。
- `GET /mcp` 支持 `Last-Event-ID`，可在缓冲窗口内重放断线期间遗漏的通知。
- `GET /mcp` 建链后会收到 `notifications/stream/connected`；带有效 `Last-Event-ID` 时会收到 `notifications/stream/resumed`；无法重放时会收到 `notifications/stream/replay_unavailable`。
- `create_database` 成功后，当前用户已建立的 `GET /mcp` SSE 流会收到 `notifications/databases/changed` 主动通知。
- 业务 REST 成功路径当前也会复用同一条 SSE 通道下发通知，已覆盖数据库、表、字段、治理任务、治理审核等主要变更事件。
- 治理类通知会按参与人投递，当前覆盖任务创建者、负责人、审核人和当前操作者；不同用户之间的 SSE 历史与重放缓冲相互隔离。
- SSE 请求会在 handler 内局部关闭写超时，以保证流式连接不被截断；普通 API 仍保留服务端默认写超时保护。
- 空 batch 请求 `[]` 当前会被直接拒绝为无效请求，不再返回 `202`。

## 5. 配置项

### 5.1 通用服务配置

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `DB_TYPE` | `sqlite` | `sqlite` 或 `postgres` |
| `DATABASE_URL` | `./cornerstone.db` | 数据库连接串或 SQLite 文件路径 |
| `PORT` | `8080` | 服务端口 |
| `SERVER_MODE` | `release` | Gin 模式 |
| `JWT_SECRET` | 内置默认值 | 生产环境必须替换 |
| `JWT_EXPIRATION` | `24` | JWT 过期小时数 |
| `MCP_ALLOWED_ORIGINS` | 空 | `/mcp` 浏览器 Origin 白名单，多个值逗号分隔 |
| `MCP_SSE_KEEPALIVE_SEC` | `25` | `/mcp` SSE keepalive 间隔（秒） |
| `MCP_SSE_RETRY_MS` | `3000` | `/mcp` SSE `retry:` 提示（毫秒） |
| `MCP_SSE_REPLAY_BUFFER` | `128` | `/mcp` 每用户 Last-Event-ID 重放缓冲条数 |

### 5.2 集成与治理回写配置

| 环境变量 | 说明 |
|---|---|
| `INTEGRATION_SHARED_TOKEN` | 入站/出站共享 token |
| `INTEGRATION_TOKENS` | 入站 source 级 token，格式 `fuckcmdb=tokenA,other=tokenB` |
| `INTEGRATION_BASE_URLS` | 出站目标系统 base URL，格式 `fuckcmdb=https://host` |
| `OUTBOUND_INTEGRATION_TOKENS` | 出站目标系统 token，格式 `fuckcmdb=tokenA` |
| `OUTBOUND_INTEGRATION_TIMEOUT_SEC` | 出站 HTTP 超时秒数或 Go duration |
| `GOVERNANCE_OUTBOX_MAX_RETRIES` | 回写最大重试次数 |
| `GOVERNANCE_OUTBOX_RETRY_INTERVAL_SEC` | 回写基础重试间隔 |
| `GOVERNANCE_OUTBOX_WORKER_ENABLED` | 是否启用后台 outbox worker |
| `INTEGRATION_UI_BASE_URLS` | 外部资源页面 URL 映射，用于详情外链 |
| `FUCKCMDB_UI_BASE_URL` | `fuckcmdb` UI 链接 fallback |
| `FUCKCMDB_BASE_URL` | `fuckcmdb` API base URL fallback |

## 6. 数据模型摘要

当前迁移会自动创建以下关键表：

| 类别 | 关键表 |
|---|---|
| 核心业务 | `users`、`organizations`、`organization_members`、`databases`、`database_access`、`tables`、`fields`、`records` |
| 权限与系统 | `field_permissions`、`token_blacklist`、`app_settings` |
| 文件与插件 | `files`、`plugins`、`plugin_bindings`、`plugin_executions` |
| 统计与审计 | `activity_logs` |
| 治理域 | `governance_tasks`、`governance_reviews`、`governance_evidences`、`governance_external_links`、`governance_comments` |
| 系统集成 | `integration_inbound_events`、`governance_outbox_events` |

补充说明：

- PostgreSQL 下会创建 `user_database_permissions` 物化视图。
- SQLite 不支持物化视图，因此该视图不会创建。

## 7. 已知边界

- 本文只描述当前仓库内已存在的服务接口，不代表下游系统 `fuckcmdb` 的真实接口契约。
- 治理回写虽然已实现 outbox、重试和状态流转，但下游路径是否完全兼容，仍需双系统联调确认。
- `/mcp` 当前是 HTTP 简化实现，已支持基于 SSE 的流式返回、断线重放和 server-initiated notifications；当前业务通知已覆盖数据库、表、字段、治理任务、治理审核，但记录、文件、插件等事件仍未接入。
- 若文档与实现不一致，优先以路由文件和 handler/service 代码为准。
