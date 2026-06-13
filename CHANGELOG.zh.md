[English](CHANGELOG.md) | [中文](CHANGELOG.zh.md)

# 更新日志

本项目的所有重要变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，
本项目遵循 [Semantic Versioning](https://semver.org/spec/v2.0.0.html)。

## [v1.7.2] - 2026-06-13

### 变更

- **统一 DTO 包** - 所有请求/响应类型合并到 `pkg/dto`。从 `internal/services/` 删除了 18 个重复类型定义（database、table、field、record、token、file 服务）。Handler 转换函数（`tokenObjectFromResponse`、`tableObjectFromResponse`、`fieldObjectFromResponse`、`fileObjectFromModel`、`recordObjectFromModel`）已删除。
- **RecordListData.Items → Records** - 重命名字段以提高清晰度
- **新增 DTO 类型** - 添加 `dto.RecordListQueryRequest`（简化列表查询）、`dto.BatchQueryData`（批量查询响应），以及 `dto.FieldObject` 的 `Deprecated` 字段

### 修复

- **Swagger 文档已重新生成** - 更新以反映合并后的 dto 类型

## [v1.7.1] - 2026-06-13

### 变更

- **Swagger 模型合并到 `pkg/dto`** - 将所有请求/响应 DTO 类型从 `internal/swagger/models.go` 合并到 `pkg/dto/types.go`，消除了 swagger 模型包，减少了导入复杂度

### 修复

- **S3 配置环境变量读取** - `FILE_STORAGE_S3_SECURE` 现在在 S3 存储提供程序测试中正确从环境变量中读取

## [v1.7.0] - 2026-06-10

### 新增

- **记录字段选择** - `GET /api/v1/records?fields=name,status` 和 `GET /api/v1/records/:id?fields=name` 仅返回指定字段
- **YAML 导入数据库** - `POST /api/v1/databases/import/yaml` 接受 YAML 格式创建数据库、表和字段
- **YAML 模板下载** - `GET /api/v1/databases/import/template` 返回带注释的 YAML 模板
- **CLI `db import` 命令** - `cornerstone db import --file schema.yaml` 通过 YAML 创建数据库
- **S3 兼容文件存储** - 可插拔的 `StorageProvider` 接口，支持本地和 S3（MinIO）后端，通过 `FILE_STORAGE_TYPE` 配置
- **Swagger UI** - 在线 API 文档 `/swagger/index.html`
- **名称引用** - CLI 和 API 大多数操作支持使用数据库名/表名（而非仅 ID）
- **类型化响应 DTO** - 所有 HTTP 响应使用 `pkg/dto` 结构体替代 `gin.H`，统一通过 `HttpResult` 封装

### 修复

- **时间戳字段泄露** - 从所有 API 响应中移除 `created_at`/`updated_at`/`deleted_at`（数据库批量创建、Token 列表/更新、文件元数据、CLI 记录 JSON）
- **S3 凭据安全** - 新增 `FILE_STORAGE_S3_SECURE` 配置（默认 `true`），防止凭据通过 HTTP 明文传输
- **下载路径 Bug** - 本地文件下载改用 `StorageProvider.Download()` 替代硬编码 `./uploads` 路径，修复自定义 `FILE_STORAGE_LOCAL_DIR` 时的下载失败
- **FileStorage 配置校验** - 拒绝未知存储类型，S3 模式下必填字段为空时报错
- **Create 后实体重载** - 数据库批量创建和文件上传后重新加载实体以填充数据库生成的默认值
- **Handler 响应字段完整性** - `CreateField`/`UpdateField` 现在包含 `options`；`UpdateTable` 现在包含 `database_id`

### 变更

- **S3 上传包含 Content-Type** - `StorageProvider.Upload` 接口新增 `contentType` 参数
- **`.env.example`** - 补充所有 `FILE_STORAGE_*` 环境变量说明
- **CLI 输出降噪** - 非 JSON 模式 CLI 默认将日志级别设为 `fatal`

## [v1.6.3] - 2026-06-09

### 修复

- **CLI `--json` 模式** - 抑制日志输出，避免污染 stdout 上的结构化 JSON
- **MCP `query_data` 参数结构** - 与 REST API 统一，移除 `query` 包装层；Query DSL 字段现在直接在 `arguments` 中传递
- **List 字段验证提示** - 改进错误信息，清晰显示所需的数组格式：`例如 ["admin"] 或 ["option1", "option2"]`

### 变更

- **Query DSL 文档** - 新增关于 JOIN 查询时使用限定列名（如 `records.id`）的说明，避免列名歧义错误
- **MCP 配置文档** - 新增完整的 JSON-RPC 请求示例，覆盖所有常用操作（初始化、列出工具、查询数据、JOIN 查询）

## [v1.6.0] - 2026-06-07

### Added

- **Full English internationalization** - 所有面向用户的字符串已从中文翻译为英文
- **Enhanced MCP tools** - 新增数据库、表、字段和记录的完整 CRUD 工具
  - `get_database`, `update_database`, `delete_database`
  - `list_tables`, `get_table`, `update_table`, `delete_table`
  - `list_fields`, `update_field`, `delete_field`
  - `list_records`, `get_record`, `batch_insert_records`
  - `create_database_with_tables` - 原子化数据库 + 表 + 字段创建
- **CLI improvements**
  - `--json` 标志用于结构化 JSON 输出
  - `--token` / `-t` 标志用于覆盖认证令牌
  - 语义化退出码（0=成功，2=验证错误，3=未找到，4=权限错误，5=服务器错误）
- **New documentation**
  - Token Scopes 参考文档
  - System Architecture 概览
  - AI Assistant 使用指南
  - MCP Client Setup 指南
  - File Handling 参考文档
  - Optimistic Locking 指南
  - Contributing 指南
  - FAQ 和故障排查

### Changed

- MCP 工具响应现在使用结构化数据并包含 RFC3339 时间戳
- `get_table_schema` 同时接受 `query_table_name` 和遗留的 `table` 参数
- 改进了 MCP 工具中的错误响应，包含错误码

### Fixed

- `callCreateTable` 现在会报告字段创建错误，不再静默忽略

## [v1.5.0] - 2026-06-06

### Added

- SQLite、MySQL 和 PostgreSQL 的性能基准测试
- MySQL 记录索引优化，支持强制复合索引
- `record_field_indexes` 派生索引表，用于保证结构化过滤的正确性
- CI 中的性能工作流，支持产物上传

### Changed

- 将性能指南移至 README

## [v1.4.1] - 2026-05-28

### Fixed

- MySQL JSON 查询性能问题
- 创建/更新/删除/批量操作时的记录字段索引同步问题

## [v1.4.0] - 2026-05-20

### Added

- 外部数据库迁移支持（MySQL、PostgreSQL、SQLite）
- 迁移预览和 dry-run 模式
- 可配置的类型映射覆盖
- 大型迁移的 checkpoint/resume 功能

## [v1.3.0] - 2026-05-10

### Added

- AI Assistant，集成 LLM
- MCP（Model Context Protocol）支持
- SSE 流式传输用于 MCP 通知
- AI 工具执行，具备权限隔离

## [v1.2.6] - 2026-04-28

### Added

- 文件上传和管理
- 文件附件字段
- 字段级文件类型和大小限制

## [v1.2.5] - 2026-04-15

### Added

- 批量记录创建
- 记录导出（CSV/JSON）
- 基于版本号的乐观锁

## [v1.2.4] - 2026-04-01

### Added

- 基于 Token scope 的权限控制
- 字段级访问控制
- Token 过期支持

## [v1.2.3] - 2026-03-20

### Added

- Redis 缓存后端
- 缓存工厂模式
- 全局缓存管理

## [v1.2.2] - 2026-03-10

### Added

- Query DSL，支持 JOIN
- UNION 和 INTERSECT 操作
- JSON path 字段访问

## [v1.2.1] - 2026-03-01

### Added

- Query DSL explain 端点
- Query DSL 验证端点
- 简化查询语法

## [v1.2.0] - 2026-02-20

### Added

- Query DSL 引擎
- 聚合和 GROUP BY
- HAVING 子句支持

## [v1.1.0] - 2026-02-10

### Added

- REST API 及 Swagger 文档
- 基于 Token 的认证
- 数据库/表/字段/记录的 CRUD 操作

## [v1.0.0] - 2026-02-01

### Added

- 初始版本
- 数据库管理 CLI
- SQLite 支持
- 基础 REST API
