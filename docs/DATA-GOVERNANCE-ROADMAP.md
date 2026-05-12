# Cornerstone 数据底座系统改进计划

## Context

Cornerstone 定位为数据底座/数据库治理系统，而非前端数据管理工具。核心目标是提供强大的RESTful API和MCP接口，供各种系统（包括AI系统如Claude Code、Codex）集成使用，实现数据联邦和统一治理。

本改进计划聚焦于：
1. **数据治理能力增强**：审计跟踪、字段备注
2. **字段类型优化**：新增附件类型、简化选择类型
3. **查询能力提升**：DSL + 可视化查询构建器
4. **API优先设计**：完善MCP接口供AI系统集成

---

## Phase 1: 字段备注功能 (Field Description)

### 目标
为字段添加备注/描述功能，支持数据治理和文档化。

### 后端改动

**文件：** `backend/internal/models/models.go`

在 `Field` 结构体添加：
```go
type Field struct {
    // ... 现有字段
    Description string `gorm:"type:text" json:"description"` // 字段备注/描述
}
```

**文件：** `backend/internal/services/field.go`

- 更新 `CreateField` 方法：支持description参数
- 更新 `UpdateField` 方法：支持更新description
- 更新 `GetField` 和 `ListFields`：返回description

**文件：** `backend/internal/handlers/field.go`

- 更新handler处理description字段
- 验证description长度（建议最大1000字符）

### 数据库迁移

**文件：** `backend/internal/db/migrate.go`

添加迁移逻辑：
```go
// 添加字段备注列
DB.Migrator().AddColumn(&models.Field{}, "description")
```

### 前端改动

**文件：** `frontend/src/views/FieldsView.vue`

- 字段列表显示description（支持tooltip或展开）
- 新建/编辑字段表单添加description输入框（多行文本）

---

## Phase 2: 新增 Attachment 字段类型

### 目标
将附件作为一等公民的字段类型，而不是通过独立File表管理。

### 后端改动

**文件：** `backend/internal/services/field.go`

在 `FieldType` 定义中添加：
```go
const (
    // ... 现有类型
    FieldTypeAttachment = "attachment"
)
```

在 `FieldConfig` 添加附件配置：
```go
type FieldConfig struct {
    // ... 现有配置
    AllowedTypes   []string `json:"allowed_types,omitempty"`   // 允许的文件类型，如 ["image/*", ".pdf", ".docx"]
    MaxFileSizeMB  int      `json:"max_file_size_mb,omitempty"`  // 最大文件大小（MB）
    Multiple       bool     `json:"multiple,omitempty"`       // 是否允许多个文件
}
```

**文件：** `backend/internal/services/record.go`

更新记录验证逻辑：
- 验证attachment字段值是有效的File ID或File ID数组
- 检查文件是否存在、是否属于当前表
- 验证文件类型和大小是否符合FieldConfig

**文件：** `backend/internal/services/file.go`

确保File表支持：
- `RecordID` 关联
- `FieldID` 关联（指明属于哪个字段）
- 文件元数据（原文件名、MIME类型、大小等）

### API端点（复用现有）

- `POST /api/files/upload` - 上传附件文件（返回File ID）
- `POST /api/records` - 创建记录时attachment字段填File ID
- `PUT /api/records/:id` - 更新记录的attachment字段

### 前端改动

**文件：** `frontend/src/utils/fieldTypes.ts`

添加attachment类型定义和图标。

**文件：** `frontend/src/views/RecordsView.vue`

- 显示attachment字段时展示文件列表
- 支持文件上传（拖拽或点击）
- 支持文件下载和删除

---

## Phase 3: 移除 Select/Multi-Select 字段类型

### 目标
简化字段类型，将数据底座定位为基础存储，枚举值由应用层实现。

### 后端改动

**文件：** `backend/internal/services/field.go`

从 `FieldType` 枚举中移除：
```go
// 删除这些常量
// FieldTypeSelect = "select"
// FieldTypeList = "list"
```

更新 `FieldTypeAliases`，移除相关别名映射。

### 数据迁移策略

**文件：** `backend/internal/db/migrate.go`

添加迁移脚本：
```go
// 1. 标记所有select/list字段为废弃（或转换为text）
// 2. 保留现有数据，但禁止创建新的select/list字段
// 3. 在API响应中标记deprecated字段
```

**建议方案：**
- 现有select字段 → 转换为text字段（保留选项值在description中）
- 现有list字段 → 转换为text字段（使用逗号分隔值或JSON数组）

### API改动

**文件：** `backend/internal/handlers/field.go`

- 创建/更新字段时拒绝select/list类型
- 列出字段时对旧字段标记`deprecated: true`

---

## Phase 4: 搜索功能增强（DSL + 可视化构建器）

### 目标
提供简单搜索框 + 查询DSL + 可视化查询构建器，满足不同用户需求。

### 4.1 查询DSL完善

**文件：** `backend/pkg/query/model.go`（如存在）或新建

定义查询DSL结构：
```go
type QueryDSL struct {
    Table    string              `json:"table"`     // 表ID或名称
    Filter   *FilterExpression   `json:"filter"`    // 过滤条件
    Sort     []SortField         `json:"sort"`      // 排序
    Page     int                 `json:"page"`      // 页码
    PageSize int                 `json:"page_size"` // 每页大小
    Fields   []string            `json:"fields"`    // 返回字段
}

type FilterExpression struct {
    Operator string      `json:"operator"` // and, or, not
    Operands []interface{} `json:"operands"` // 嵌套条件或字段条件
}

type FieldCondition struct {
    Field    string      `json:"field"`    // 字段ID
    Operator string      `json:"operator"` // eq, ne, gt, lt, gte, lte, like, in, between, is_null
    Value    interface{} `json:"value"`    // 值
}
```

**文件：** `backend/internal/services/query.go`（新建或扩展现有）

实现DSL解析和执行：
1. 解析JSON DSL
2. 验证字段存在性和权限
3. 构建数据库查询（支持SQLite和PostgreSQL）
4. 返回结果

### 4.2 API端点

已存在查询端点，需增强：
- `POST /api/query` - 执行DSL查询（body: QueryDSL）
- `POST /api/query/simple` - 简单搜索（keyword, tableId）
- `GET /api/query/schema/:table` - 获取表结构（字段列表、类型，用于构建器）

### 4.3 前端可视化查询构建器

**新建文件：** `frontend/src/components/QueryBuilder.vue`

组件功能：
- 显示表的所有可查询字段
- 支持添加条件（AND/OR）
- 支持嵌套条件组
- 支持多种操作符（等于、包含、大于等）
- 实时预览生成的DSL JSON
- 执行查询并展示结果

**文件：** `frontend/src/views/RecordsView.vue`

集成QueryBuilder：
- 添加"高级搜索"按钮
- 切换简单搜索/高级搜索模式
- 高级搜索模式使用QueryBuilder组件

### 4.4 文档

**新建文件：** `docs/query-dsl-guide.md`

内容包括：
- DSL语法说明
- 示例查询
- 操作符列表
- 最佳实践

---

## Phase 5: MCP HTTP 接口完善

### 目标
确保MCP接口提供完整的CRUD能力，支持AI系统(Claude Code, Codex)作为动态数据层使用。

### 5.1 认证机制

**当前实现：** 使用Integration Token（已实现）

**配置：** 在环境变量中设置：
```bash
INTEGRATION_SHARED_TOKEN=your-secret-token-here
```

**MCP客户端使用：**
```
Headers:
  X-Source-System: claude-code
  Authorization: Bearer your-secret-token-here
```

### 5.2 MCP工具增强

**文件：** `backend/internal/mcp/tools.go`

确保覆盖以下工具：

**数据库管理：**
- `create_database` - 创建数据库
- `list_databases` - 列出数据库
- `get_database` - 获取数据库详情

**表管理：**
- `create_table` - 创建表
- `list_tables` - 列出表
- `get_table_schema` - 获取表结构

**字段管理：**
- `create_field` - 创建字段
- `list_fields` - 列出字段

**记录CRUD：**
- `create_record` - 创建记录
- `query_records` - 查询记录（支持DSL）
- `get_record` - 获取单条记录
- `update_record` - 更新记录
- `delete_record` - 删除记录

**附件管理：**
- `upload_file` - 上传附件
- `get_file` - 获取文件信息
- `download_file` - 下载文件

### 5.3 MCP文档

**新建文件：** `docs/mcp-integration-guide.md`

内容包括：
- MCP服务器配置
- 认证方式（Integration Token）
- 可用工具列表
- 使用示例（Claude Code、Codex）
- 错误处理

---

## Phase 6: API文档和测试

### 6.1 OpenAPI文档

使用swaggo自动生成：
```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g backend/cmd/server/main.go
```

访问：`http://localhost:8080/swagger/index.html`

### 6.2 集成测试

**新建文件：** `backend/integration/mcp_integration_test.go`

测试场景：
- MCP客户端连接和认证
- 执行各种工具调用
- 验证返回结果
- 测试错误处理

---

## 关键文件路径汇总

### 需要修改的文件：

**后端核心：**
- `backend/internal/models/models.go` - 数据模型
- `backend/internal/services/field.go` - 字段服务
- `backend/internal/services/record.go` - 记录服务
- `backend/internal/services/file.go` - 文件服务
- `backend/internal/handlers/field.go` - 字段API
- `backend/internal/handlers/record.go` - 记录API
- `backend/internal/handlers/mcp.go` - MCP处理
- `backend/internal/mcp/tools.go` - MCP工具定义
- `backend/internal/db/migrate.go` - 数据库迁移

**新建文件：**
- `backend/internal/services/query.go` - 查询DSL服务
- `backend/pkg/query/model.go` - 查询模型
- `docs/query-dsl-guide.md` - 查询DSL文档
- `docs/mcp-integration-guide.md` - MCP集成文档

**前端：**
- `frontend/src/views/FieldsView.vue` - 字段管理页
- `frontend/src/views/RecordsView.vue` - 记录管理页
- `frontend/src/components/QueryBuilder.vue` - 查询构建器（新建）
- `frontend/src/utils/fieldTypes.ts` - 字段类型工具

---

## 验证计划

### 1. 字段备注功能
- 创建字段时添加备注
- 验证备注在API中正确返回
- 前端正确显示备注

### 2. 附件字段类型
- 创建attachment类型字段，配置文件类型限制
- 上传文件并创建记录，验证attachment字段
- 验证文件类型和大小限制生效

### 3. 字段类型简化
- 创建select/list字段应被拒绝
- 旧数据正确迁移为text类型
- API响应标记deprecated

### 4. 查询DSL
- 使用DSL构建复杂查询（AND/OR嵌套）
- 验证不同操作符（eq, like, in等）
- 测试排序和分页
- 可视化构建器生成正确DSL

### 5. MCP接口
- 使用Integration Token连接MCP
- 执行完整CRUD流程
- 验证错误处理和权限控制

### 6. 端到端测试
- 使用Claude Code通过MCP操作数据
- 创建数据库→表→字段→记录
- 上传附件并查询
- 验证数据完整性

---

## 实施顺序建议

1. **Phase 1（字段备注）** - 简单，无破坏性
2. **Phase 3（移除select）** - 清理技术债
3. **Phase 2（附件字段）** - 核心功能
4. **Phase 4（查询增强）** - 用户体验
5. **Phase 5（MCP完善）** - AI集成
6. **Phase 6（文档测试）** - 质量保证

每阶段完成后进行测试验证，确保不影响现有功能。
