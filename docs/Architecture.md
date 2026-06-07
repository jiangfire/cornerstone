[English](Architecture.md) | [中文](Architecture.zh.md)

# Architecture

> Cornerstone system architecture and component overview.

---

## Overall Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
├─────────────┬─────────────┬─────────────┬───────────────────┤
│   CLI Tool  │  REST API   │    MCP      │   AI Assistant    │
│  (cobra)    │  (gin)      │  (SSE/JSON) │   (LLM + Tools)   │
└──────┬──────┴──────┬──────┴──────┬──────┴─────────┬─────────┘
       │             │             │                │
       └─────────────┴─────────────┴────────────────┘
                         │
              ┌──────────▼──────────┐
              │   Service Layer      │
              │  (business logic)    │
              └──────────┬──────────┘
                         │
              ┌──────────▼──────────┐
              │   Data Layer         │
              │  (SQLite/MySQL/      │
              │   PostgreSQL)        │
              └─────────────────────┘
```

---

## Component Overview

### 1. CLI (cmd/main.go + internal/cli/)

A command-line tool built on [Cobra](https://github.com/spf13/cobra) that provides a CLI for all data management operations.

- **Characteristics**: Zero dependencies (except for the database), ideal for script automation
- **Global flags**:
  - `--json`: Output structured JSON (suitable for pipeline processing)
  - `--token` / `-t`: Specify authentication token (overrides the `MASTER_TOKEN` environment variable)
- **Semantic exit codes**:
  - `0` - Success
  - `1` - General error
  - `2` - Validation error
  - `3` - Resource not found
  - `4` - Insufficient permissions
  - `5` - Server error

### 2. REST API (internal/handlers/)

An HTTP API server built on [Gin](https://gin-gonic.com/). All endpoints are prefixed with `/api/v1/`.

- **Authentication**: `Authorization: Bearer <token>` or `X-API-Key: <token>`
- **Swagger docs**: Visit `/swagger/index.html` after starting the server
- **Endpoint groups**:
  - `/api/v1/tokens` - Token management
  - `/api/v1/databases` - Database management
  - `/api/v1/tables` - Table management
  - `/api/v1/fields` - Field management
  - `/api/v1/records` - Record management
  - `/api/v1/files` - File management
  - `/api/v1/query` - Query DSL queries
  - `/api/v1/ai/chat` - AI assistant
  - `/mcp` - MCP protocol endpoints
  - `/health`, `/ready`, `/metrics` - Health and monitoring probes

### 3. MCP Protocol (internal/mcp/)

Native support for the [Model Context Protocol](https://modelcontextprotocol.io/). AI agents can operate on data through the standard protocol.

- **Transport methods**:
  - SSE stream: `GET /mcp` (`Accept: text/event-stream`)
  - JSON-RPC: `POST /mcp`
- **Tool list**: query_data, create_database, list_databases, get_database, update_database, delete_database, create_database_with_tables, create_table, list_tables, get_table, update_table, delete_table, create_field, list_fields, update_field, delete_field, insert_record, list_records, get_record, update_record, delete_record, batch_insert_records, generate_test_data, get_table_schema
- **Authentication**: Shares the same token-based authentication as the REST API

### 4. AI Assistant (internal/handlers/ai.go + internal/services/ai_*.go)

An LLM-integrated data assistant that supports natural language interaction.

- **Configuration**: `LLM_API_KEY`, `LLM_MODEL`, `LLM_BASE_URL`
- **Capabilities**:
  - Query data (via Query DSL)
  - Create / modify database schemas
  - Insert / update / delete records
  - Generate test data
- **Tool invocation**: The AI calls internal services through `ExecuteAIToolForToken`, with the same permissions as a regular token

### 5. Query DSL (pkg/query/)

A SQL-like JSON query language that supports:

- Filtering (where / having)
- Sorting (orderBy)
- Pagination (page / size)
- Aggregation (groupBy + aggregate)
- JOINs (left / right / inner / outer)
- UNION / INTERSECT
- Automatic permission filtering (conditions are automatically injected based on token scope)

### 6. Authorization System (internal/authz/)

- **Master Token**: Full permissions
- **Regular Token**: Fine-grained control based on JSON Scope
- **Caching**: Token and permission context are cached for 5 minutes
- **Field-level permissions**: Whitelist-based field access control is supported

### 7. Data Layer (pkg/db/)

Supports three database backends:

| Backend | Use Case | Features |
|---------|----------|----------|
| SQLite | Local development, CI, small-scale deployments | Zero configuration, file-as-database |
| PostgreSQL | Production environments with heavy JSON queries | Excellent JSONB performance |
| MySQL 8.0+ | MySQL ecosystem compatibility | Supports generated column index optimization |

### 8. Cache (pkg/cache/)

- **In-memory cache**: Default, no external dependencies required
- **Redis cache**: Automatically switches when `REDIS_URL` is configured
- **Cache types**: Token cache, permission context cache, field definition cache

### 9. File Storage (internal/services/file.go)

- **Storage path**: `./uploads` (relative to the working directory)
- **Security validation**: All paths are validated through `ResolveSecureStoragePath` to prevent directory traversal
- **Default limits**: Single file max 10 MB, supports `.jpg/.jpeg/.png/.gif/.pdf/.doc/.docx/.xls/.xlsx/.txt/.zip`
- **Field-level limits**: `max_file_size_mb` and `allowed_types` can be set in the field configuration

---

## Request Flow

```
Client Request
    │
    ├─ CLI ──────┐
    ├─ REST API ─┼──> internal/handlers/ ──> internal/services/ ──> pkg/db/ ──> Database
    ├─ MCP ──────┤         │                      │
    └─ AI ───────┘    middleware/auth.go    internal/authz/
                         (Token validation)    (Permission check)
```

1. **Authentication**: `middleware/auth.go` extracts the token and validates its validity
2. **Authorization**: `internal/authz/` determines whether the operation is permitted based on the token scope
3. **Business logic**: `internal/services/` executes the specific business operations
4. **Data access**: `pkg/db/` accesses the database through GORM

---

## Deployment Architecture

### Single-Node Deployment (SQLite)

```
┌─────────────────┐
│  Cornerstone    │
│  (single binary)│
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

Suitable for: personal development, small teams, CI testing

### Production Deployment (PostgreSQL/MySQL + Optional Redis)

```
┌─────────────────┐     ┌─────────────┐     ┌─────────────┐
│  Cornerstone    │────▶│ PostgreSQL  │     │   Redis     │
│  (container)    │     │  (data)     │     │  (cache)    │
│                 │     └─────────────┘     └─────────────┘
│  ┌───────────┐  │
│  │  uploads/ │  │  ← persistent volume mount
│  └───────────┘  │
└─────────────────┘
```

Suitable for: production environments, multi-instance deployments

---

## Extension Points

| Extension Point | Description |
|-----------------|-------------|
| Custom LLM | Configure `LLM_BASE_URL` to connect to any OpenAI-compatible API |
| Custom cache | Implement the `pkg/cache.Cache` interface and register it through the factory |
| Custom migration | Extend the type mapping in `internal/migration/mapper/` |
| Custom MCP tools | Extend `ListTools()` in `internal/mcp/tools.go` |

---

## Related Documentation

- [Query DSL](Query.md) - Detailed query engine documentation
- [Migration](Migration.md) - External database migration
- [Token Scopes](TokenScopes.md) - Permission configuration reference
