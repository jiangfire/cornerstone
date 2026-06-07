[English](FAQ.md) | [中文](FAQ.zh.md)

# FAQ & Troubleshooting

## General

### Q: Cornerstone did not generate a Master Token after startup

**A:** The Master Token is automatically generated and printed to the console on first startup. If you don't see it:

1. Check the log output, search for `master token` or `token`
2. If the database is initialized but the token table is empty, restarting the service will regenerate it
3. Or preset it via environment variable: `MASTER_TOKEN=cs_your_custom_token`

### Q: How to view the current Master Token

**A:**

```bash
cornerstone token list    # The first one is the Master Token
cornerstone db create test  # If you can create without a Master Token, it means the current one is the Master Token
```

### Q: Which databases are supported

**A:** SQLite (default, zero configuration), MySQL 8.0+, PostgreSQL. Recommendations:

- **Development/Testing**: SQLite
- **Production (heavy JSON queries)**: PostgreSQL
- **Production (MySQL ecosystem)**: MySQL 8.0+

---

## Authentication

### Q: What to do when a Token expires

**A:**

1. Check if the Token has expired (`expires_at` field)
2. Recreate it using the Master Token
3. If you forgot the Master Token, check the service startup logs or look for the record with `is_master = true` in the database `tokens` table

### Q: "permission denied: cannot access this database"

**A:**

1. Confirm the Token's Scope includes the target database: `scopes.databases.db_xxx = "viewer"`
2. Confirm the database exists and has not been deleted
3. The Master Token has full permissions; you can temporarily use it to troubleshoot

### Q: What is the Scope format

**A:** JSON object, example:

```json
{"databases":{"db_xxx":"editor"},"tables":{"tbl_yyy":{"role":"viewer"}}}
```

See [Token Scopes](TokenScopes.md) for details.

---

## Query DSL

### Q: Query DSL query returned no data

**A:**

1. Check if the Token has permission to access the target table
2. Check if the `from` table name is in the allowed list: `records`, `tables`, `databases`, `fields`, `files`, `tokens`
3. Check if the query conditions are correct, especially the `data.xxx` path
4. Use `/api/v1/query/explain` to view the execution plan

### Q: JSON field queries are slow

**A:**

- **SQLite**: JSON query performance is inherently limited; consider reducing data volume or adding indexes
- **PostgreSQL**: Use the `data->>status` syntax; PostgreSQL will optimize automatically
- **MySQL**: Consider using generated columns + indexes. See the Performance section in [README](README.md) for details.

### Q: JOIN query returns "invalid JOIN operator" error

**A:** The JOIN `on` condition only allows `=` and `<>` operators; `>`, `<`, `like`, etc. are not supported.

---

## MCP / AI

### Q: Claude Desktop cannot connect

**A:**

1. Confirm the Cornerstone service is running and the port is accessible
2. Check if the Token is valid
3. Confirm the URL and Token in `claude_desktop_config.json` are correct
4. Check Claude Desktop's MCP logs (Developer -> MCP Logs)

### Q: AI assistant returns "AI agent not configured"

**A:** Configure `LLM_API_KEY` in `.env`, then restart the service.

### Q: MCP tool call failed

**A:**

1. Check if the Token has sufficient permissions (Scope)
2. Check if the parameter format is correct
3. Check the Cornerstone server logs for detailed errors

---

## Migration

### Q: How to resume after a migration is interrupted

**A:**

```bash
cornerstone migration run --config ./migration.yaml --resume mig_xxx
```

Status files are saved in `~/.cornerstone/migrations/`.

### Q: Data inconsistency after migration

**A:**

1. Use `migration run --validate` to validate
2. Check if there are unhandled type mappings in `type_mapping_warnings`
3. Check the database timezone settings of the source and target databases

---

## Performance

### Q: Record list queries are slow

**A:**

1. Confirm database indexes are created: check if `idx_records_table_deleted_created` exists
2. MySQL users: confirm the query uses the correct execution plan (`EXPLAIN`)
3. Reduce the `size` parameter (page size)
4. Use database-level field projection (only return needed fields in `select`)

### Q: How to clear the cache

**A:**

```bash
cornerstone cache clear
```

This clears all in-memory/Redis caches. Typically used after modifying Token Scopes or permissions.

---

## Docker

### Q: Database connection fails after Docker startup

**A:**

1. Confirm the database connection string in `.env` is correct
2. If using Docker Compose, confirm the service startup order (`depends_on`)
3. PostgreSQL/MySQL users: confirm the database is initialized and the user has permissions

### Q: Uploaded files are lost after container restart

**A:** Ensure the `uploads` directory is mounted as a persistent volume:

```yaml
volumes:
  - ./uploads:/app/uploads
```

---

## Development

### Q: Tests fail with "database is locked"

**A:** SQLite concurrency issue. When running tests:

```bash
go test ./... -p 1    # Run serially
```

Or use PostgreSQL/MySQL for testing.

### Q: Swagger documentation is not updated

**A:** Regenerate after modifying handler annotations:

```bash
make swagger
```

---

## Still Stuck?

- Check [GitHub Issues](https://github.com/jiangfire/cornerstone/issues)
- Check [Architecture](Architecture.md) to understand system components
- Enable `LOG_LEVEL=debug` for detailed logs
