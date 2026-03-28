# Cornerstone HTTP MCP

**最后校对**: 2026-03-28

## 目标

Cornerstone 现在提供一个最小可用的 HTTP 版 MCP（Model Context Protocol）端点，可让 MCP 客户端在**当前登录用户上下文**下调用受权限约束的数据库管理与查询能力。

## 启动方式

启动现有后端服务即可：

```powershell
cd backend
go run ./cmd/server/main.go
```

MCP endpoint：

```text
POST    /mcp
GET     /mcp
OPTIONS /mcp
```

说明：

- 当前只提供 **HTTP 版 MCP**，不提供 CLI / `stdio` 版 MCP。
- `POST /mcp` 用于 JSON-RPC 调用；`GET /mcp` 用于建立 SSE 长连接并接收主动通知。
- MCP 能力默认复用当前 Cornerstone 服务的认证、权限和业务服务层，不单独维护第二套权限模型。

认证方式：

```http
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

可选安全配置：

| 环境变量 | 说明 |
|---|---|
| `MCP_ALLOWED_ORIGINS` | 允许访问 `/mcp` 的浏览器 `Origin` 白名单，多个值用逗号分隔 |
| `MCP_SSE_KEEPALIVE_SEC` | SSE keepalive 注释帧间隔，单位秒 |
| `MCP_SSE_RETRY_MS` | SSE `retry:` 建议间隔，单位毫秒 |
| `MCP_SSE_REPLAY_BUFFER` | 每个用户保留的 Last-Event-ID 重放缓冲条数 |

## 当前提供的 Tools

| Tool | 说明 |
|---|---|
| `query_data` | 执行 Cornerstone Query DSL 查询 |
| `create_database` | 创建数据库 |
| `list_databases` | 列出当前用户可访问数据库 |
| `get_table_schema` | 获取 Query DSL 可访问表的字段清单 |

当前没有暴露表、字段、记录的通用写入 tool；这是有意收口，不是遗漏。

## Tool 参数

### `query_data`

参数示例：

```json
{
  "query": {
    "from": "users",
    "select": ["id", "username", "email"],
    "page": 1,
    "size": 20
  }
}
```

说明：

- 使用现有 Query DSL 执行器。
- 会复用现有白名单、权限验证和自动过滤逻辑。
- `select: ["*"]` 会自动展开为允许字段，不会返回 `users.password`。

### `create_database`

参数示例：

```json
{
  "name": "MCP Created DB",
  "description": "created from MCP",
  "is_public": false,
  "is_personal": true
}
```

### `list_databases`

参数示例：

```json
{}
```

### `get_table_schema`

参数示例：

```json
{
  "table": "records"
}
```

## 权限边界

- 所有 tool 调用都绑定到当前 JWT 对应的 Cornerstone 用户。
- `query_data` 不是任意 SQL 执行器，只能访问 Query DSL 白名单中的表和字段。
- `create_database` 会校验当前用户是否真实存在，并按现有服务逻辑创建所有者权限。

## 当前边界

- 当前是 **Streamable HTTP 简化实现**：
  - `POST /mcp` 支持普通 JSON 响应
  - 当客户端发送 `Accept: text/event-stream` 时，`POST /mcp` 会以 SSE 返回本次请求的 JSON-RPC 响应
  - 当客户端发送 `Accept: text/event-stream` 时，`GET /mcp` 可建立 SSE 长连接
  - `GET /mcp` 会维持 SSE 流并发送 keepalive 注释帧，间隔可由 `MCP_SSE_KEEPALIVE_SEC` 配置
  - `GET /mcp` 会输出 `retry:` 建议间隔，供客户端断线重连时参考
  - `GET /mcp` 支持通过 `Last-Event-ID` 做断线续传，重放窗口由 `MCP_SSE_REPLAY_BUFFER` 控制
  - 当数据库、表、字段、治理任务、治理审核等相关变更成功落地后，已建立的 `GET /mcp` SSE 流会收到对应主动通知
  - SSE 请求会按请求级别清除写超时，普通业务 API 仍保留服务端默认写超时保护
  - 空 batch 请求 `[]` 会被直接拒绝，不会再被错误接受为 `202`
- 若请求携带 `Origin`，服务会校验其是否与当前 Host 一致，或命中 `MCP_ALLOWED_ORIGINS`。
- 当前 focus 在“查询”和“创建数据库”最小闭环，没有暴露表、字段、记录的写入工具。

## 当前通知覆盖范围

截至 2026-03-28，SSE 主动通知已覆盖以下主链：

- 数据库创建、更新
- 表创建、更新
- 字段创建、更新
- 治理任务创建、更新、审核状态联动
- 治理审核创建、通过、驳回、回写触发

以下业务仍**未**接入 MCP SSE 主动通知：

- 记录 CRUD
- 文件上传 / 删除
- 插件绑定 / 执行
- 组织、成员和用户资料类事件

## SSE 主动通知

当前已实现的服务端主动通知：

| 方法 | 触发时机 | 说明 |
|---|---|---|
| `notifications/stream/connected` | `GET /mcp` 建链成功后 | 下发当前 stream 元信息、keepalive 与 retry 配置 |
| `notifications/stream/resumed` | `GET /mcp` 携带有效 `Last-Event-ID` 时 | 告知客户端本次恢复状态以及回放条数 |
| `notifications/stream/replay_unavailable` | `GET /mcp` 携带无法回放的 `Last-Event-ID` 时 | 告知客户端指定事件已不在缓冲窗口内 |
| `notifications/databases/changed` | MCP 建库或 REST 数据库创建/更新成功后 | 向当前用户已建立的 `GET /mcp` SSE 流推送数据库变更 |
| `notifications/tables/changed` | REST 表创建/更新成功后 | 向当前用户已建立的 `GET /mcp` SSE 流推送表变更 |
| `notifications/fields/changed` | REST 字段创建/更新成功后 | 向当前用户已建立的 `GET /mcp` SSE 流推送字段变更 |
| `notifications/governance/tasks/changed` | 治理任务创建/更新、审核状态联动成功后 | 向任务参与人（创建者、负责人、当前操作者）推送任务变更 |
| `notifications/governance/reviews/changed` | 治理审核创建、通过、驳回、回写触发成功后 | 向审核参与人（发起人、审核人、任务参与人）推送审核变更 |

当前通知受众规则：

- 数据库、表、字段类通知按当前成功操作用户投递。
- 治理类通知按参与人投递，当前覆盖创建者、负责人、审核人和当前操作者。
- SSE 历史缓冲按用户维度隔离，不同用户之间不会共享 `Last-Event-ID` 重放历史。

## 验证建议

若要验证当前 HTTP MCP 能力，建议至少做下面 4 项：

1. 用有效 JWT 调用 `POST /mcp` 的 `query_data`，确认 Query DSL 权限过滤生效。
2. 用有效 JWT 调用 `POST /mcp` 的 `create_database`，确认数据库创建成功且当前用户自动成为 owner。
3. 用 `Accept: text/event-stream` 建立 `GET /mcp` 长连接，再通过 REST 或 MCP 创建数据库 / 修改表字段，确认能收到主动通知。
4. 携带有效 `Last-Event-ID` 重连 `GET /mcp`，确认恢复、回放和缓冲失效提示行为符合预期。

恢复请求示例：

```http
GET /mcp
Accept: text/event-stream
Authorization: Bearer <jwt-token>
Last-Event-ID: 2cfa3d16-5d36-4b25-8b3f-9ed1323d0f10
```

通知示例：

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/databases/changed",
  "params": {
    "action": "created",
    "database": {
      "id": "db_xxx",
      "name": "MCP Created DB",
      "description": "created from MCP",
      "is_public": false,
      "is_personal": true,
      "owner_id": "usr_xxx"
    }
  }
}
```
