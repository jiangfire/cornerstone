# Cornerstone 外部数据库迁移功能规格书

## 1. 功能概述

Migration 功能允许用户通过 CLI 将外部关系型数据库（MySQL、PostgreSQL、SQLite 等）的**库结构**与**数据**迁移到 Cornerstone 系统中。

迁移的映射关系：

| 外部数据库 | Cornerstone |
|-----------|-------------|
| Database (Schema) | Database |
| Table | Table |
| Column | Field |
| Row | Record |
| Index / Constraint | （暂不迁移，仅做类型推断参考） |

---

## 2. 竞品分析与优秀实践

### 2.1 竞品工具对比

| 工具 | 定位 | 核心特性 | 可借鉴之处 |
|------|------|----------|-----------|
| **Liquibase** | Schema 变更管理 | Changelog / XML/YAML/JSON 格式、内置回滚、支持 50+ 数据库 | 变更集原子化、版本标记、校验和验证 |
| **Flyway** | Schema 变更管理 | SQL 优先、线性迁移、文件编号排序 | 简单直观的版本控制、CI/CD 集成 |
| **MySQL2PG** | MySQL → PostgreSQL 数据迁移 | 类型自动映射、批量插入、数据校验 | 类型映射表设计、批量性能优化 |
| **Debezium + Kafka** | CDC 增量同步 | 基于 binlog/WAL、断点续传、至少一次投递 | 增量迁移架构、位点持久化 |
| **Strapi Import** | Headless CMS 数据导入 | 内置 `import`/`export` CLI、Schema 绑定 | 配置化导入、媒体文件处理 |
| **Directus Schema Sync** | Headless CMS 结构同步 | Schema API diff/apply、扩展机制 | 结构预览与差异对比、环境间推广 |
| **阿里云 DTS / 华为云 DRS** | 云上一体化迁移 | 全量+增量一体化、自动重试、冲突策略 | 全量/增量一体化设计、自动重试机制 |

### 2.2 业界核心原则

1. **预览先行（Dry-Run First）**：任何写入操作前，先生成迁移计划并输出差异报告，供人工审核。Liquibase 的 `updateSQL`、Flyway 的 `info` 命令均遵循此原则。
2. **原子变更（Atomic Changeset）**：每个变更集只做一件事，确保精确回滚和清晰审计。单表迁移失败时不应污染其他已成功的表。
3. **标记状态（Tag Before Migrate）**：迁移前标记当前系统状态，为回滚提供明确锚点。
4. **验证完整性（Validate After）**：迁移后执行三级校验 —— 结构校验 → 行数校验 → 内容抽样校验。
5. **断点续传（Resumable）**：大表迁移需持久化进度（cursor 位点、已处理记录数），避免网络抖动或进程重启导致全量重做。
6. **游标分页替代 OFFSET**：`OFFSET` 分页在高偏移量时性能线性退化（读取并丢弃大量行），应使用 Keyset/Cursor 分页（`WHERE id > lastId ORDER BY id LIMIT N`）保持恒定性能。

### 2.3 现状与约束

### 2.3.1 现有体系

- CLI 使用 Cobra 框架，命令位于 `internal/cli/`
- 服务层通过 `DatabaseService`、`TableService`、`FieldService`、`RecordService` 操作 GORM 模型
- 认证依赖 `MASTER_TOKEN` 环境变量
- Record 数据以 JSON 字符串形式存储在 `records.data` 字段中
- Cornerstone 字段类型固定为：`string`, `text`, `number`, `boolean`, `date`, `datetime`, `file`, `json`, `list`

### 2.2 设计约束

- 不修改现有 `migrate` 命令（内部表结构迁移），新增 `migration` 命令族
- 保持 CLI 与服务层现有调用方式一致
- 大数据量迁移需支持分页/分批，避免内存溢出
- 源数据库只读访问，不对源库做写操作
- 迁移过程需支持断点续传（持久化 cursor 位点）
- 迁移后需支持数据一致性校验（行数 + 抽样）

---

## 3. CLI 命令设计

### 3.1 命令结构

```
cornerstone migration
├── run       # 执行迁移
├── preview   # 预览迁移计划（只读，不写入 Cornerstone）
├── config    # 管理迁移配置文件
│   ├── create
│   ├── validate
│   └── list
└── template  # 输出示例配置文件
```

### 3.2 `migration run` — 执行迁移

```bash
# 通过配置文件执行
cornerstone migration run --config ./migration.yaml

# 通过命令行参数快速执行（单表迁移）
cornerstone migration run \
  --source-type mysql \
  --source-dsn "user:pass@tcp(localhost:3306)/source_db" \
  --target-db "target_db_name" \
  --include-tables "users,orders" \
  --with-data \
  --batch-size 500
```

**参数说明：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--config` / `-c` | string | 否 | 迁移配置文件路径，与命令行参数互斥 |
| `--source-type` | string | 条件 | 源数据库类型：`mysql`, `postgres`, `sqlite` |
| `--source-dsn` | string | 条件 | 源数据库连接 DSN |
| `--target-db` | string | 否 | 目标 Cornerstone Database 名称，默认使用源库名 |
| `--include-tables` | string | 否 | 要迁移的表，逗号分隔，默认迁移全部 |
| `--exclude-tables` | string | 否 | 要排除的表，逗号分隔 |
| `--with-data` | bool | 否 | 是否迁移数据，默认 `true` |
| `--skip-data` | bool | 否 | 是否跳过数据，仅迁移结构 |
| `--batch-size` | int | 否 | 数据批量插入大小，默认 `500`（小记录建议 200~500，大记录建议 50~100） |
| `--dry-run` | bool | 否 | 空跑模式：解析并打印计划，不写入 Cornerstone |
| `--type-map-override` | string | 否 | 自定义类型映射 JSON 文件路径 |
| `--resume` | string | 否 | 从指定的迁移任务 ID 断点续传 |
| `--validate` | bool | 否 | 迁移后执行数据校验，默认 `true` |
| `--continue-on-error` | bool | 否 | 单表/单字段错误时不中断，记录失败日志后继续 |

### 3.3 `migration preview` — 预览迁移计划

```bash
cornerstone migration preview --config ./migration.yaml
```

输出示例：
```json
{
  "source": {
    "type": "mysql",
    "database": "ecommerce"
  },
  "target_database": "ecommerce",
  "tables": [
    {
      "source_table": "users",
      "target_table": "users",
      "fields": 8,
      "estimated_rows": 12450,
      "type_mapping_warnings": [
        "column 'avatar' (BLOB) → mapped to 'file' (manual review suggested)"
      ],
      "migration_strategy": "cursor-based",
      "estimated_duration": "2m30s"
    }
  ],
  "total_estimated_rows": 45230
}
```

### 3.4 `migration template` — 生成配置模板

```bash
# 输出默认模板到 stdout
cornerstone migration template

# 直接生成到文件
cornerstone migration template --output ./migration.yaml
```

---

## 4. 配置文件规格

### 4.1 YAML 配置示例

```yaml
# migration.yaml
source:
  type: mysql              # mysql | postgres | sqlite
  dsn: "user:pass@tcp(localhost:3306)/source_db"
  # 或分字段配置（与 dsn 互斥）
  host: localhost
  port: 3306
  user: user
  password: pass
  database: source_db
  # SSL / 连接参数
  params:
    charset: utf8mb4
    parseTime: "true"
  # 只读连接设置
  read_only_hint: true     # 提示驱动使用只读会话（如 MySQL SESSION TRANSACTION READ ONLY）

target:
  database_name: ""        # 空字符串则使用源库名
  # 如果目标 Database 已存在，复用；否则自动创建

tables:
  include: []              # 空数组 = 全部
  exclude:
    - "_migration_log"
    - "schema_migrations"
  rename:
    old_users: "users"     # 源表名 → 目标表名

data:
  enabled: true
  batch_size: 500
  pagination_strategy: cursor   # cursor | offset，cursor 为默认推荐
  # 游标分页配置（cursor 策略时生效）
  cursor_column: ""             # 空字符串则自动选择主键或第一个索引列
  # 数据过滤（可选，按表配置）
  filters:
    orders: "created_at > '2024-01-01'"
  # 并发控制
  max_concurrent_tables: 1      # 同时迁移的表数，默认 1（避免目标库压力过大）

mapping:
  # 自定义类型映射，覆盖默认值
  overrides:
    "tinyint(1)": boolean
    "jsonb": json
    "longtext": text

options:
  dry_run: false
  continue_on_error: true
  log_level: info          # debug | info | warn | error
  validate_after: true     # 迁移后自动执行数据校验
  checkpoint_interval: 100 # 每处理 N 条记录持久化一次位点（用于断点续传）
  # 回滚策略
  rollback_on_failure: table   # table | none，table=单表失败时回滚该表已写入数据
```

---

## 5. 类型映射表

### 5.1 MySQL → Cornerstone

| MySQL 类型 | Cornerstone 类型 | 说明 |
|-----------|-----------------|------|
| `VARCHAR`, `CHAR`, `TINYTEXT` | `string` | 默认字符串 |
| `TEXT`, `MEDIUMTEXT`, `LONGTEXT` | `text` | 长文本 |
| `INT`, `BIGINT`, `FLOAT`, `DOUBLE`, `DECIMAL`, `NUMERIC` | `number` | 数值统一为 number |
| `TINYINT(1)` | `boolean` | 布尔语义推断 |
| `DATE` | `date` | 日期 |
| `DATETIME`, `TIMESTAMP` | `datetime` | 日期时间 |
| `JSON` | `json` | JSON 对象/数组 |
| `ENUM`, `SET` | `list` | 选项列表 |
| `BLOB`, `BINARY`, `VARBINARY` | `string` ⚠️ | 标记警告，建议人工审核 |
| 其他未识别类型 | `string` ⚠️ | 兜底映射，输出警告 |

### 5.2 PostgreSQL → Cornerstone

| PostgreSQL 类型 | Cornerstone 类型 | 说明 |
|----------------|-----------------|------|
| `VARCHAR`, `CHAR`, `TEXT` | `string` / `text` | 根据长度阈值区分（默认 >5000 为 text） |
| `INTEGER`, `BIGINT`, `SMALLINT`, `REAL`, `DOUBLE PRECISION`, `NUMERIC`, `DECIMAL` | `number` | |
| `BOOLEAN` | `boolean` | |
| `DATE` | `date` | |
| `TIMESTAMP`, `TIMESTAMPTZ` | `datetime` | |
| `JSON`, `JSONB` | `json` | |
| `ARRAY` | `list` | 推断元素类型为 list 内部类型 |
| `BYTEA` | `string` ⚠️ | 标记警告 |
| `UUID`, `INET`, `CIDR`, `MACADDR` | `string` | 字符串存储 |
| 其他未识别类型 | `string` ⚠️ | 兜底映射，输出警告 |

### 5.3 SQLite → Cornerstone

| SQLite 亲和类型 | Cornerstone 类型 | 说明 |
|----------------|-----------------|------|
| `TEXT` | `string` / `text` | 根据长度推断 |
| `INTEGER` | `number` / `boolean` | TINYINT(1) 语义推断为 boolean |
| `REAL`, `NUMERIC` | `number` | |
| `BLOB` | `string` ⚠️ | 标记警告 |
| 无类型声明 | `string` ⚠️ | 兜底 |

### 5.4 映射规则优先级

1. 用户自定义映射（`mapping.overrides`）
2. 数据库类型精确匹配
3. 类型前缀/家族匹配（如 `VARCHAR(255)` 匹配 `VARCHAR`）
4. SQLite 亲和类型推断
5. 兜底映射为 `string`，输出 `WARN` 日志

---

## 6. 迁移流程

### 6.1 整体流程图

```
┌─────────────┐
│  解析配置    │
└──────┬──────┘
       ▼
┌─────────────┐
│ 连接源数据库 │
└──────┬──────┘
       ▼
┌─────────────┐
│ 获取源库元数据│──→ 库名、表列表、列定义、行数估算、主键/索引识别
└──────┬──────┘
       ▼
┌─────────────┐
│  生成迁移计划 │──→ 类型映射、冲突检测、WARN 汇总、分页策略选择
└──────┬──────┘
       ▼
┌─────────────┐
│ dry-run?    │──Y→ 输出计划并退出
└──────┬──────┘ N
       ▼
┌─────────────┐
│ 初始化 Cornerstone DB │──→ ensureDB()
└──────┬──────┘
       ▼
┌─────────────┐
│ 创建/复用目标 Database │
└──────┬──────┘
       ▼
┌─────────────────────────┐
│  逐表迁移（事务隔离）    │
│  ├── 创建 Table         │
│  ├── 创建 Fields        │
│  └── 批量导入 Records   │──→ 游标分页 + 批量插入 + Checkpoints
└──────┬──────────────────┘
       ▼
┌─────────────┐
│ 数据校验?   │──Y→ 行数对比 → 内容抽样校验 → 生成校验报告
└──────┬──────┘ N
       ▼
┌─────────────┐
│ 输出迁移报告 │
└─────────────┘
```

### 6.2 逐表迁移细节

对于每张待迁移表：

1. **表结构迁移**
   - 在 Cornerstone 创建 Table（使用 `TableService.CreateTable`）
   - 遍历所有列，按映射规则创建 Field（使用 `FieldService.CreateField`）
   - 将源库 `NOT NULL` 映射为 `Field.Required = true`

2. **数据迁移**
   - 查询源表总条数，识别主键/唯一索引列作为游标列
   - **分页策略选择**：
     - 有主键/唯一索引 → **Keyset/Cursor 分页**（推荐）：`WHERE cursor_col > last_val ORDER BY cursor_col LIMIT batch_size`
     - 无主键 → 回退到 OFFSET 分页，输出 WARN
   - 每页读取后转换为 `map[string]interface{}`（字段名 → 值）
   - 使用 `RecordService.BatchCreateRecords` 批量插入
   - 日期/时间类型统一转换为 ISO 8601 / RFC3339 字符串
   - JSON 类型保持原结构
   - **Checkpoint**：每完成 `checkpoint_interval` 条记录，将 `migration_id`、`table`、`cursor_value`、`processed_count` 写入本地状态文件（`~/.cornerstone/migrations/{migration_id}.state.json`）

3. **错误处理与回滚**
   - 单表失败时：
     - `continue_on_error=true`：
       - `rollback_on_failure=table`：删除该表已创建的 Fields 和 Records（软删除），保留 Table 定义或一并删除，输出该表失败报告，继续下一张
       - `rollback_on_failure=none`：保留已写入数据，仅记录错误
     - `continue_on_error=false`：回滚当前表已写入数据，退出程序
   - 单批次插入失败：自动重试 3 次（指数退避），仍失败则按上述策略处理

### 6.3 批量插入与分页优化

#### 分页策略对比

| 策略 | 适用场景 | 性能特征 | 推荐度 |
|------|----------|----------|--------|
| **Keyset/Cursor** | 有主键或唯一索引列 | 恒定时间 O(1)，与偏移量无关 | ⭐⭐⭐ 强烈推荐 |
| **OFFSET** | 无主键表（兜底） | 线性退化 O(N)，高偏移量极慢 | ⭐ 仅兜底 |

**Keyset 分页实现（以自增 ID 为例）：**
```sql
-- 第一批
SELECT * FROM users ORDER BY id ASC LIMIT 500;
-- 后续批次（last_id = 上一批最后一条的 id）
SELECT * FROM users WHERE id > :last_id ORDER BY id ASC LIMIT 500;
```

**UUID 主键适配：**
- 若无单调递增列，使用 `created_at + id` 复合排序确保确定性
- 输出 WARN 提示用户可能存在的重复 `created_at` 场景

#### 批量插入优化

- 使用 `RecordService.BatchCreateRecords` 批量模式
- **Batch Size 建议**：
  - 小记录（<1KB）：200~500 条/批
  - 大记录（>10KB）或含 JSON/文本：50~100 条/批
- **并发控制**：默认 `max_concurrent_tables=1`，避免目标库压力过大；未来可支持多表并发
- **进度输出**：每完成一批打印 `表名: 已迁移 X / Y 条 (速度 Z 条/秒)`

#### 断点续传机制

迁移状态持久化到本地文件：
```json
{
  "migration_id": "mig_20260531_100000",
  "source": "mysql://localhost/source_db",
  "target_db": "source_db",
  "started_at": "2026-05-31T10:00:00Z",
  "tables": {
    "users": {
      "status": "in_progress",
      "cursor_column": "id",
      "cursor_value": 12450,
      "processed_count": 12450,
      "total_estimate": 50000
    }
  }
}
```

使用 `--resume mig_20260531_100000` 时：
1. 读取状态文件
2. 跳过已完成的表
3. 对 `in_progress` 表从 `cursor_value` 继续读取

---

## 7. 源数据库访问层设计

### 7.1 接口抽象

```go
// internal/migration/source/source.go
package source

type Source interface {
    Connect(dsn string) error
    Close() error
    ListDatabases() ([]string, error)
    ListTables(dbName string) ([]string, error)
    GetTableSchema(dbName, tableName string) (*TableSchema, error)
    EstimateRowCount(dbName, tableName string) (int64, error)
    // 分页查询（兼容 OFFSET 和 Cursor 两种模式）
    QueryRows(dbName, tableName string, opts QueryOptions) ([]map[string]interface{}, error)
    // 获取推荐的分页策略
    RecommendPaginationStrategy(dbName, tableName string) PaginationStrategy
}

type PaginationStrategy string

const (
    StrategyCursor PaginationStrategy = "cursor"
    StrategyOffset PaginationStrategy = "offset"
)

type QueryOptions struct {
    Strategy    PaginationStrategy
    CursorColumn string              // cursor 策略时的排序列
    CursorValue  interface{}         // cursor 策略时的起始值（nil 表示从头开始）
    Offset       int64               // offset 策略时的偏移量
    Limit        int64
}

type TableSchema struct {
    Name        string
    Columns     []ColumnSchema
    PrimaryKey  []string            // 主键列名
    UniqueKeys  [][]string          // 唯一索引列组
    RowEstimate int64               // 估算行数
}

type ColumnSchema struct {
    Name         string
    Type         string             // 原始数据库类型字符串
    Nullable     bool
    DefaultValue interface{}
    MaxLength    *int
    Comment      string
    IsPrimaryKey bool
    IsUnique     bool
}
```

### 7.2 实现列表

| 实现 | 文件 | 说明 |
|------|------|------|
| `MySQLSource` | `internal/migration/source/mysql.go` | 基于 `database/sql` + `go-sql-driver/mysql` |
| `PostgresSource` | `internal/migration/source/postgres.go` | 基于 `database/sql` + `lib/pq` 或 `pgx` |
| `SQLiteSource` | `internal/migration/source/sqlite.go` | 基于 `database/sql` + `mattn/go-sqlite3` / `modernc.org/sqlite` |

### 7.3 工厂方法

```go
func NewSource(dbType string) (Source, error)
```

---

## 8. 类型映射器设计

```go
// internal/migration/mapper/mapper.go
package mapper

type TypeMapper interface {
    Map(rawType string) (cornerstoneType string, warning string)
}

func NewTypeMapper(dbType string, overrides map[string]string) TypeMapper
```

实现按数据库类型分文件：
- `mysql_mapper.go`
- `postgres_mapper.go`
- `sqlite_mapper.go`

---

## 9. 迁移报告

执行结束后输出 JSON 格式报告（同时可写日志文件）：

```json
{
  "status": "completed",
  "started_at": "2026-05-31T10:00:00Z",
  "finished_at": "2026-05-31T10:05:30Z",
  "summary": {
    "tables_total": 5,
    "tables_success": 4,
    "tables_failed": 1,
    "records_total": 45230,
    "records_inserted": 45200
  },
  "tables": [
    {
      "source": "users",
      "target": "users",
      "status": "success",
      "fields_created": 8,
      "records_inserted": 12450,
      "duration_ms": 1200
    },
    {
      "source": "orders",
      "target": "orders",
      "status": "failed",
      "error": "字段 'amount' 类型 DECIMAL(38,20) 映射为 number 时精度丢失",
      "records_inserted": 0
    }
  ]
}
```

---

## 10. 错误码与日志

### 10.1 错误码

| 错误码 | 场景 | 处理方式 |
|--------|------|----------|
| `MIG-001` | 源数据库连接失败 | 立即退出，返回非 0 |
| `MIG-002` | 目标 Database 创建失败 | 立即退出 |
| `MIG-003` | 单表结构迁移失败 | 依 `continue_on_error` 决定，支持回滚 |
| `MIG-004` | 单表数据迁移失败 | 依 `continue_on_error` 决定，批次级重试 3 次 |
| `MIG-005` | 类型映射警告 | 输出 WARN，继续执行 |
| `MIG-006` | 不支持的源数据库类型 | 立即退出 |
| `MIG-007` | 断点续传状态文件损坏 | 提示用户使用 `--skip-resume` 重新执行 |
| `MIG-008` | 数据校验失败 | 输出差异报告，不中断迁移流程 |
| `MIG-009` | 无主键表使用 OFFSET 分页 | 输出 WARN，建议用户添加主键或使用小表 |
| `MIG-010` | 配置文件验证失败 | 输出具体字段错误，立即退出 |

### 10.2 日志规范

使用 `pkg/log`（zap）输出结构化日志，关键字段：

```json
{
  "level": "info",
  "msg": "表迁移完成",
  "migration_id": "mig_20260531_100000",
  "table": "users",
  "records": 12450,
  "duration_ms": 1200
}
```

---

## 11. 数据校验策略

迁移完成后（或 `--validate=true` 时），执行三级校验体系：

### 11.1 校验维度

| 维度 | 方法 | 优先级 | 说明 |
|------|------|--------|------|
| **结构校验** | 对比源表和目标表的字段数量、字段名称 | 高 | 快速筛查结构遗漏 |
| **行数校验** | `SELECT COUNT(*)` vs Cornerstone 记录数 | 高 | 快速筛查数据量差异 |
| **内容抽样校验** | 随机抽取 1%~5% 记录对比字段值 | 高 | 精准定位值转换错误 |
| **统计指标校验** | 对比数值列 SUM / AVG、日期范围 | 中 | 快速发现系统性偏差 |

### 11.2 校验流程

```
行数对比（快筛）
    ├── 一致 → 进入抽样校验
    └── 不一致 → 标记差异表，进入抽样定位差异行

抽样校验（精确定位）
    ├── 全部通过 → 校验通过
    └── 存在差异 → 输出差异详情（源值 vs 目标值）
```

### 11.3 校验报告示例

```json
{
  "validation": {
    "status": "passed_with_warnings",
    "tables_checked": 5,
    "tables_passed": 4,
    "tables_failed": 0,
    "tables_warnings": 1,
    "details": [
      {
        "table": "orders",
        "row_count_match": true,
        "sample_checked": 500,
        "sample_mismatch": 0,
        "warnings": ["decimal 精度从 38 位截断到 15 位，3 条记录受影响"]
      }
    ]
  }
}
```

---

## 12. 权限与安全

- 迁移命令需要 `MASTER_TOKEN` 环境变量（与现有 CLI 命令保持一致）
- 源数据库连接信息通过配置文件或命令行传入，**不写入 Cornerstone 数据库**
- 配置文件若包含密码，建议设置文件权限 `0600`，CLI 启动时检查并警告
- 源数据库使用只读连接（不执行任何写操作）
- 迁移状态文件（含 cursor 位点）存储在用户本地目录 `~/.cornerstone/migrations/`，**不包含敏感数据**（仅含表名、记录数、cursor 值）

---

## 13. 测试策略

| 测试类型 | 范围 | 说明 |
|----------|------|------|
| 单元测试 | `internal/migration/source/*` | 使用 `sqlmock` 或 `dockertest` 模拟源库，验证游标分页正确性 |
| 单元测试 | `internal/migration/mapper/*` | 类型映射覆盖所有组合，包含边界类型和未识别类型兜底 |
| 单元测试 | `internal/migration/runner.go` | 模拟断点续传场景：中途终止 → 恢复 → 验证数据完整性 |
| 集成测试 | `internal/migration/` | 启动 SQLite/MySQL 容器 → 迁移 → 断言 Cornerstone 数据 + 校验报告 |
| 性能测试 | `internal/migration/` | 10万+ 记录大表迁移，验证 cursor 分页性能优于 offset |
| E2E 测试 | CLI 层 | 调用 `go run cmd/main.go migration run` 验证输出报告格式 |

---

## 14. 文件变更清单

| 文件/目录 | 操作 | 说明 |
|-----------|------|------|
| `internal/cli/migration.go` | 新建 | `migration` 主命令及子命令注册 |
| `internal/migration/config.go` | 新建 | 配置文件解析与验证 |
| `internal/migration/source/source.go` | 新建 | Source 接口定义 |
| `internal/migration/source/mysql.go` | 新建 | MySQL 源实现 |
| `internal/migration/source/postgres.go` | 新建 | PostgreSQL 源实现 |
| `internal/migration/source/sqlite.go` | 新建 | SQLite 源实现 |
| `internal/migration/mapper/mapper.go` | 新建 | TypeMapper 接口 |
| `internal/migration/mapper/mysql_mapper.go` | 新建 | MySQL 类型映射 |
| `internal/migration/mapper/postgres_mapper.go` | 新建 | PostgreSQL 类型映射 |
| `internal/migration/mapper/sqlite_mapper.go` | 新建 | SQLite 类型映射 |
| `internal/migration/runner.go` | 新建 | 迁移协调器（计划生成 + 执行调度） |
| `internal/migration/reporter.go` | 新建 | 报告生成与输出 |
| `pkg/migration/template.yaml` | 新建 | 默认配置文件模板 |
| `internal/cli/db.go` | 修改 | 提取 `ensureDB` 等公共函数到 `internal/cli/common.go`（若尚未提取） |
| `go.mod` | 修改 | 按需添加 `go-sql-driver/mysql`、`lib/pq` 等驱动 |
| `docs/migration-spec.md` | 新建 | 本文档 |
| `~/.cornerstone/migrations/` | 新建（运行时） | 迁移状态文件存储目录 |

---

## 15. 后续扩展（V2 规划）

- [ ] **多源数据库支持**：SQL Server、Oracle、MongoDB（文档 → JSON Field）
- [ ] **索引/约束映射**：将唯一约束映射为 Field 的 `Validation` 正则或唯一性校验
- [ ] **外键关系映射**：识别外键并映射为 Cornerstone 的关联字段或引用字段
- [ ] **增量迁移（CDC 模式）**：
  - 基于源库 binlog/MySQL WAL 的实时捕获
  - 或基于 `updated_at` + `id` 的轮询增量模式
  - 架构参考：Debezium + Kafka 的简化版
- [ ] **数据转换钩子**：允许用户注册 Go 插件或配置 Lua/JS 脚本进行字段级转换
- [ ] **迁移任务持久化到数据库**：
  - 新增 `migrations` 表记录任务元数据
  - Web UI 展示迁移进度、历史记录、差异报告
  - 支持分布式环境下的断点续传
- [ ] **多表并发迁移**：`max_concurrent_tables > 1` 时的并发控制与死锁避免
- [ ] **Schema 版本对比**：类似 Directus 的 Schema diff，支持环境间结构同步
- [ ] **媒体文件迁移**：BLOB / BYTEA 字段自动上传为 Cornerstone File 并绑定到记录
