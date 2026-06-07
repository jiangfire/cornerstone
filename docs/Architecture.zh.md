[English](Architecture.md) | [中文](Architecture.zh.md)

# 架构

> Cornerstone 系统架构与组件概览。

---

## 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端层                               │
├─────────────┬─────────────┬─────────────┬───────────────────┤
│   CLI 工具   │  REST API   │    MCP      │    AI 助手        │
│  (cobra)    │  (gin)      │  (SSE/JSON) │   (LLM + 工具)    │
└──────┬──────┴──────┬──────┴──────┬──────┴─────────┬─────────┘
       │             │             │                │
       └─────────────┴─────────────┴────────────────┘
                         │
              ┌──────────▼──────────┐
              │     服务层           │
              │    (业务逻辑)        │
              └──────────┬──────────┘
                         │
              ┌──────────▼──────────┐
              │     数据层           │
              │  (SQLite/MySQL/      │
              │   PostgreSQL)        │
              └─────────────────────┘
```

---

## 组件概览

### 1. CLI (cmd/main.go + internal/cli/)

基于 [Cobra](https://github.com/spf13/cobra) 构建的命令行工具，为所有数据管理操作提供 CLI。

- **特性**：零依赖（数据库除外），适合脚本自动化
- **全局标志**：
  - `--json`：输出结构化 JSON（适合管道处理）
  - `--token` / `-t`：指定认证令牌（覆盖 `MASTER_TOKEN` 环境变量）
- **语义化退出码**：
  - `0` - 成功
  - `1` - 一般错误
  - `2` - 校验错误
  - `3` - 资源未找到
  - `4` - 权限不足
  - `5` - 服务器错误

### 2. REST API (internal/handlers/)

基于 [Gin](https://gin-gonic.com/) 构建的 HTTP API 服务器。所有端点都以 `/api/v1/` 为前缀。

- **认证**：`Authorization: Bearer <token>` 或 `X-API-Key: <token>`
- **Swagger 文档**：启动服务器后访问 `/swagger/index.html`
- **端点分组**：
  - `/api/v1/tokens` - 令牌管理
  - `/api/v1/databases` - 数据库管理
  - `/api/v1/tables` - 表管理
  - `/api/v1/fields` - 字段管理
  - `/api/v1/records` - 记录管理
  - `/api/v1/files` - 文件管理
  - `/api/v1/query` - 查询 DSL
  - `/api/v1/ai/chat` - AI 助手
  - `/mcp` - MCP 协议端点
  - `/health`、`/ready`、`/metrics` - 健康与监控探针

### 3. MCP 协议 (internal/mcp/)

原生支持 [Model Context Protocol](https://modelcontextprotocol.io/)。AI 代理可通过标准协议操作数据。

- **传输方式**：
  - SSE 流：`GET /mcp`（`Accept: text/event-stream`）
  - JSON-RPC：`POST /mcp`
- **工具列表**：query_data、create_database、list_databases、get_database、update_database、delete_database、create_database_with_tables、create_table、list_tables、get_table、update_table、delete_table、create_field、list_fields、update_field、delete_field、insert_record、list_records、get_record、update_record、delete_record、batch_insert_records、generate_test_data、get_table_schema
- **认证**：与 REST API 共用基于令牌的认证

### 4. AI 助手 (internal/handlers/ai.go + internal/services/ai_*.go)

集成 LLM 的数据助手，支持自然语言交互。

- **配置**：`LLM_API_KEY`、`LLM_MODEL`、`LLM_BASE_URL`
- **能力**：
  - 查询数据（通过查询 DSL）
  - 创建 / 修改数据库 schema
  - 插入 / 更新 / 删除记录
  - 生成测试数据
- **工具调用**：AI 通过 `ExecuteAIToolForToken` 调用内部服务，权限与普通令牌相同

### 5. 查询 DSL (pkg/query/)

类 SQL 的 JSON 查询语言，支持：

- 过滤（where / having）
- 排序（orderBy）
- 分页（page / size）
- 聚合（groupBy + aggregate）
- JOIN（left / right / inner / outer）
- UNION / INTERSECT
- 自动权限过滤（根据令牌作用域自动注入条件）

### 6. 授权系统 (internal/authz/)

- **主令牌**：完整权限
- **普通令牌**：基于 JSON Scope 的细粒度控制
- **缓存**：令牌和权限上下文缓存 5 分钟
- **字段级权限**：支持基于白名单的字段访问控制

### 7. 数据层 (pkg/db/)

支持三种数据库后端：

| 后端 | 适用场景 | 特性 |
|---------|----------|----------|
| SQLite | 本地开发、CI、小规模部署 | 零配置，文件即数据库 |
| PostgreSQL | 需要大量 JSON 查询的生产环境 | 优秀的 JSONB 性能 |
| MySQL 8.0+ | MySQL 生态兼容 | 支持生成列索引优化 |

### 8. 缓存 (pkg/cache/)

- **内存缓存**：默认，无需外部依赖
- **Redis 缓存**：配置 `REDIS_URL` 后自动切换
- **缓存类型**：令牌缓存、权限上下文缓存、字段定义缓存

### 9. 文件存储 (internal/services/file.go)

- **存储路径**：`./uploads`（相对于工作目录）
- **安全校验**：所有路径通过 `ResolveSecureStoragePath` 校验，防止目录遍历
- **默认限制**：单文件最大 10 MB，支持 `.jpg/.jpeg/.png/.gif/.pdf/.doc/.docx/.xls/.xlsx/.txt/.zip`
- **字段级限制**：可在字段配置中设置 `max_file_size_mb` 和 `allowed_types`

---

## 请求流程

```
客户端请求
    │
    ├─ CLI ──────┐
    ├─ REST API ─┼──> internal/handlers/ ──> internal/services/ ──> pkg/db/ ──> 数据库
    ├─ MCP ──────┤         │                      │
    └─ AI ───────┘    middleware/auth.go    internal/authz/
                         （令牌验证）            （权限检查）
```

1. **认证**：`middleware/auth.go` 提取并校验令牌有效性
2. **授权**：`internal/authz/` 根据令牌作用域判断是否允许操作
3. **业务逻辑**：`internal/services/` 执行具体业务操作
4. **数据访问**：`pkg/db/` 通过 GORM 访问数据库

---

## 部署架构

### 单节点部署（SQLite）

```
┌─────────────────┐
│  Cornerstone    │
│  （单二进制）    │
│                 │
│  ┌───────────┐  │
│  │  SQLite   │  │
│  │  (.db)    │  │
│  └───────────┘  │
│  ┌───────────┐  │
│  │  uploads/ │  │
│  └───────────┘  │
└─────────────────┘
```

适用于：个人开发、小团队、CI 测试

### 生产部署（PostgreSQL/MySQL + 可选 Redis）

```
┌─────────────────┐     ┌─────────────┐     ┌─────────────┐
│  Cornerstone    │────▶│ PostgreSQL  │     │   Redis     │
│  （容器）        │     │  （数据）    │     │  （缓存）    │
│                 │     └─────────────┘     └─────────────┘
│  ┌───────────┐  │
│  │  uploads/ │  │  ← 持久卷挂载
│  └───────────┘  │
└─────────────────┘
```

适用于：生产环境、多实例部署

---

## 扩展点

| 扩展点 | 说明 |
|-----------------|-------------|
| 自定义 LLM | 配置 `LLM_BASE_URL` 以连接任意兼容 OpenAI 的 API |
| 自定义缓存 | 实现 `pkg/cache.Cache` 接口并通过工厂注册 |
| 自定义迁移 | 扩展 `internal/migration/mapper/` 中的类型映射 |
| 自定义 MCP 工具 | 扩展 `internal/mcp/tools.go` 中的 `ListTools()` |

---

## 相关文档

- [查询 DSL](Query.md) - 查询引擎详细文档
- [迁移](Migration.md) - 外部数据库迁移
- [令牌作用域](TokenScopes.md) - 权限配置参考
