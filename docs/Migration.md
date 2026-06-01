# Migration 指南

## 概述

`cornerstone migration` 用于把外部关系型数据库的表结构和数据迁移到 Cornerstone。

当前映射关系：

| 源数据库对象 | Cornerstone 对象 |
| --- | --- |
| Database / Schema | Database |
| Table | Table |
| Column | Field |
| Row | Record |

当前已支持的源数据库：

- `sqlite`
- `mysql`
- `postgres`

当前 CI 已覆盖：

- SQLite 单元与集成测试
- GitHub Actions 中的 `MySQL 8.4`
- GitHub Actions 中的 `PostgreSQL 16`

## 命令总览

```bash
cornerstone migration run
cornerstone migration preview
cornerstone migration template
cornerstone migration config create
cornerstone migration config validate
cornerstone migration config list
```

## 快速开始

### 1. 生成配置模板

```bash
cornerstone migration template --output ./migration.yaml
```

或者：

```bash
cornerstone migration config create --output ./migration.yaml
```

### 2. 预览迁移计划

```bash
cornerstone migration preview --config ./migration.yaml
```

这一步只读取源库元数据，不会写入 Cornerstone。建议先看预览输出里的：

- `target_database`
- `tables`
- `type_mapping_warnings`
- `migration_strategy`

### 3. 执行迁移

```bash
cornerstone migration run --config ./migration.yaml
```

执行成功后会输出 JSON 报告，其中包含：

- `migration_id`
- `status`
- `summary`
- `tables`
- `validation`

`migration_id` 会用于断点续传。

## 常见用法

### 使用配置文件迁移

```bash
cornerstone migration run --config ./migration.yaml
```

### 单次命令快速迁移

```bash
cornerstone migration run \
  --source-type mysql \
  --source-dsn "user:pass@tcp(localhost:3306)/shop?parseTime=true" \
  --target-db shop \
  --include-tables "users,orders" \
  --batch-size 500 \
  --validate
```

### 只迁移结构，不迁移数据

```bash
cornerstone migration run \
  --source-type sqlite \
  --source-dsn ./source.db \
  --target-db imported_demo \
  --skip-data
```

### 用 `run --dry-run` 做空跑

```bash
cornerstone migration run \
  --source-type postgres \
  --source-dsn "host=127.0.0.1 port=5432 user=postgres password=postgres dbname=shop sslmode=disable" \
  --dry-run
```

这和 `preview` 一样，只输出迁移计划，不执行写入。

### 从断点恢复

```bash
cornerstone migration run --config ./migration.yaml --resume mig_20260531_100000
```

## 配置文件

示例：

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

### 关键字段说明

| 字段 | 说明 |
| --- | --- |
| `source.type` | 源数据库类型：`sqlite` / `mysql` / `postgres` |
| `source.dsn` | 源数据库连接串 |
| `target.database_name` | 目标 Cornerstone Database 名称；为空时自动推导 |
| `tables.include` | 只迁移这些表；空表示全部 |
| `tables.exclude` | 排除这些表 |
| `tables.rename` | 迁移时重命名表 |
| `data.enabled` | 是否迁移数据；`false` 时只建结构 |
| `data.batch_size` | 每批读取和写入的记录数 |
| `data.pagination_strategy` | `cursor` 或 `offset` |
| `data.cursor_column` | 显式指定游标列；为空时自动推断 |
| `data.filters` | 按表追加源库过滤条件 |
| `data.max_concurrent_tables` | 同时迁移的表数 |
| `mapping.overrides` | 覆盖默认类型映射 |
| `options.continue_on_error` | 单表失败后是否继续下一个表 |
| `options.validate_after` | 迁移完成后是否校验 |
| `options.checkpoint_interval` | 每处理多少条记录写一次状态文件 |
| `options.rollback_on_failure` | 单表失败时回滚策略：`table` / `none` |

## CLI 参数

`migration run` 和 `migration preview` 共享这些参数：

| 参数 | 说明 |
| --- | --- |
| `--config`, `-c` | 配置文件路径 |
| `--source-type` | 源数据库类型 |
| `--source-dsn` | 源数据库连接 DSN |
| `--target-db` | 目标 Database 名称 |
| `--include-tables` | 逗号分隔的表白名单 |
| `--exclude-tables` | 逗号分隔的表排除名单 |
| `--with-data` | 迁移数据，默认开启 |
| `--skip-data` | 只迁移结构 |
| `--batch-size` | 批大小，默认 `500` |
| `--dry-run` | 只输出计划，不执行写入 |
| `--type-map-override` | 额外类型映射 JSON 文件 |
| `--resume` | 从指定 `migration_id` 恢复 |
| `--validate` | 迁移后校验，默认开启 |
| `--continue-on-error` | 单表失败后继续 |
| `--pagination-strategy` | `cursor` / `offset` |
| `--cursor-column` | 指定游标列 |
| `--checkpoint-interval` | checkpoint 间隔 |
| `--rollback-on-failure` | `table` / `none` |
| `--max-concurrent-tables` | 同时迁移的表数 |

## 类型映射

默认会根据源库类型做自动映射。

常见规则：

| 源类型 | Cornerstone 类型 |
| --- | --- |
| `varchar`, `char`, `text` | `string` / `text` |
| `int`, `bigint`, `float`, `double`, `decimal`, `numeric` | `number` |
| `tinyint(1)`, `boolean` | `boolean` |
| `date` | `date` |
| `datetime`, `timestamp`, `timestamptz` | `datetime` |
| `json`, `jsonb` | `json` |
| `array`, `enum`, `set` | `list` |
| `blob`, `bytea`, 未识别类型 | 默认回退为 `string`，并给 warning |

如果需要覆盖默认行为，可以提供 JSON 文件：

```json
{
  "tinyint(1)": "boolean",
  "jsonb": "json",
  "longtext": "text"
}
```

然后执行：

```bash
cornerstone migration run --config ./migration.yaml --type-map-override ./type-map.json
```

## 迁移输出

### 预览输出

`preview` 或 `run --dry-run` 输出大致如下：

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

### 执行报告

`run` 输出大致如下：

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

## 断点续传与状态文件

迁移状态默认保存在：

```text
~/.cornerstone/migrations/<migration_id>.state.json
```

状态文件包含：

- `migration_id`
- `target_db`
- 每张表的 `status`
- `cursor_column`
- `cursor_value`
- `processed_count`

当前状态文件只保存**源类型和源库名**，不会保存完整 DSN 或密码。

如果状态文件损坏，当前处理方式是：

1. 删除损坏的状态文件；
2. 不带 `--resume` 重新执行；
3. 或使用新的 `migration_id` 重新发起迁移。

## 校验策略

开启 `validate_after` 后，迁移完成后会执行：

1. 结构校验
2. 行数校验
3. 内容抽样校验
4. 数值列 / 日期列的统计对比

如果存在差异，最终状态通常会变成：

- `completed_with_issues`
- 或 `validation.status = passed_with_warnings`

## 安全建议

1. 迁移前确保 `MASTER_TOKEN` 可用。
2. 配置文件如果包含密码，建议权限收紧到 `0600`。
3. MySQL / PostgreSQL 源库建议使用只读账号。
4. 不要把源库 DSN 直接写入版本库。

## 当前边界

这部分是当前实现边界，不是未来规划：

1. 只迁移 Database / Table / Field / Record，不迁移索引和约束。
2. `BLOB` / `BYTEA` 目前不会自动转成 Cornerstone 文件资源，只会回退为字符串并给 warning。
3. `read_only_hint` 目前不是所有驱动都强制生效：
   - SQLite 会启用 `PRAGMA query_only = ON`
   - MySQL / PostgreSQL 仍主要依赖源库账号本身的只读权限
4. 当前没有 `--skip-resume` 参数；遇到损坏状态文件时，按“删除状态文件后重新执行”处理。
5. `options.log_level` 当前保留在配置结构中，但还没有单独驱动 migration 日志级别切换。
6. 预览输出当前不包含“预计耗时”，重点是结构、行数、warning 和分页策略。

## 推荐流程

建议按这个顺序使用：

1. `migration template` 生成模板
2. 编辑 `migration.yaml`
3. `migration config validate --config ./migration.yaml`
4. `migration preview --config ./migration.yaml`
5. `migration run --config ./migration.yaml`
6. 如中断，使用 `migration_id` 配合 `--resume` 恢复
