# Cornerstone

> 轻量数据资产平台 CLI：集中存储、Token 接入、AI 助手、MCP 协议。

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

Cornerstone 是一个轻量数据资产平台，面向**测试、开发和内部数据管理**场景。
核心定位：**"数据库 + Token 接口 + Query DSL + AI 助手 + MCP 协议"**。

---

## 适合谁用

- 需要集中管理散落数据（API 推送、CSV/JSON 导入、AI 生成）的团队
- 希望用 Token（API Key）直接对接，不想走传统登录流程的场景
- 需要 AI 辅助建库、建表、生成测试数据的开发者
- 通过 MCP 协议让 AI Agent 操作结构化数据的场景

---

## 核心能力

- **CLI 命令行**：Cobra 框架，支持 db/table/field/record/token 全套 CRUD 命令
- **Token 认证**：Master Token 管理一切，普通 Token 可配置权限范围（数据库/表级读写）
- **数据管理**：Database → Table → Field → Record 完整 CRUD
- **文件管理**：上传、下载、关联记录
- **Query DSL**：JSON 描述查询，支持过滤、排序、聚合、JOIN
- **AI 助手**：自然语言建库、建表、查询数据、生成测试数据
- **MCP 协议**：通过 `/mcp` 暴露数据库管理与查询工具，SSE 接收变更通知
- **HTTP API**：`serve` 子命令启动 REST API + MCP 服务器

---

## 快速开始

### 1) 配置

```bash
git clone https://github.com/jiangfire/cornerstone.git
cd cornerstone
cp .env.example .env
# 编辑 .env，按需配置（可选：LLM_API_KEY、MASTER_TOKEN 等）
```

### 2) Docker 启动

```bash
# 生产模式
docker compose up -d --build

# 开发模式
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
```

### 3) 本地开发

```bash
go mod download

# 启动 API 服务器
go run ./cmd/main.go serve

# 或使用 CLI 命令
go run ./cmd/main.go --help
go run ./cmd/main.go migrate
go run ./cmd/main.go db list
```

---

## CLI 命令

```bash
cornerstone [command]

# 服务器
cornerstone serve                    # 启动 HTTP API + MCP 服务器

# 数据库管理
cornerstone db list                  # 列出所有数据库
cornerstone db create <name>         # 创建数据库
cornerstone db get <id>              # 获取数据库详情
cornerstone db update <id> -n <name> # 更新数据库
cornerstone db delete <id>           # 删除数据库

# 表管理
cornerstone table list <database-id>     # 列出表
cornerstone table create <db-id> <name>  # 创建表
cornerstone table get <id>               # 获取表详情
cornerstone table update <id> -n <name>  # 更新表
cornerstone table delete <id>            # 删除表

# 字段管理
cornerstone field list <table-id>               # 列出字段
cornerstone field create <table-id> <name> <type> # 创建字段
cornerstone field get <id>                      # 获取字段详情
cornerstone field update <id> -n <name>         # 更新字段
cornerstone field delete <id>                   # 删除字段

# 记录管理
cornerstone record list <table-id>              # 列出记录
cornerstone record create <table-id> '<json>'   # 创建记录
cornerstone record get <id>                     # 获取记录
cornerstone record update <id> '<json>'         # 更新记录
cornerstone record delete <id>                  # 删除记录
cornerstone record batch <table-id> '<json>' <count> # 批量创建

# Token 管理
cornerstone token list              # 列出 Token
cornerstone token create <name>     # 创建 Token
cornerstone token update <id>       # 更新 Token
cornerstone token delete <id>       # 删除 Token

# 其他
cornerstone migrate                 # 执行数据库迁移
cornerstone version                 # 显示版本
```

---

## API 概览（serve 模式）

| 领域 | 路径 | 说明 |
|------|------|------|
| Token | `GET/POST/PUT/DELETE /api/tokens` | Token 管理 |
| 数据库 | `GET/POST/PUT/DELETE /api/databases` | 数据库 CRUD |
| 表 | `GET/POST/PUT/DELETE /api/tables` | 表 CRUD |
| 字段 | `GET/POST/PUT/DELETE /api/fields` | 字段 CRUD |
| 记录 | `GET/POST/PUT/DELETE /api/records` | 记录 CRUD + 批量 + 导出 |
| 文件 | `POST/GET/DELETE /api/files` | 文件上传下载 |
| 查询 | `GET/POST /api/query` | Query DSL |
| AI | `POST /api/ai/chat` | AI 助手对话 |
| MCP | `GET/POST /mcp` | MCP 协议端点 |
| 健康检查 | `GET /health` `/ready` | 健康探针 |

---

## 数据模型

```text
Database ──1:N──> Table ──1:N──> Field
                  Table ──1:N──> Record ──1:N──> File
```

| 模型 | ID 前缀 | 说明 |
|------|---------|------|
| Token | `tok_` | API 认证令牌，支持 Master Token 和权限范围 |
| Database | `db_` | 数据库 |
| Table | `tbl_` | 表 |
| Field | `fld_` | 字段（支持 string/number/boolean/date/attachment 等类型） |
| Record | `rec_` | 记录（JSONB 存储，乐观锁） |
| File | `fil_` | 文件附件 |

---

## 认证

所有 API 请求（除 `/health`）需要携带 Token：

```http
Authorization: Bearer <token>
```

- **Master Token**：系统启动时自动生成（或通过 `MASTER_TOKEN` 环境变量预设），拥有全部权限
- **普通 Token**：由 Master Token 创建，可配置数据库/表级权限范围

---

## 项目结构

```text
cornerstone/
  cmd/main.go                  # CLI 入口
  internal/
    cli/                       # Cobra CLI 命令
    config/                    # 配置管理
    db/                        # 数据库初始化/迁移
    handlers/                  # HTTP API 处理器
    mcp/                       # MCP 协议实现
    middleware/                # HTTP 中间件
    models/                    # 数据模型
    services/                  # 业务逻辑
    authz/                     # 权限控制
  pkg/
    db/                        # GORM 数据库封装
    dto/                       # 响应格式
    log/                       # 日志
    query/                     # Query DSL 引擎
  docs/
    swagger/                   # Swagger API 文档 (swag 生成)
```

---

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DB_TYPE` | `sqlite` 或 `postgres` | `sqlite` |
| `DATABASE_URL` | 数据库连接串 | `./cornerstone.db` |
| `SERVER_MODE` | `release` 或 `debug` | `release` |
| `PORT` | 服务端口 | `8080` |
| `MASTER_TOKEN` | 预设 Master Token（留空则自动生成） | - |
| `LLM_API_KEY` | LLM API Key | - |
| `LLM_MODEL` | LLM 模型名 | `gpt-4o` |
| `LLM_BASE_URL` | 自定义 LLM API 地址 | - |

---

## License

AGPL-3.0
