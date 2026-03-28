# Query DSL - 前端查询 DSL

前端通过 JSON 字符串描述查询需求，后端安全解析并执行。无需手写 SQL，同时保证安全性。

支持 **PostgreSQL** 和 **SQLite** 两种数据库。

## 数据库配置

### 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `DB_TYPE` | 数据库类型 | `postgres` 或 `sqlite` |
| `DATABASE_URL` | 连接字符串 | PostgreSQL: `postgres://...` / SQLite: `文件路径` |
| `DB_MAX_OPEN` | 最大连接数 | `10` |
| `DB_MAX_IDLE` | 最大空闲连接 | `5` |
| `DB_MAX_LIFETIME` | 连接最大生命周期(秒) | `3600` |

### PostgreSQL 配置示例

```bash
DB_TYPE=postgres
DATABASE_URL=postgres://postgres:postgres@localhost:5432/cornerstone?sslmode=disable
```

### SQLite 配置示例

```bash
DB_TYPE=sqlite
DATABASE_URL=./cornerstone.db
# 或使用内存数据库
DATABASE_URL=:memory:
```

## 快速开始

```go
import "github.com/jiangfire/cornerstone/backend/pkg/query"

// 创建执行器
executor := query.NewExecutor(db)

// 执行查询
result, err := executor.Execute(ctx, req, userID)
```

## 查询语法

### 基础查询
```json
{
  "from": "records",
  "select": ["id", "data", "created_at"],
  "where": {
    "and": [
      {"field": "table_id", "op": "eq", "value": "tbl_xxx"},
      {"field": "data.status", "op": "in", "value": ["paid", "shipped"]}
    ]
  },
  "orderBy": [{"field": "created_at", "dir": "desc"}],
  "page": 1,
  "size": 20
}
```

### 简化语法
```json
{
  "table": "records",
  "filter": {
    "table_id": "tbl_xxx",
    "status": {"in": ["paid", "shipped"]},
    "created_at": {"gt": "2024-01-01"}
  },
  "sort": "-created_at",
  "page": 1,
  "size": 20
}
```

### JOIN 查询
```json
{
  "from": "records",
  "select": ["id", "data"],
  "join": [
    {
      "type": "left",
      "table": "users",
      "as": "u",
      "on": "records.created_by = u.id",
      "select": ["u.username", "u.email"]
    }
  ],
  "where": {
    "and": [
      {"field": "table_id", "value": "tbl_xxx"},
      {"field": "u.username", "op": "like", "value": "zhang"}
    ]
  }
}
```

### 聚合查询
```json
{
  "from": "records",
  "select": ["data.status"],
  "groupBy": ["data.status"],
  "aggregate": [
    {"func": "count", "field": "*", "as": "total"},
    {"func": "sum", "field": "data.amount", "as": "total_amount"}
  ],
  "where": {
    "table_id": {"op": "eq", "value": "tbl_xxx"}
  }
}
```

## 操作符

| 操作符 | 说明 | 示例 |
|--------|------|------|
| eq | 等于 | `{"field": "status", "op": "eq", "value": "paid"}` |
| ne | 不等于 | `{"field": "status", "op": "ne", "value": "deleted"}` |
| gt | 大于 | `{"field": "total", "op": "gt", "value": 100}` |
| gte | 大于等于 | `{"field": "total", "op": "gte", "value": 100}` |
| lt | 小于 | `{"field": "total", "op": "lt", "value": 500}` |
| lte | 小于等于 | `{"field": "total", "op": "lte", "value": 500}` |
| like | 模糊查询 | `{"field": "name", "op": "like", "value": "zhang"}` |
| in | IN 查询 | `{"field": "status", "op": "in", "value": ["paid", "shipped"]}` |
| between | 范围查询 | `{"field": "created_at", "op": "between", "value": ["2024-01-01", "2024-12-31"]}` |
| is_null | 为空判断 | `{"field": "deleted_at", "op": "is_null", "value": true}` |

## API 接口

### 统一查询
```
POST /api/query
GET  /api/query?q={json_string}
```

### 简化查询
```
GET /api/query/simple?table=records&filter={...}&sort=-created_at&page=1&size=20
```

### 批量查询
```
POST /api/query/batch
```

### 查询解释（调试）
```
POST /api/query/explain
```

### 获取可访问表列表
```
GET /api/query/tables
```

## 安全防护

1. **表/字段白名单** - 只允许访问预定义的表和字段
2. **SQL 注入防护** - 所有参数使用占位符
3. **查询复杂度限制** - 限制 JOIN 数量、分页大小、嵌套深度
4. **用户权限自动限制** - 自动添加用户可访问的数据过滤

## 数据库适配

查询 DSL 自动检测数据库类型并生成对应的 SQL：

| 功能 | PostgreSQL | SQLite |
|------|------------|--------|
| JSON 字段查询 | `data->>'field'` | `JSON_EXTRACT(data, '$.field')` |
| 数组类型 | 原生支持 | JSON 模拟 |
| 全文搜索 | GIN 索引 | LIKE 查询 |
| 物化视图 | 支持 | 不支持（自动跳过） |

## 查询限制

| 限制项 | 默认值 | 说明 |
|--------|--------|------|
| MaxJoins | 3 | 最多 JOIN 表数 |
| MaxPageSize | 1000 | 最大分页大小 |
| MaxDepth | 5 | 嵌套查询最大深度 |
| MaxRows | 10000 | 最大返回行数 |
| MaxFields | 100 | 最大查询字段数 |
