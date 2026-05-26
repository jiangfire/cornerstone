# Cornerstone 精简重构计划（Token 权限 + AI 助手版）

## 背景与目标

将 Cornerstone 改造为一个**轻量数据资产平台**：

- **数据集中**：把分散的数据源（API 推送、CSV/JSON 导入、AI 生成）统一到一个平台
- **低频查询但需关联**：Query DSL 是核心开发接口，支持跨表 join 和复杂查询
- **省去手动 CRUD**：AI 助手负责建库、建表、生成测试数据；JSON 也可直接建表
- **Token 权限**：完全用 Token（类似 API Key）替代用户登录，Master Token 管理一切
- **数据治理基础**：数据集中存放后，天然具备统一治理的前提

> 数据主要用于**测试/开发**，非生产环境，使用频率低但查询可能复杂。

---

## 认证模型：Token 体系（替代 User + JWT）

### 核心设计

- **完全去掉 User 表和 JWT 登录流程**
- **所有身份标识通过 Token**：调用方携带 `X-API-Key` 或 `Authorization: Bearer <token>`
- **Master Token**：系统初始化时自动生成（打印到日志或写入文件），拥有全部权限，用于创建其他 Token
- **普通 Token**：由 Master Token 创建，可配置权限范围（数据库/表/字段级别的读/写/删）

### Token 模型

```go
// backend/internal/models/models.go
type Token struct {
    ID        string         `gorm:"primaryKey"`
    Token     string         `gorm:"uniqueIndex;not null"` // 实际 token 字符串，如 "cs_abc123xyz"
    Name      string         // 显示名称，如 "前端服务"、"测试脚本"
    IsMaster  bool           `gorm:"default:false"` // 是否主 token
    Scopes    string         `gorm:"type:text"`     // JSON 权限配置
    ExpiresAt *time.Time     // 过期时间，nil 表示永不过期
    CreatedAt time.Time
}

// Scopes JSON 结构示例：
// {
//   "databases": {
//     "db_xxx": "admin",      // admin: 全部权限
//     "db_yyy": "viewer"      // viewer: 只读
//   },
//   "tables": {
//     "tbl_xxx": { "role": "editor", "fields": { "fld_xxx": ["read","write"] } }
//   }
// }
```

### 权限校验

- 中间件从 header 读取 token，查询有效性（存在且未过期）
- Master Token 跳过权限校验，直接放行
- 普通 Token 根据 Scopes  JSON 校验当前请求是否有权访问目标资源
- 无权限返回 403，无效 token 返回 401

### Token 管理 API

```
GET    /api/tokens          -> 列出 Token（Master 看全部，普通 Token 只看自己）
POST   /api/tokens          -> 创建 Token（需 Master Token）
DELETE /api/tokens/:id      -> 删除 Token（需 Master Token 或删除自己）
PUT    /api/tokens/:id      -> 修改 Token 权限（需 Master Token）
```

---

## 前端方案（3 个页面）

### 页面清单

| 页面 | 路由 | 功能 |
|------|------|------|
| **数据浏览器** | `/` | 浏览数据库/表/记录，支持搜索筛选，查看记录详情 |
| **Token 管理** | `/tokens` | 创建/查看/删除 Token，配置权限范围 |
| **AI 助手** | `/ai` | 自然语言对话：建库、建表、查数据、生成测试数据 |

### 数据浏览器（DataBrowserView.vue）

融合原 `RecordsView + DatabasesView + TableView`：

- **左侧边栏**：数据库列表 -> 点击展开显示该库下的表列表 -> 点击表加载记录
- **顶部工具栏**：
  - 当前位置：`数据库名 / 表名`
  - 搜索框：支持按字段筛选、全文搜索
  - 新建记录按钮
- **主区域**：记录列表表格（虚拟滚动，参考原 RecordsView）
  - 分页
  - 点击行弹出抽屉/弹窗显示记录详情（JSON 格式化）
  - 行内编辑（可选）
- **右侧详情抽屉**：单条记录的完整 JSON 展示

### Token 管理（TokensView.vue）

- **Token 列表**：显示名称、权限摘要、创建时间、过期时间
- **创建 Token**：
  - 输入名称
  - 选择权限范围：数据库级（admin/viewer）-> 表级 -> 字段级（read/write/delete）
  - 可选过期时间
  - 创建后显示 token 字符串（**只显示一次**）
- **删除 Token**：确认后删除

### AI 助手（AIAssistantView.vue）

- **聊天界面**：Element Plus 风格或自定义
- **输入框**：自然语言描述需求
- **消息展示**：
  - 用户消息：原始输入
  - AI 消息：回复文本 + 操作结果（如"已创建表 orders，包含 3 个字段"）
  - 数据表格：如果 AI 返回查询结果，渲染为表格
- **快捷指令**："创建数据库"、"创建表"、"生成测试数据"、"查询数据"

### 路由与布局

- 路由精简为 3 条 + 404
- `AppLayout` 简化导航栏：数据浏览器 | Token 管理 | AI 助手
- 去掉所有登录/注册/个人资料相关逻辑
- API 客户端改为从配置文件读取 `API_KEY`，每个请求携带 `X-API-Key` header

---

## 后端方案

### 保留的 API

| 领域 | 接口 | 说明 |
|------|------|------|
| 数据库 | `GET/POST/PUT/DELETE /api/databases` | 数据库 CRUD |
| 表 | `GET/POST/PUT/DELETE /api/tables` | 表 CRUD |
| 字段 | `GET/POST/PUT/DELETE /api/fields` | 字段 CRUD |
| 记录 | `GET/POST/PUT/DELETE /api/records` | 记录 CRUD + 批量 + 导出 |
| 查询 | `GET/POST /api/query` 等 | **核心接口，保留并增强** |
| Token | `GET/POST/DELETE/PUT /api/tokens` | Token 管理 |
| AI | `POST /api/ai/chat` | AI 助手对话 |
| MCP | `/mcp` | MCP 协议端点 |
| 文件 | `POST/GET/DELETE /api/files` | 文件上传下载（可选保留） |

### 删除的 API

- `/api/auth/*` -- 注册/登录/注销
- `/api/users/*` -- 用户管理
- `/api/organizations/*` -- 组织管理
- `/api/databases/:id/share` -- 数据库分享（被 Token 权限替代）
- `/api/tables/:id/field-permissions` -- 字段权限配置（被 Token 权限替代）
- `/api/plugins/*` -- 插件管理
- `/api/governance/*` -- 治理任务
- `/api/stats/*` -- 统计
- `/api/settings/*` -- 系统设置
- `/api/integrations/events` -- 集成事件

### 新增：AI Agent 服务

**文件**：`backend/internal/services/ai_agent.go`

功能：
- 接收自然语言请求 + 当前数据库 schema 上下文
- 调用 LLM API（Claude / OpenAI，通过环境变量 `LLM_API_KEY`、`LLM_MODEL` 配置）
- 实现 function calling，暴露以下 tools 给 LLM：

| Tool | 功能 |
|------|------|
| `create_database` | 创建数据库 |
| `create_table` | 在指定库中创建表（支持 JSON 字段定义） |
| `create_field` | 在指定表中创建字段 |
| `execute_query` | 执行 Query DSL 查询 |
| `insert_records` | 批量插入记录 |
| `update_record` | 更新单条记录 |
| `delete_record` | 删除单条记录 |
| `generate_test_data` | 按约束生成测试数据 |
| `get_schema` | 获取数据库/表/字段结构 |
| `list_databases` | 列出数据库 |
| `list_tables` | 列出表 |

- 解析 LLM 返回的 tool_calls，调用对应的内部 service
- 返回执行结果或错误给前端

**文件**：`backend/internal/handlers/ai.go`

```
POST /api/ai/chat
Request:  {
  "message": "帮我创建一个订单表，包含订单号、金额、状态字段，并生成 10 条测试数据",
  "context": { "database_id": "db_xxx" }
}
Response: {
  "type": "result",           // result / error / query_result
  "message": "已创建表 orders，包含 3 个字段，并插入 10 条测试数据",
  "data": { "table_id": "tbl_xxx", "inserted_count": 10 }
}
```

### MCP 增强

**文件**：`backend/internal/mcp/tools.go`

在现有 4 个工具基础上新增：

| 工具名 | 功能 |
|--------|------|
| `create_table` | 创建表 |
| `create_field` | 创建字段 |
| `insert_record` | 插入记录 |
| `update_record` | 更新记录 |
| `delete_record` | 删除记录 |
| `generate_test_data` | 生成测试数据 |

### Query DSL 增强

**文件**：`backend/pkg/query/`

增强方向（后续开发主要使用此接口）：

- **更多聚合函数**：`sum`, `avg`, `min`, `max`（已有）+ `stddev`, `variance`, `count_distinct`
- **HAVING 子句**：支持对聚合结果过滤
- **子查询**：支持 `IN (SELECT ...)`、`EXISTS` 等
- **窗口函数**（PostgreSQL 模式）：`ROW_NUMBER()`, `RANK()`, `LAG()`, `LEAD()`
- **UNION/INTERSECT**：多查询结果合并
- **简化语法增强**：
  - 支持 `or` 条件：`{"or": [{"status": "paid"}, {"status": "shipped"}]}`
  - 支持范围简写：`{"amount": {"between": [100, 500]}}`
  - 支持空值判断：`{"name": {"is_null": true}}`

### JSON 建表/建库

支持直接通过 JSON 定义创建数据库和表：

```json
POST /api/databases
{
  "name": "ecommerce",
  "tables": [
    {
      "name": "orders",
      "fields": [
        { "name": "order_no", "type": "string", "required": true },
        { "name": "amount", "type": "number", "required": true },
        { "name": "status", "type": "select", "options": { "choices": ["pending", "paid", "shipped"] } }
      ]
    }
  ]
}
```

后端自动依次创建：数据库 -> 表 -> 字段。

### 字段类型增强

**文件**：`backend/internal/services/field.go`、`frontend/src/views/DataBrowserView.vue`

在现有类型（`string`、`number`、`boolean`、`date`、`attachment`、`select`、`list`）基础上扩展：

| 类型 | 说明 | PostgreSQL 存储 | SQLite 存储 |
|------|------|-----------------|-------------|
| `string` | 单行文本 | `TEXT` | `TEXT` |
| `text` | 多行长文本 | `TEXT` | `TEXT` |
| `number` | 数值 | `NUMERIC` | `REAL` |
| `boolean` | 布尔 | `BOOLEAN` | `INTEGER` (0/1) |
| `date` | 日期 | `TIMESTAMP` | `TEXT` (ISO8601) |
| `select` | 单选 | `TEXT` | `TEXT` |
| `multiselect` | 多选 | `JSONB` | `TEXT` (逗号分隔) |
| `list` / `array` | 列表/数组 | `JSONB` | `TEXT` (逗号分隔或 JSON 字符串) |
| `json` | 任意 JSON 对象 | `JSONB` | `TEXT` (JSON 字符串) |
| `file` | 文件附件 | 存 `File` 表外键 | 存 `File` 表外键 |
| `link` | 关联到另一张表 | 存目标表记录 ID | 存目标表记录 ID |
| `email` | 邮箱 | `TEXT` | `TEXT` |
| `url` | URL | `TEXT` | `TEXT` |
| `color` | 颜色 | `TEXT` | `TEXT` |
| `rating` | 评分（1-5 星） | `INTEGER` | `INTEGER` |

**存储策略**：
- **PostgreSQL**：有 `JSONB` 的类型直接用 JSONB（`json`、`multiselect`、`list`）
- **SQLite**：无原生 JSONB，统一用 `TEXT` 存储
  - `json`：存完整 JSON 字符串
  - `multiselect` / `list`：优先尝试 JSON 数组字符串（`["a","b"]`），降级为逗号分隔（`"a,b"`）
- `link` 类型在 `Field.options` 中配置目标表：`{"target_table": "tbl_xxx"}`
- `file` 类型保留现有 `File` 表关联机制

**前端渲染**：
- `select` / `multiselect`：下拉选择/多选标签
- `list`：可增删的列表输入
- `json`：JSON 编辑器（带格式化）
- `link`：搜索选择器，关联到目标表记录
- `file`：文件上传/下载组件
- `rating`：星级评分组件
- `color`：颜色选择器

---

## 数据模型调整

### 保留的模型

| 模型 | 说明 |
|------|------|
| `Database` | 数据库 |
| `Table` | 表 |
| `Field` | 字段 |
| `Record` | 记录（data JSONB + version 乐观锁） |
| `File` | 文件（可选保留） |
| `Token` | **新增，替代 User 和权限模型** |

### 删除的模型

| 模型 | 说明 |
|------|------|
| `User` | 用户表不再需要 |
| `Organization` | 组织概念去掉 |
| `OrganizationMember` | |
| `DatabaseAccess` | 数据库级权限移到 Token.Scopes |
| `FieldPermission` | 字段级权限移到 Token.Scopes |
| `Plugin` | 插件去掉 |
| `PluginBinding` | |
| `PluginExecution` | |
| `GovernanceTask` | 治理任务去掉（但数据集中为后续治理打基础） |
| `GovernanceReview` | |
| `GovernanceEvidence` | |
| `GovernanceComment` | |
| `GovernanceExternalLink` | |
| `GovernanceOutboxEvent` | |
| `IntegrationInboundEvent` | |
| `ActivityLog` | 活动日志去掉 |
| `AppSettings` | 系统设置去掉 |
| `TokenBlacklist` | JWT 相关，不再需要 |

### 迁移调整

**文件**：`backend/internal/db/migrate.go`

- 去掉所有删除模型的表迁移
- 新增 Token 表的迁移
- 系统初始化时生成 Master Token：
  - 从环境变量 `MASTER_TOKEN` 读取（如果设置）
  - 否则随机生成一个 token 字符串
  - 写入 Token 表，`IsMaster: true`，`Scopes: {}`（空表示全部）
  - 打印到日志：`Master Token: cs_xxxxxxxxxxxx`

---

## 关键文件清单

### 前端

| 文件 | 操作 | 说明 |
|------|------|------|
| `frontend/src/views/DataBrowserView.vue` | 新增 | 数据浏览器 |
| `frontend/src/views/TokensView.vue` | 新增 | Token 管理 |
| `frontend/src/views/AIAssistantView.vue` | 新增 | AI 助手 |
| `frontend/src/router/index.ts` | 修改 | 3 条路由 |
| `frontend/src/services/api.ts` | 修改 | API Key 认证 |
| `frontend/src/App.vue` | 修改 | 简化布局 |
| `frontend/src/views/*`（除 NotFound） | 删除 | 其余页面全部删除 |

### 后端

| 文件 | 操作 | 说明 |
|------|------|------|
| `backend/internal/services/ai_agent.go` | 新增 | AI Agent 服务 |
| `backend/internal/handlers/ai.go` | 新增 | AI handler |
| `backend/internal/handlers/token.go` | 新增 | Token 管理 handler |
| `backend/internal/middleware/auth.go` | 重写 | Token 认证替代 JWT |
| `backend/internal/config/config.go` | 修改 | 新增 LLM 配置、去掉 JWT/组织/插件配置 |
| `backend/internal/mcp/tools.go` | 修改 | 新增工具 |
| `backend/pkg/query/` | 修改 | Query DSL 增强 |
| `backend/internal/models/models.go` | 修改 | 精简模型，新增 Token |
| `backend/internal/db/migrate.go` | 修改 | 精简迁移，新增 Master Token 初始化 |
| `backend/cmd/server/main.go` | 修改 | 路由调整、认证调整 |
| `backend/internal/handlers/basic.go` | 删除 | |
| `backend/internal/handlers/user.go` | 删除 | |
| `backend/internal/handlers/organization.go` | 删除 | |
| `backend/internal/handlers/plugin.go` | 删除 | |
| `backend/internal/handlers/governance.go` | 删除 | |
| `backend/internal/handlers/stats.go` | 删除 | |
| `backend/internal/handlers/settings.go` | 删除 | |
| `backend/internal/services/auth.go` | 删除 | JWT 相关 |
| `backend/internal/services/user.go` | 删除 | |
| `backend/internal/services/organization.go` | 删除 | |
| `backend/internal/services/plugin.go` | 删除 | |
| `backend/internal/services/governance.go` | 删除 | |
| `backend/internal/services/governance_apply.go` | 删除 | |
| `backend/internal/services/integration_events.go` | 删除 | |
| `backend/internal/services/stats.go` | 删除 | |
| `backend/internal/services/activity.go` | 删除 | |
| `backend/internal/services/settings.go` | 删除 | |
| `backend/internal/services/llm_governor_client.go` | 删除 | 替换为 AI Agent |

---

## 环境变量配置

新增/变更：

| 变量 | 说明 |
|------|------|
| `LLM_API_KEY` | LLM API Key（OpenAI / Anthropic） |
| `LLM_MODEL` | 模型名称，如 `claude-sonnet-4-6` 或 `gpt-4o` |
| `LLM_BASE_URL` | 可选，自定义 LLM API 地址 |
| `MASTER_TOKEN` | 可选，预设 Master Token 值 |

删除：
- `JWT_SECRET`（不再需要 JWT）
- `LLM_GOVERNOR_URL` / `LLM_GOVERNOR_TOKEN`（替换为直接 LLM 调用）
- `INTEGRATION_*` 系列（集成系统去掉）
- `GOVERNANCE_*` 系列（治理去掉）

---

## 实施步骤

### 第一阶段：后端核心改造
1. 精简 models.go，删除不需要的模型，新增 Token 模型
2. 重写认证中间件为 Token 认证
3. 新增 Token 管理 service + handler
4. 调整 migrate.go：精简表迁移 + Master Token 初始化
5. 新增 AI Agent service + handler
6. 调整 main.go：注册新路由，去掉旧路由

### 第二阶段：Query DSL + MCP 增强
1. Query DSL 功能增强（聚合、HAVING、子查询等）
2. MCP 新增工具（create_table、insert_record 等）
3. 支持 JSON 建表/建库

### 第三阶段：前端重构
1. 新建 DataBrowserView.vue（提取 RecordsView 核心逻辑 + DB/Table 导航）
2. 新建 TokensView.vue
3. 新建 AIAssistantView.vue
4. 简化路由和布局
5. 调整 API 客户端为 API Key 认证

### 第四阶段：清理
1. 删除废弃的前端页面
2. 删除废弃的后端 handler 和 service
3. 更新测试
4. 验证端到端流程

---

## 验证方案

- **Token 认证**：用 Master Token 创建普通 Token，验证权限隔离
- **数据浏览器**：打开首页，浏览数据库/表/记录，执行搜索
- **AI 助手**：输入"创建一个用户表，包含姓名和邮箱，生成 5 条测试数据"，验证自动建表 + 插入
- **Query DSL**：通过 `/api/query` 执行关联查询，验证 join 和聚合正常
- **MCP**：用 Claude Desktop 连接 `/mcp`，验证 `create_table`、`insert_record`、`query_data` 工具可用
- **JSON 建表**：`POST /api/databases` 带 JSON 定义，验证一键建库建表
