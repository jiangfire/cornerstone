[English](Migration.md) | [中文](Migration.zh.md)

# Migration Guide

## Overview

`cornerstone migration` is used to migrate table schemas and data from external relational databases into Cornerstone.

Current mapping:

| Source Database Object | Cornerstone Object |
| --- | --- |
| Database / Schema | Database |
| Table | Table |
| Column | Field |
| Row | Record |

Currently supported source databases:

- `sqlite`
- `mysql`
- `postgres`

Currently covered in CI:

- SQLite unit and integration tests
- MySQL 8.4 in GitHub Actions
- PostgreSQL 16 in GitHub Actions

## Command Overview

```bash
cornerstone migration run
cornerstone migration preview
cornerstone migration template
cornerstone migration config create
cornerstone migration config validate
cornerstone migration config list
```

## Quick Start

### 1. Generate Config Template

```bash
cornerstone migration template --output ./migration.yaml
```

Or:

```bash
cornerstone migration config create --output ./migration.yaml
```

### 2. Preview Migration Plan

```bash
cornerstone migration preview --config ./migration.yaml
```

This step only reads source metadata and does not write to Cornerstone. It is recommended to review the following in the preview output:

- `target_database`
- `tables`
- `type_mapping_warnings`
- `migration_strategy`

### 3. Execute Migration

```bash
cornerstone migration run --config ./migration.yaml
```

After successful execution, a JSON report is output containing:

- `migration_id`
- `status`
- `summary`
- `tables`
- `validation`

`migration_id` is used for resuming from a checkpoint.

## Common Usage

### Migrate with Config File

```bash
cornerstone migration run --config ./migration.yaml
```

### Quick One-Off Migration

```bash
cornerstone migration run \
  --source-type mysql \
  --source-dsn "user:pass@tcp(localhost:3306)/shop?parseTime=true" \
  --target-db shop \
  --include-tables "users,orders" \
  --batch-size 500 \
  --validate
```

### Migrate Schema Only, Skip Data

```bash
cornerstone migration run \
  --source-type sqlite \
  --source-dsn ./source.db \
  --target-db imported_demo \
  --skip-data
```

### Dry Run with `run --dry-run`

```bash
cornerstone migration run \
  --source-type postgres \
  --source-dsn "host=127.0.0.1 port=5432 user=postgres password=postgres dbname=shop sslmode=disable" \
  --dry-run
```

This is the same as `preview`, only outputting the migration plan without executing writes.

### Resume from Checkpoint

```bash
cornerstone migration run --config ./migration.yaml --resume mig_20260531_100000
```

## Configuration File

Example:

```yaml
source:
  type: mysql
  dsn: "user:pass@tcp(localhost:3306)/shop?parseTime=true"

target:
  database_name: "shop"

tables:
  include:
    - users
    - orders
  exclude: []
  rename:
    old_users: users

data:
  enabled: true
  batch_size: 500
  pagination_strategy: cursor
  cursor_column: ""
  filters:
    orders: "created_at > '2024-01-01'"
  max_concurrent_tables: 1

mapping:
  overrides:
    jsonb: json
    tinyint(1): boolean

options:
  dry_run: false
  continue_on_error: false
  log_level: info
  validate_after: true
  checkpoint_interval: 100
  rollback_on_failure: table
```

### Key Field Reference

| Field | Description |
| --- | --- |
| `source.type` | Source database type: `sqlite` / `mysql` / `postgres` |
| `source.dsn` | Source database connection string |
| `target.database_name` | Target Cornerstone Database name; auto-derived if empty |
| `tables.include` | Only migrate these tables; empty means all |
| `tables.exclude` | Exclude these tables |
| `tables.rename` | Rename tables during migration |
| `data.enabled` | Whether to migrate data; `false` migrates schema only |
| `data.batch_size` | Number of records to read and write per batch |
| `data.pagination_strategy` | `cursor` or `offset` |
| `data.cursor_column` | Explicitly specify the cursor column; auto-inferred if empty |
| `data.filters` | Append source database filter conditions per table |
| `data.max_concurrent_tables` | Number of tables to migrate concurrently |
| `mapping.overrides` | Override default type mappings |
| `options.continue_on_error` | Whether to continue with the next table after a single table failure |
| `options.validate_after` | Whether to validate after migration completes |
| `options.checkpoint_interval` | Write state file after processing this many records |
| `options.rollback_on_failure` | Rollback strategy on single table failure: `table` / `none` |

## CLI Parameters

`migration run` and `migration preview` share these parameters:

| Parameter | Description |
| --- | --- |
| `--config`, `-c` | Configuration file path |
| `--source-type` | Source database type |
| `--source-dsn` | Source database connection DSN |
| `--target-db` | Target Database name |
| `--include-tables` | Comma-separated table allowlist |
| `--exclude-tables` | Comma-separated table exclusion list |
| `--with-data` | Migrate data, enabled by default |
| `--skip-data` | Migrate schema only |
| `--batch-size` | Batch size, default `500` |
| `--dry-run` | Only output plan, do not execute writes |
| `--type-map-override` | Additional type mapping JSON file |
| `--resume` | Resume from the specified `migration_id` |
| `--validate` | Validate after migration, enabled by default |
| `--continue-on-error` | Continue after a single table failure |
| `--pagination-strategy` | `cursor` / `offset` |
| `--cursor-column` | Specify the cursor column |
| `--checkpoint-interval` | Checkpoint interval |
| `--rollback-on-failure` | `table` / `none` |
| `--max-concurrent-tables` | Number of tables to migrate concurrently |

## Type Mapping

Mapping is automatic based on the source database type.

Common rules:

| Source Type | Cornerstone Type |
| --- | --- |
| `varchar`, `char`, `text` | `string` / `text` |
| `int`, `bigint`, `float`, `double`, `decimal`, `numeric` | `number` |
| `tinyint(1)`, `boolean` | `boolean` |
| `date` | `date` |
| `datetime`, `timestamp`, `timestamptz` | `datetime` |
| `json`, `jsonb` | `json` |
| `array`, `enum`, `set` | `list` |
| `blob`, `bytea`, unrecognized types | Falls back to `string` by default with a warning |

To override the default behavior, provide a JSON file:

```json
{
  "tinyint(1)": "boolean",
  "jsonb": "json",
  "longtext": "text"
}
```

Then run:

```bash
cornerstone migration run --config ./migration.yaml --type-map-override ./type-map.json
```

## Migration Output

### Preview Output

The output of `preview` or `run --dry-run` looks like this:

```json
{
  "source": {
    "type": "sqlite",
    "database": "source"
  },
  "target_database": "shop",
  "tables": [
    {
      "source_table": "users",
      "target_table": "users",
      "fields": 4,
      "estimated_rows": 1200,
      "type_mapping_warnings": [],
      "migration_strategy": "cursor"
    }
  ],
  "total_estimated_rows": 1200
}
```

### Execution Report

The output of `run` looks like this:

```json
{
  "migration_id": "mig_20260531_100000",
  "status": "completed",
  "started_at": "2026-05-31T10:00:00Z",
  "finished_at": "2026-05-31T10:02:30Z",
  "summary": {
    "tables_total": 2,
    "tables_success": 2,
    "tables_failed": 0,
    "records_total": 45230,
    "records_inserted": 45230
  },
  "tables": [
    {
      "source": "users",
      "target": "users",
      "status": "completed",
      "fields_created": 8,
      "records_inserted": 12450
    }
  ],
  "validation": {
    "status": "passed",
    "tables_checked": 2,
    "tables_passed": 2,
    "tables_failed": 0,
    "tables_warnings": 0
  }
}
```

## Resumable Migration & State File

Migration state is saved by default at:

```text
~/.cornerstone/migrations/<migration_id>.state.json
```

The state file contains:

- `migration_id`
- `target_db`
- `status` for each table
- `cursor_column`
- `cursor_value`
- `processed_count`

The current state file only stores the **source type and source database name**, not the full DSN or password.

If the state file is corrupted, the current approach is:

1. Delete the corrupted state file;
2. Re-run without `--resume`;
3. Or initiate a new migration with a new `migration_id`.

## Validation Strategy

When `validate_after` is enabled, the following checks are performed after migration:

1. Schema validation
2. Row count validation
3. Content sampling validation
4. Statistical comparison for numeric / date columns

If discrepancies exist, the final status typically becomes:

- `completed_with_issues`
- or `validation.status = passed_with_warnings`

## Security Recommendations

1. Ensure `MASTER_TOKEN` is available before migration.
2. If the configuration file contains passwords, restrict its permissions to `0600`.
3. For MySQL / PostgreSQL source databases, use a read-only account.
4. Do not commit source database DSNs directly into version control.

## Current Limitations

These are current implementation limitations, not future plans:

1. Only Database / Table / Field / Record are migrated; indexes and constraints are not migrated.
2. `BLOB` / `BYTEA` are not automatically converted to Cornerstone file resources; they fall back to string with a warning.
3. `read_only_hint` is not enforced by all drivers:
   - SQLite enables `PRAGMA query_only = ON`
   - MySQL / PostgreSQL still mainly rely on the read-only permissions of the source database account itself
4. There is currently no `--skip-resume` parameter; when encountering a corrupted state file, follow the "delete state file and re-run" approach.
5. `options.log_level` is currently retained in the configuration structure, but does not yet independently drive migration log level switching.
6. Preview output currently does not include "estimated time"; the focus is on structure, row counts, warnings, and pagination strategy.

## Recommended Workflow

Recommended usage order:

1. `migration template` generates the template
2. Edit `migration.yaml`
3. `migration config validate --config ./migration.yaml`
4. `migration preview --config ./migration.yaml`
5. `migration run --config ./migration.yaml`
6. If interrupted, resume using `migration_id` with `--resume`
