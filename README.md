# Cornerstone

[English](README.md) | [中文](README.zh.md)

> Self-hosted structured data platform. Single binary, zero external dependencies, CLI + REST API dual-mode interaction.

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Tests](https://github.com/jiangfire/cornerstone/actions/workflows/ci.yml/badge.svg)](https://github.com/jiangfire/cornerstone/actions/workflows/ci.yml)

Cornerstone is designed for developers and teams who need **lightweight, controllable, and programmable** data management. It provides database-level structural definitions (database/table/field/record) and fine-grained permission controls, while supporting external database migration, AI assistant, and MCP protocol integration.

Compared to SaaS platforms like Airtable/Notion, Cornerstone gives you **full control over your data**; compared to building your own database + ORM, it gives you a **complete data management backend in minutes**.

---

## Quick Start

### Docker (Recommended)

```bash
docker compose up -d --build
```

### Build from Source

```bash
make build    # Build binary
make dev      # Start development server
```

Then use CLI or REST API to manage data:

```bash
# CLI
cornerstone db create mydb
cornerstone table create <db-id> users
cornerstone field create <table-id> name string --required
cornerstone record create <table-id> '{"name":"John"}'

# REST API
curl http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer <token>"
```

---

## Core Features

- **Dual-mode Interaction**: CLI for scripting automation, REST API for application integration
- **Fine-grained Permissions**: Token-level database/table permission control
- **External Migration**: One-click migration from MySQL / PostgreSQL / SQLite to Cornerstone
- **AI Ready**: Built-in AI assistant with MCP protocol support, allowing AI Agents to directly operate on data
- **Query DSL**: SQL-like JSON query language supporting filtering, sorting, aggregation, and JOIN
- **Lightweight Deployment**: Single binary, runs on SQLite with minimal resource usage

---

## Documentation

| Document | Description |
|----------|-------------|
| [Query DSL](docs/Query.md) | JSON query language syntax and examples |
| [Migration](docs/Migration.md) | External database migration guide |
| [Token Scopes](docs/TokenScopes.md) | Permission configuration and scope format |
| [Architecture](docs/Architecture.md) | System architecture and component overview |
| [AI Assistant](docs/AI-Assistant.md) | AI assistant usage guide |
| [MCP Setup](docs/MCP-Setup.md) | MCP client configuration (Claude Desktop, etc.) |
| [File Handling](docs/File-Handling.md) | File upload, download, and limits |
| [Optimistic Locking](docs/Optimistic-Locking.md) | Optimistic locking mechanism and usage |
| [FAQ](docs/FAQ.md) | Common questions and troubleshooting |
| [Contributing](CONTRIBUTING.md) | Contribution guide |
| [Changelog](CHANGELOG.md) | Version update log |

---

## Configuration

Copy `.env.example` to `.env` and modify as needed:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_TYPE` | `sqlite`, `postgres`, or `mysql` (MySQL 8.0+) | `sqlite` |
| `DATABASE_URL` | Database connection string | `./cornerstone.db` |
| `DB_MAX_OPEN` | Maximum open database connections | `10` |
| `DB_MAX_IDLE` | Maximum idle database connections | `5` |
| `DB_MAX_LIFETIME` | Maximum connection lifetime (seconds) | `3600` |
| `SERVER_MODE` | `release` or `debug` | `release` |
| `PORT` | Server port | `8080` |
| `LOG_LEVEL` | Log level | `info` |
| `MASTER_TOKEN` | Master Token (leave empty to disable Master Token auth) | - |
| `LLM_API_KEY` | LLM API Key (enables AI assistant) | - |
| `LLM_MODEL` | LLM model name | `gpt-4o` |
| `LLM_BASE_URL` | Custom LLM API URL | - |
| `MCP_ALLOWED_ORIGINS` | MCP allowed origins (comma-separated) | (empty) |
| `MCP_SSE_KEEPALIVE_SEC` | SSE heartbeat interval (seconds) | `25` |
| `MCP_SSE_RETRY_MS` | SSE reconnection interval (milliseconds) | `3000` |
| `MCP_SSE_REPLAY_BUFFER` | SSE replay buffer size | `128` |
| `REDIS_URL` | Redis connection string (leave empty for in-memory cache) | - |

---

## CLI Usage

```bash
cornerstone serve                    # Start HTTP API + MCP server

# Data Management
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

# Token and Permissions
cornerstone token list
cornerstone token create <name> [-s scopes] [-e expires]
cornerstone token update|delete <id>

# External Database Migration
cornerstone migration run [-c config] [--source-type mysql|postgres|sqlite] [--source-dsn ...] [--target-db ...]
cornerstone migration preview
cornerstone migration template

# Other
cornerstone cache clear
cornerstone migrate                  # Execute database schema migration
cornerstone --version
```

---

## REST API

After starting the server (`cornerstone serve`), all requests are authenticated via `Authorization: Bearer <token>`.

> All endpoints use the `/api/v1/` prefix; legacy `/api/` paths automatically redirect to `/api/v1/` for backward compatibility.

### Endpoint List

| Domain | Method | Path | Description |
|--------|--------|------|-------------|
| Token | GET | `/api/v1/tokens` | List tokens |
| Token | POST | `/api/v1/tokens` | Create token |
| Token | PUT | `/api/v1/tokens/{id}` | Update token |
| Token | DELETE | `/api/v1/tokens/{id}` | Delete token |
| Database | GET | `/api/v1/databases` | List databases |
| Database | POST | `/api/v1/databases` | Create database |
| Database | GET | `/api/v1/databases/{id}` | Get database |
| Database | PUT | `/api/v1/databases/{id}` | Update database |
| Database | DELETE | `/api/v1/databases/{id}` | Delete database |
| Database | POST | `/api/v1/databases/with-tables` | One-click database + table + field creation |
| Table | GET | `/api/v1/databases/{id}/tables` | List tables |
| Table | POST | `/api/v1/tables` | Create table |
| Table | GET | `/api/v1/tables/{id}` | Get table |
| Table | PUT | `/api/v1/tables/{id}` | Update table |
| Table | DELETE | `/api/v1/tables/{id}` | Delete table |
| Field | GET | `/api/v1/tables/{id}/fields` | List fields |
| Field | POST | `/api/v1/fields` | Create field |
| Field | GET | `/api/v1/fields/{id}` | Get field |
| Field | PUT | `/api/v1/fields/{id}` | Update field |
| Field | DELETE | `/api/v1/fields/{id}` | Delete field |
| Record | GET | `/api/v1/records` | List records |
| Record | POST | `/api/v1/records` | Create record |
| Record | GET | `/api/v1/records/{id}` | Get record |
| Record | PUT | `/api/v1/records/{id}` | Update record |
| Record | DELETE | `/api/v1/records/{id}` | Delete record |
| Record | POST | `/api/v1/records/batch` | Batch create records |
| Record | GET | `/api/v1/records/export` | Export records |
| File | POST | `/api/v1/files/upload` | Upload file |
| File | GET | `/api/v1/files/{id}` | Get file info |
| File | GET | `/api/v1/files/{id}/download` | Download file |
| File | DELETE | `/api/v1/files/{id}` | Delete file |
| File | GET | `/api/v1/records/{id}/files` | List record-associated files |
| Query | POST | `/api/v1/query` | Query DSL query |
| Query | GET | `/api/v1/query` | Query DSL query (GET) |
| Query | GET | `/api/v1/query/simple` | Simplified query |
| Query | POST | `/api/v1/query/batch` | Batch query |
| Query | POST | `/api/v1/query/explain` | Query explanation |
| Query | POST | `/api/v1/query/validate` | Validate query |
| Query | GET | `/api/v1/query/tables` | Accessible tables list |
| Query | GET | `/api/v1/query/schema/{table}` | Table schema |
| AI | POST | `/api/v1/ai/chat` | AI assistant chat |
| MCP | POST | `/mcp` | MCP protocol (JSON-RPC) |
| MCP | GET | `/mcp` | MCP SSE event stream |
| Monitoring | GET | `/metrics` | Prometheus metrics |
| Health | GET | `/health` | Health probe |
| Readiness | GET | `/ready` | Readiness probe |

### Request Examples

```bash
# Create database
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"name": "TestDB", "description": "For testing"}'

# Create record
curl -X POST http://localhost:8080/api/v1/records \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"table_id": "tbl_xxx", "data": {"name": "John", "age": 28}}'

# Query DSL query
curl -X POST http://localhost:8080/api/v1/query \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", "select": ["id", "data"], "where": {"and": [{"field": "table_id", "op": "eq", "value": "tbl_xxx"}]}, "page": 1, "size": 20}'
```

---

## Data Model

```text
Database --1:N--> Table --1:N--> Field
                  Table --1:N--> Record --1:N--> File
```

You can freely define databases, tables, and field structures via API or CLI, without pre-compiled migration scripts. Records are stored as JSONB with optimistic locking version control. File attachments are associated with records and support permission isolation.

---

## Performance and Database Selection

Cornerstone supports SQLite, MySQL 8.0+, and PostgreSQL. Different backends serve different purposes:

| Backend | Recommended Use | Performance Summary |
|---------|-----------------|---------------------|
| SQLite | Local development, CI quick regression, small-scale self-hosting | Low startup cost, suitable for smoke benchmarks; not recommended for JSON-heavy production queries |
| PostgreSQL | Production deployments with many JSON conditional queries | Most stable JSON query performance in current benchmarks, recommended as the default JSON-heavy production backend |
| MySQL 8.0+ | Production deployments requiring MySQL ecosystem compatibility | Regular list path optimized; JSON hot fields require generated columns or `JSON_VALUE()` expression indexes |

Key optimizations already implemented:

- Record list primary path uses composite index `idx_records_table_deleted_created(table_id, deleted_at, created_at DESC)`.
- MySQL record list uses `FORCE INDEX (idx_records_table_deleted_created)` to stabilize regular pagination / COUNT execution plans.
- `record_field_indexes` derived index table serves as the correctness foundation for dynamic field equality filtering, and covers create/update/delete/batch write synchronization and historical backfill.
- A standalone Performance workflow runs benchmarks on SQLite / MySQL / PostgreSQL, and uploads `auth.txt`, `services.txt`, `query.txt`, `explain.txt` artifacts.

Current MySQL JSON conclusion:

- `JSON_EXTRACT(data, ?) = ?` is still row-by-row JSON function filtering, and should not be relied upon as a high-performance MySQL JSON query solution long-term.
- The current `record_field_indexes` query form is functional, but slower than raw `JSON_EXTRACT` in existing benchmarks, and should not be promised as a MySQL performance improvement.
- For a small number of stable hot fields, prioritize generated columns or `JSON_VALUE()` expression indexes, combined with composite indexes like `(table_id, deleted_at, derived_col, created_at DESC)`.
- For related MySQL capabilities, refer to the official documentation: [JSON_VALUE() and JSON Expression Indexes](https://dev.mysql.com/doc/refman/8.0/en/json-search-functions.html), [Generated Column Secondary Indexes](https://dev.mysql.com/doc/refman/8.0/en/create-table-secondary-indexes.html).

---

## Authentication

All API requests (except `/health`, `/ready`, `/metrics`) must carry a token:

```http
Authorization: Bearer <token>
```

Alternatively, you can use the `X-API-Key` header as an alternative (priority higher than `Authorization: Bearer`):

```http
X-API-Key: <token>
```

- **Master Token**: Automatically generated at startup (or preset via `MASTER_TOKEN` environment variable), has full permissions
- **Regular Token**: Created by Master Token via `POST /api/v1/tokens`, can be configured with database/table-level permission scopes

---

## MCP Protocol

Cornerstone natively supports [MCP (Model Context Protocol)](https://modelcontextprotocol.io/), allowing AI Agents to directly read and write your data through a standard protocol, without writing custom integration code.

Connection methods:
- **SSE Event Stream**: `GET /mcp` (`Accept: text/event-stream`)
- **JSON-RPC Request**: `POST /mcp`

---

## AI Assistant

Enable: configure `LLM_API_KEY` in `.env`.

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Help me create a users table"}'
```

Supports natural language database creation, table creation, data querying, and test data generation. The AI assistant understands Cornerstone's data model and API, and can directly invoke internal tools to complete operations.

---

## Query DSL

Describe queries through JSON, supporting filtering, sorting, aggregation, and JOIN. No need to hand-write SQL for complex data queries. See [Query DSL Documentation](docs/Query.md) for details.

---

## Development

```bash
make build          # Build binary (output to bin/)
make test           # Run all tests (including race detection)
make test-cover     # Run tests and generate coverage report
make lint           # Run golangci-lint
make check          # Full check (fmt + vet + test)
make swagger        # Regenerate Swagger documentation
make dev            # Start local development server
```

---

## Testing

```bash
go test ./...                           # Run all tests
go test ./... -coverprofile=coverage.out # Generate coverage report
go tool cover -func=coverage.out        # View function-level coverage
```

Core package test coverage 80%+, CI includes MySQL/PostgreSQL migration integration tests, golangci-lint, govulncheck, and Trivy security scanning.

### Performance Benchmarks

```bash
go test ./internal/middleware -run ^$ -bench BenchmarkValidateToken -benchmem -count 1
go test ./internal/services -run ^$ -bench BenchmarkFieldServiceListFields -benchmem -count 1
go test ./internal/services -run ^$ -bench BenchmarkRecordServiceListRecords -benchmem -count 1
go test ./pkg/query -run ^$ -bench BenchmarkExecutorExecute -benchmem -count 1
go test ./internal/services -run TestExplainPlanListRecords -v
```

Defaults to local SQLite. After setting `DB_TYPE` and `DATABASE_URL`, the same benchmarks can switch to MySQL or PostgreSQL. The [Performance workflow](https://github.com/jiangfire/cornerstone/actions/workflows/perf.yml) in GitHub Actions automatically runs on all three database backends and retains raw benchmark / EXPLAIN artifacts.

---

## License

AGPL-3.0
