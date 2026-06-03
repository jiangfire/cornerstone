# Cornerstone

> 自托管结构化数据平台。单个二进制，零外部依赖，CLI + REST API 双模交互。

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Tests](https://github.com/jiangfire/cornerstone/actions/workflows/ci.yml/badge.svg)](https://github.com/jiangfire/cornerstone/actions/workflows/ci.yml)

Cornerstone 面向需要**轻量、可控、可编程**数据管理的开发者和团队。它提供数据库级别的结构定义（库/表/字段/记录）和细粒度权限控制，同时支持外部数据库迁移、AI 助手和 MCP 协议集成。

相比 Airtable/Notion 等 SaaS，Cornerstone 让你**完全掌控数据**；相比自建数据库 + ORM，它让你**几分钟内获得完整的数据管理后台**。

---

## 快速开始

### Docker（推荐）

```bash
docker compose up -d --build
```

### 从源码构建

```bash
make build    # 构建二进制
make dev      # 启动开发服务器
```

然后使用 CLI 或 REST API 管理数据：

```bash
# CLI
cornerstone db create mydb
cornerstone table create <db-id> users
cornerstone field create <table-id> name string --required
cornerstone record create <table-id> '{"name":"张三"}'

# REST API
curl http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer <token>"
```

---

## 核心特性

- **双模交互**：CLI 适合脚本自动化，REST API 适合应用集成
- **细粒度权限**：Token 级别的数据库/表级权限控制
- **外部迁移**：MySQL / PostgreSQL / SQLite 一键迁移到 Cornerstone
- **AI Ready**：内置 AI 助手，支持 MCP 协议，AI Agent 可直接操作数据
- **Query DSL**：类 SQL 的 JSON 查询语言，支持过滤、排序、聚合、JOIN
- **轻量部署**：单个二进制，SQLite 即可运行，资源占用极低

---

## 配置

复制 `.env.example` 为 `.env`，按需修改：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DB_TYPE` | `sqlite`、`postgres` 或 `mysql`（MySQL 8.0+） | `sqlite` |
| `DATABASE_URL` | 数据库连接串 | `./cornerstone.db` |
| `DB_MAX_OPEN` | 数据库最大打开连接数 | `10` |
| `DB_MAX_IDLE` | 数据库最大空闲连接数 | `5` |
| `DB_MAX_LIFETIME` | 连接最大生命周期（秒） | `3600` |
| `SERVER_MODE` | `release` 或 `debug` | `release` |
| `PORT` | 服务端口 | `8080` |
| `LOG_LEVEL` | 日志级别 | `info` |
| `MASTER_TOKEN` | Master Token（留空则 Master Token 认证不可用） | - |
| `LLM_API_KEY` | LLM API Key（启用 AI 助手） | - |
| `LLM_MODEL` | LLM 模型名 | `gpt-4o` |
| `LLM_BASE_URL` | 自定义 LLM API 地址 | - |
| `MCP_ALLOWED_ORIGINS` | MCP 允许的来源（逗号分隔） | (空) |
| `MCP_SSE_KEEPALIVE_SEC` | SSE 心跳间隔（秒） | `25` |
| `MCP_SSE_RETRY_MS` | SSE 重连间隔（毫秒） | `3000` |
| `MCP_SSE_REPLAY_BUFFER` | SSE 重放缓冲区大小 | `128` |
| `REDIS_URL` | Redis 连接串（留空使用内存缓存） | - |

---

## CLI 使用

```bash
cornerstone serve                    # 启动 HTTP API + MCP 服务器

# 数据管理
cornerstone db list
cornerstone db create <name> [-d description]
cornerstone db get|update|delete <id>

cornerstone table list <db-id>
cornerstone table create <db-id> <name>
cornerstone table get|update|delete <id>

cornerstone field list <table-id>
cornerstone field create <table-id> <name> <type> [-r] [-d desc]
cornerstone field get|update|delete <id>

cornerstone record list <table-id> [-l limit] [-o offset] [-f filter]
cornerstone record create <table-id> '<json>'
cornerstone record get|update|delete <id>
cornerstone record batch <table-id> '<json>' <count>

# Token 与权限
cornerstone token list
cornerstone token create <name> [-s scopes] [-e expires]
cornerstone token update|delete <id>

# 外部数据库迁移
cornerstone migration run [-c config] [--source-type mysql|postgres|sqlite] [--source-dsn ...] [--target-db ...]
cornerstone migration preview
cornerstone migration template

# 其他
cornerstone cache clear
cornerstone migrate                  # 执行数据库结构迁移
cornerstone --version
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

你可以通过 API 或 CLI 自由定义数据库、表、字段结构，无需预编译迁移脚本。记录以 JSONB 存储，支持乐观锁版本控制。文件附件与记录关联，支持权限隔离。

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

Cornerstone 原生支持 [MCP（Model Context Protocol）](https://modelcontextprotocol.io/)，AI Agent 可以通过标准协议直接读取和写入你的数据，无需编写自定义集成代码。

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

支持自然语言建库、建表、查询数据、生成测试数据。AI 助手理解 Cornerstone 的数据模型和 API，可以直接调用内部工具完成操作。

---

## Query DSL

通过 JSON 描述查询，支持过滤、排序、聚合、JOIN。无需手写 SQL，即可实现复杂的数据查询。详见 [Query DSL 文档](docs/Query.md)。

---

## 开发

```bash
make build          # 构建二进制（输出到 bin/）
make test           # 运行全部测试（含 race 检测）
make test-cover     # 运行测试并生成覆盖率报告
make lint           # 运行 golangci-lint
make check          # 完整检查（fmt + vet + test）
make swagger        # 重新生成 Swagger 文档
make dev            # 启动本地开发服务器
```

---

## 测试

```bash
go test ./...                           # 运行全部测试
go test ./... -coverprofile=coverage.out # 生成覆盖率报告
go tool cover -func=coverage.out        # 查看函数级覆盖率
```

核心包测试覆盖率 80%+，CI 包含 MySQL/PostgreSQL 迁移集成测试、golangci-lint、govulncheck 和 Trivy 安全扫描。

---

## License

AGPL-3.0
