# Cornerstone

> 轻量数据资产平台：CLI + REST API + AI 助手 + MCP 协议

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

---

## 安装

### Docker（推荐）

```bash
docker compose up -d --build
```

### 下载二进制

从 [Releases](https://github.com/jiangfire/cornerstone/releases) 下载对应平台的二进制文件。

---

## 配置

复制 `.env.example` 为 `.env`，按需修改：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DB_TYPE` | `sqlite` 或 `postgres` | `sqlite` |
| `DATABASE_URL` | 数据库连接串 | `./cornerstone.db` |
| `DB_MAX_OPEN` | 数据库最大打开连接数 | `25` |
| `DB_MAX_IDLE` | 数据库最大空闲连接数 | `5` |
| `DB_MAX_LIFETIME` | 连接最大生命周期 | `5m` |
| `SERVER_MODE` | `release` 或 `debug` | `release` |
| `PORT` | 服务端口 | `8080` |
| `LOG_LEVEL` | 日志级别 | `info` |
| `MASTER_TOKEN` | Master Token（留空则自动生成） | - |
| `LLM_API_KEY` | LLM API Key（启用 AI 助手） | - |
| `LLM_MODEL` | LLM 模型名 | `gpt-4o` |
| `LLM_BASE_URL` | 自定义 LLM API 地址 | - |
| `MCP_ALLOWED_ORIGINS` | MCP 允许的来源（逗号分隔） | `*` |
| `MCP_SSE_KEEPALIVE_SEC` | SSE 心跳间隔（秒） | `30` |
| `MCP_SSE_RETRY_MS` | SSE 重连间隔（毫秒） | `5000` |
| `MCP_SSE_REPLAY_BUFFER` | SSE 重放缓冲区大小 | `100` |

---

## CLI 使用

```bash
cornerstone [command]
```

### 服务器

```bash
cornerstone serve                    # 启动 HTTP API + MCP 服务器
```

### 数据库管理

```bash
cornerstone db list                  # 列出所有数据库
cornerstone db create <name>         # 创建数据库
  -d, --description string           数据库描述
cornerstone db get <id>              # 获取数据库详情
cornerstone db update <id> -n <name> # 更新数据库
  -d, --description string           新描述
cornerstone db delete <id>           # 删除数据库
```

### 表管理

```bash
cornerstone table list <database-id>     # 列出表
cornerstone table create <db-id> <name>  # 创建表
  -d, --description string               表描述
cornerstone table get <id>               # 获取表详情
cornerstone table update <id> -n <name>  # 更新表
  -d, --description string               新描述
cornerstone table delete <id>            # 删除表
```

### 字段管理

```bash
cornerstone field list <table-id>                 # 列出字段
cornerstone field create <table-id> <name> <type> # 创建字段
  -d, --description string                        字段描述
  -r, --required                                  是否必填
  -o, --options string                            选项（逗号分隔）
cornerstone field get <id>                        # 获取字段详情
cornerstone field update <id> -n <name>           # 更新字段
  -t, --type string                               新类型
  -d, --description string                        新描述
  -r, --required                                  是否必填
  -o, --options string                            选项（逗号分隔）
cornerstone field delete <id>                     # 删除字段
```

### 记录管理

```bash
cornerstone record list <table-id>              # 列出记录
  -l, --limit int                                每页数量（默认 20）
  -o, --offset int                               偏移量（默认 0）
  -f, --filter string                            过滤条件（JSON）
cornerstone record create <table-id> '<json>'   # 创建记录
cornerstone record get <id>                     # 获取记录
cornerstone record update <id> '<json>'         # 更新记录
  -v, --version int                              乐观锁版本号
cornerstone record delete <id>                  # 删除记录
cornerstone record batch <table-id> '<json>' <count> # 批量创建
```

### Token 管理

```bash
cornerstone token list              # 列出 Token
cornerstone token create <name>     # 创建 Token
  -s, --scopes string               权限范围（JSON）
  -e, --expires string              过期时间（RFC3339）
cornerstone token update <id>       # 更新 Token
  -s, --scopes string               权限范围（JSON）
  -e, --expires string              过期时间（RFC3339）
cornerstone token delete <id>       # 删除 Token
```

### 其他

```bash
cornerstone migrate                 # 执行数据库迁移
cornerstone --version               # 显示版本
```

---

## REST API

服务器启动后（`cornerstone serve`），所有请求通过 `Authorization: Bearer <token>` 认证。

> 所有端点使用 `/api/v1/` 前缀；原有 `/api/` 路径自动重定向至 `/api/v1/` 以保持兼容。

### 接口列表

| 领域 | 方法 | 路径 | 说明 |
|------|------|------|------|
| Token | GET | `/api/v1/tokens` | 列出 Token |
| Token | POST | `/api/v1/tokens` | 创建 Token |
| Token | PUT | `/api/v1/tokens/{id}` | 更新 Token |
| Token | DELETE | `/api/v1/tokens/{id}` | 删除 Token |
| 数据库 | GET | `/api/v1/databases` | 列出数据库 |
| 数据库 | POST | `/api/v1/databases` | 创建数据库 |
| 数据库 | GET | `/api/v1/databases/{id}` | 获取数据库 |
| 数据库 | PUT | `/api/v1/databases/{id}` | 更新数据库 |
| 数据库 | DELETE | `/api/v1/databases/{id}` | 删除数据库 |
| 数据库 | POST | `/api/v1/databases/with-tables` | 一键建库+建表+建字段 |
| 表 | GET | `/api/v1/databases/{id}/tables` | 列出表 |
| 表 | POST | `/api/v1/tables` | 创建表 |
| 表 | GET | `/api/v1/tables/{id}` | 获取表 |
| 表 | PUT | `/api/v1/tables/{id}` | 更新表 |
| 表 | DELETE | `/api/v1/tables/{id}` | 删除表 |
| 字段 | GET | `/api/v1/tables/{id}/fields` | 列出字段 |
| 字段 | POST | `/api/v1/fields` | 创建字段 |
| 字段 | GET | `/api/v1/fields/{id}` | 获取字段 |
| 字段 | PUT | `/api/v1/fields/{id}` | 更新字段 |
| 字段 | DELETE | `/api/v1/fields/{id}` | 删除字段 |
| 记录 | GET | `/api/v1/records` | 列出记录 |
| 记录 | POST | `/api/v1/records` | 创建记录 |
| 记录 | GET | `/api/v1/records/{id}` | 获取记录 |
| 记录 | PUT | `/api/v1/records/{id}` | 更新记录 |
| 记录 | DELETE | `/api/v1/records/{id}` | 删除记录 |
| 记录 | POST | `/api/v1/records/batch` | 批量创建记录 |
| 记录 | GET | `/api/v1/records/export` | 导出记录 |
| 文件 | POST | `/api/v1/files/upload` | 上传文件 |
| 文件 | GET | `/api/v1/files/{id}` | 获取文件信息 |
| 文件 | GET | `/api/v1/files/{id}/download` | 下载文件 |
| 文件 | DELETE | `/api/v1/files/{id}` | 删除文件 |
| 文件 | GET | `/api/v1/records/{id}/files` | 列出记录关联文件 |
| 查询 | POST | `/api/v1/query` | Query DSL 查询 |
| 查询 | GET | `/api/v1/query` | Query DSL 查询（GET） |
| 查询 | GET | `/api/v1/query/simple` | 简化查询 |
| 查询 | POST | `/api/v1/query/batch` | 批量查询 |
| 查询 | POST | `/api/v1/query/explain` | 查询解释 |
| 查询 | POST | `/api/v1/query/validate` | 校验查询 |
| 查询 | GET | `/api/v1/query/tables` | 可访问表列表 |
| 查询 | GET | `/api/v1/query/schema/{table}` | 表 Schema |
| AI | POST | `/api/v1/ai/chat` | AI 助手对话 |
| MCP | POST | `/mcp` | MCP 协议（JSON-RPC） |
| MCP | GET | `/mcp` | MCP SSE 事件流 |
| 监控 | GET | `/metrics` | Prometheus 指标 |
| 健康检查 | GET | `/health` | 健康探针 |
| 就绪探针 | GET | `/ready` | 就绪探针 |

### 请求示例

```bash
# 创建数据库
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"name": "测试库", "description": "用于测试"}'

# 创建记录
curl -X POST http://localhost:8080/api/v1/records \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"table_id": "tbl_xxx", "data": {"name": "张三", "age": 28}}'

# Query DSL 查询
curl -X POST http://localhost:8080/api/v1/query \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", "select": ["id", "data"], "where": {"and": [{"field": "table_id", "op": "eq", "value": "tbl_xxx"}]}, "page": 1, "size": 20}'
```

---

## 数据模型

```text
Database ──1:N──> Table ──1:N──> Field
                  Table ──1:N──> Record ──1:N──> File
```

| 模型 | ID 前缀 | 说明 |
|------|---------|------|
| Token | `tok_` | API 认证令牌，Master Token 拥有全部权限 |
| Database | `db_` | 数据库 |
| Table | `tbl_` | 表 |
| Field | `fld_` | 字段（string/number/boolean/date/attachment 等） |
| Record | `rec_` | 记录（JSONB 存储，乐观锁） |
| File | `fil_` | 文件附件 |

---

## 认证

所有 API 请求（除 `/health`、`/ready`、`/metrics`）需携带 Token：

```http
Authorization: Bearer <token>
```

也可使用 `X-API-Key` 请求头作为替代方案（优先级高于 `Authorization: Bearer`）：

```http
X-API-Key: <token>
```

- **Master Token**：启动时自动生成（或通过 `MASTER_TOKEN` 环境变量预设），拥有全部权限
- **普通 Token**：由 Master Token 通过 `POST /api/v1/tokens` 创建，可配置数据库/表级权限范围

---

## MCP 协议

Cornerstone 通过 `/mcp` 端点暴露 MCP（Model Context Protocol）接口，AI Agent 可直接操作数据。

连接方式：
- **SSE 事件流**：`GET /mcp`（`Accept: text/event-stream`）
- **JSON-RPC 请求**：`POST /mcp`

---

## AI 助手

启用方式：在 `.env` 中配置 `LLM_API_KEY`。

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "帮我创建一个用户表"}'
```

支持自然语言建库、建表、查询数据、生成测试数据。

---

## Query DSL

通过 JSON 描述查询，支持过滤、排序、聚合、JOIN。详见 [Query DSL 文档](docs/Query.md)。

---

## License

AGPL-3.0
