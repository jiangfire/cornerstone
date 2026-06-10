# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.7.0] - 2026-06-10

### Added

- **Field selection on record CRUD** - `GET /api/v1/records?fields=name,status` and `GET /api/v1/records/:id?fields=name` return only specified fields
- **YAML import for databases** - `POST /api/v1/databases/import/yaml` accepts YAML body to create databases with tables and fields
- **YAML template download** - `GET /api/v1/databases/import/template` returns an annotated YAML template
- **CLI `db import` command** - `cornerstone db import --file schema.yaml` for YAML-based database creation
- **S3-compatible file storage** - Pluggable `StorageProvider` interface with local and S3 (MinIO) backends, configurable via `FILE_STORAGE_TYPE`
- **Swagger UI** - Online API documentation at `/swagger/index.html`
- **Name resolution** - CLI and API accept database/table names (not just IDs) for most operations
- **Typed response DTOs** - All HTTP responses use `pkg/dto` structs instead of `gin.H` maps, wrapped in `HttpResult`

### Fixed

- **Timestamp field leaks** - Removed `created_at`/`updated_at`/`deleted_at` from all API responses (database bulk create, token list/update, file metadata, CLI record JSON)
- **S3 credential security** - Added `FILE_STORAGE_S3_SECURE` config (default `true`) to prevent plaintext credential transmission over HTTP
- **Download path bug** - Local file download now uses `StorageProvider.Download()` instead of hardcoded `./uploads` path, fixing breakage with custom `FILE_STORAGE_LOCAL_DIR`
- **FileStorage config validation** - Rejects unknown storage types, requires S3 fields when `FILE_STORAGE_TYPE=s3`
- **Entity reload after create** - Database bulk create and file upload now reload entities to populate DB-generated defaults
- **Handler response completeness** - `CreateField`/`UpdateField` now include `options`; `UpdateTable` now includes `database_id`

### Changed

- **S3 uploads include Content-Type** - `StorageProvider.Upload` interface now accepts `contentType` parameter
- **`.env.example`** - Added all `FILE_STORAGE_*` environment variable documentation
- **CLI output noise** - Non-JSON CLI mode sets log level to `fatal` by default

## [v1.6.3] - 2026-06-09

### Fixed

- **CLI `--json` mode** - Suppressed log output to avoid polluting structured JSON on stdout
- **MCP `query_data` parameter structure** - Unified with REST API by removing the `query` wrapper layer; Query DSL fields are now passed directly in `arguments`
- **List field validation messages** - Improved error messages to clearly show the required array format: `e.g. ["admin"] or ["option1", "option2"]`

### Changed

- **Query DSL documentation** - Added clear note about using qualified column names (e.g. `records.id`) in `select` when using JOIN to avoid ambiguous column errors
- **MCP Setup documentation** - Added complete JSON-RPC request examples for all common operations (initialize, list tools, query data, JOIN queries)

## [v1.6.0] - 2026-06-07

### Added

- **Full English internationalization** - All user-facing strings translated from Chinese to English
- **Enhanced MCP tools** - Added full CRUD tools for databases, tables, fields, and records
  - `get_database`, `update_database`, `delete_database`
  - `list_tables`, `get_table`, `update_table`, `delete_table`
  - `list_fields`, `update_field`, `delete_field`
  - `list_records`, `get_record`, `batch_insert_records`
  - `create_database_with_tables` - atomic database + tables + fields creation
- **CLI improvements**
  - `--json` flag for structured JSON output
  - `--token` / `-t` flag for auth token override
  - Semantic exit codes (0=success, 2=validation, 3=not found, 4=permission, 5=server error)
- **New documentation**
  - Token Scopes reference
  - System Architecture overview
  - AI Assistant usage guide
  - MCP Client Setup guide
  - File Handling reference
  - Optimistic Locking guide
  - Contributing guidelines
  - FAQ and troubleshooting

### Changed

- MCP tool responses now use shaped data with RFC3339 timestamps
- `get_table_schema` accepts both `query_table_name` and legacy `table` parameter
- Improved error responses in MCP tools with error codes

### Fixed

- `callCreateTable` now reports field creation errors instead of silently ignoring them

## [v1.5.0] - 2026-06-06

### Added

- Performance benchmarks for SQLite, MySQL, and PostgreSQL
- MySQL record index optimization with composite index forcing
- `record_field_indexes` derived index table for structured filter correctness
- Performance workflow in CI with artifact uploads

### Changed

- Moved performance guidance to README

## [v1.4.1] - 2026-05-28

### Fixed

- MySQL JSON query performance issues
- Record field index synchronization on create/update/delete/batch operations

## [v1.4.0] - 2026-05-20

### Added

- External database migration support (MySQL, PostgreSQL, SQLite)
- Migration preview and dry-run modes
- Configurable type mapping overrides
- Checkpoint/resume for large migrations

## [v1.3.0] - 2026-05-10

### Added

- AI Assistant with LLM integration
- MCP (Model Context Protocol) support
- SSE streaming for MCP notifications
- AI tool execution with permission isolation

## [v1.2.6] - 2026-04-28

### Added

- File upload and management
- File attachment fields
- Field-level file type and size restrictions

## [v1.2.5] - 2026-04-15

### Added

- Batch record creation
- Record export (CSV/JSON)
- Optimistic locking with version numbers

## [v1.2.4] - 2026-04-01

### Added

- Token scope-based permissions
- Field-level access control
- Token expiration support

## [v1.2.3] - 2026-03-20

### Added

- Redis cache backend
- Cache factory pattern
- Global cache management

## [v1.2.2] - 2026-03-10

### Added

- Query DSL with JOIN support
- UNION and INTERSECT operations
- JSON path field access

## [v1.2.1] - 2026-03-01

### Added

- Query DSL explain endpoint
- Query validation endpoint
- Simplified query syntax

## [v1.2.0] - 2026-02-20

### Added

- Query DSL engine
- Aggregations and GROUP BY
- HAVING clause support

## [v1.1.0] - 2026-02-10

### Added

- REST API with Swagger documentation
- Token-based authentication
- Database/Table/Field/Record CRUD operations

## [v1.0.0] - 2026-02-01

### Added

- Initial release
- CLI for database management
- SQLite support
- Basic REST API
