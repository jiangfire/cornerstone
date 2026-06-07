[English](MCP-Setup.md) | [中文](MCP-Setup.zh.md)

# MCP 客户端配置

> 将 Cornerstone 接入支持 MCP 协议的 AI 客户端。

---

## 支持的客户端

- [Claude Desktop](https://claude.ai/download)
- [Cline](https://github.com/cline/cline) (VS Code 插件)
- [其他 SSE MCP 客户端](https://modelcontextprotocol.io/clients)

---

## 连接方式

Cornerstone 提供两种传输方式：

| 方式 | 端点 | 说明 |
|------|------|------|
| **SSE 事件流** | `GET /mcp` | 长连接，适合实时交互 |
| **JSON-RPC** | `POST /mcp` | 请求/响应，适合简单调用 |

认证方式：所有请求需携带 `Authorization: Bearer <token>` 头。

---

## Claude Desktop 配置

在 Claude Desktop 的 `claude_desktop_config.json` 中添加：

### macOS

```json
{
  "mcpServers": {
    "cornerstone": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sse"],
      "env": {
        "SSE_URL": "http://localhost:8080/mcp",
        "AUTH_TOKEN": "cs_your_token"
      }
    }
  }
}
```

配置文件路径：`~/Library/Application Support/Claude/claude_desktop_config.json`

### Windows

```json
{
  "mcpServers": {
    "cornerstone": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sse"],
      "env": {
        "SSE_URL": "http://localhost:8080/mcp",
        "AUTH_TOKEN": "cs_your_token"
      }
    }
  }
}
```

配置文件路径：`%APPDATA%\Claude\claude_desktop_config.json`

### 重启 Claude Desktop

保存配置后重启 Claude Desktop，左侧边栏应该出现 **锯子图标** → 点击后可见 Cornerstone 工具列表。

---

## Cline (VS Code) 配置

在 Cline 的 MCP 设置中添加：

```json
{
  "mcpServers": [
    {
      "name": "cornerstone",
      "transport": "sse",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer cs_your_token"
      }
    }
  ]
}
```

---

## 可用工具列表

接入后，AI 客户端可以调用以下工具：

### 数据库管理
- `create_database` - 创建数据库
- `list_databases` - 列出数据库
- `get_database` - 获取数据库详情
- `update_database` - 更新数据库
- `delete_database` - 删除数据库
- `create_database_with_tables` - 一键创建数据库+表+字段

### 表管理
- `create_table` - 创建表
- `list_tables` - 列出表
- `get_table` - 获取表详情
- `update_table` - 更新表
- `delete_table` - 删除表

### 字段管理
- `create_field` - 创建字段
- `list_fields` - 列出字段
- `update_field` - 更新字段
- `delete_field` - 删除字段

### 记录管理
- `insert_record` - 插入记录
- `list_records` - 列出记录（分页）
- `get_record` - 获取单条记录
- `update_record` - 更新记录
- `delete_record` - 删除记录
- `batch_insert_records` - 批量插入记录
- `generate_test_data` - 生成测试数据

### 查询
- `query_data` - Query DSL 查询
- `get_table_schema` - 获取系统表字段 Schema

---

## SSE 流特性

### 心跳保活

SSE 流每 25 秒发送一次 keepalive 注释，确保连接不被代理/网关超时断开。

可通过环境变量调整：

```bash
MCP_SSE_KEEPALIVE_SEC=25
```

### 断线重连

支持通过 `Last-Event-ID` 头实现断线重连和消息重放：

```
GET /mcp
Accept: text/event-stream
Last-Event-ID: <event-id>
```

重放缓冲区默认 128 条消息，可通过环境变量调整：

```bash
MCP_SSE_REPLAY_BUFFER=128
```

### 重试间隔

SSE 流的重试间隔默认 3000ms，可通过环境变量调整：

```bash
MCP_SSE_RETRY_MS=3000
```

---

## 跨域配置

如果 AI 客户端和 Cornerstone 运行在不同域，配置允许的来源：

```bash
MCP_ALLOWED_ORIGINS=https://claude.ai,https://app.claude.ai
```

留空则允许任何来源（仅建议开发环境）。

---

## 故障排查

| 问题 | 原因 | 解决 |
|------|------|------|
| 客户端无法连接 | 服务未启动或端口被阻止 | 确认 `cornerstone serve` 已运行，检查防火墙 |
| 401 Unauthorized | Token 无效或缺失 | 确认 `Authorization: Bearer <token>` 正确 |
| 工具列表为空 | SSE 流未正确建立 | 检查 `Accept: text/event-stream` 头是否正确 |
| 无法执行操作 | Token 权限不足 | 检查 Token 的 Scope 是否包含目标资源 |
| SSE 流经常断开 | 代理超时 | 增大 `MCP_SSE_KEEPALIVE_SEC`，确保代理不关闭长连接 |

---

## 相关文档

- [AI Assistant](AI-Assistant.md) - HTTP API 方式调用 AI
- [Token Scopes](TokenScopes.md) - 控制 AI 客户端的访问权限
- [Architecture](Architecture.md) - MCP 协议在系统中的位置
