# Cornerstone

> 轻量数据资产平台：集中存储、Token 接入、AI 助手、MCP 协议。

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Vue Version](https://img.shields.io/badge/Vue-3.5+-4FC08D?style=flat&logo=vue.js&logoColor=white)](https://vuejs.org/)
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

- **Token 认证**：Master Token 管理一切，普通 Token 可配置权限范围（数据库/表级读写）
- **数据管理**：Database → Table → Field → Record 完整 CRUD
- **文件管理**：上传、下载、关联记录
- **Query DSL**：JSON 描述查询，支持过滤、排序、聚合、JOIN
- **AI 助手**：自然语言建库、建表、查询数据、生成测试数据
- **MCP 协议**：通过 `/mcp` 暴露数据库管理与查询工具，SSE 接收变更通知
- **批量操作**：批量创建记录、导出数据
- **JSON 建表**：一个请求创建数据库 + 表 + 字段

---

## 快速开始（Docker）

### 1) 配置

```bash
git clone https://github.com/jiangfire/cornerstone.git
cd cornerstone
cp .env.example .env
# 编辑 .env，按需配置（可选：LLM_API_KEY、MASTER_TOKEN 等）
```

### 2) 启动

```bash
# 生产模式
docker compose up -d --build

# 开发模式（额外暴露 5432，CORS 全放开）
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
```

### 3) 访问

- 应用首页：`http://localhost:8080`
- API：`http://localhost:8080/api`
- 健康检查：`http://localhost:8080/health`
- Swagger：`http://localhost:8080/swagger/index.html`
- MCP：`http://localhost:8080/mcp`

---

## 本地开发

### 环境要求

- Go 1.25+
- Node.js 20+ / pnpm
- SQLite（默认）或 PostgreSQL 15+

### 后端

```bash
cd backend
go mod download
go run ./cmd/server/main.go
```

### 前端

```bash
cd frontend
pnpm install
pnpm dev
```

### 嵌入前端到 Go 二进制

```bash
cd frontend
pnpm run build:embed
```

### 测试与质量

```bash
# 后端
cd backend && go test ./...

# 前端
cd frontend && pnpm type-check && pnpm lint && pnpm test:unit
```

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

## API 概览

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

---

## 项目结构

```text
cornerstone/
  backend/      # Go + Gin + GORM
  frontend/     # Vue 3 + TypeScript + Element Plus
  docs/         # 文档
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
