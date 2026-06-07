[English](CHANGELOG.md) | [中文](CHANGELOG.zh.md)

# 更新日志

本项目的所有重要变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，
本项目遵循 [Semantic Versioning](https://semver.org/spec/v2.0.0.html)。

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
