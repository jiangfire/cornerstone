[English](Query.md) | [中文](Query.zh.md)

# Query DSL

通过 JSON 描述查询需求，无需手写 SQL。支持过滤、排序、聚合、JOIN。

---

## 接口

### 统一查询

```bash
# POST
curl -X POST http://localhost:8080/api/v1/query \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", ...}'

# GET（查询参数编码）
curl "http://localhost:8080/api/v1/query?q=%7B%22from%22%3A%22records%22%7D" \
  -H "Authorization: Bearer cs_your_token"
```

### 简化查询

```bash
curl "http://localhost:8080/api/v1/query/simple?table=records&filter=%7B%7D&sort=-created_at&page=1&size=20" \
  -H "Authorization: Bearer cs_your_token"
```

### 批量查询

```bash
curl -X POST http://localhost:8080/api/v1/query/batch \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"queries": {"q1": {"from": "records", ...}, "q2": {"from": "tables", ...}}}'
```

### 查询解释

```bash
curl -X POST http://localhost:8080/api/v1/query/explain \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", ...}'
```

### 可访问表列表

```bash
curl http://localhost:8080/api/v1/query/tables \
  -H "Authorization: Bearer cs_your_token"
```

---

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
      "on": {"left": "records.created_by", "op": "eq", "right": "u.id"},
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
    "and": [
      {"field": "table_id", "op": "eq", "value": "tbl_xxx"}
    ]
  }
}
```

---

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

---

## 查询限制

| 限制项 | 默认值 | 说明 |
|--------|--------|------|
| MaxJoins | 3 | 最多 JOIN 表数 |
| MaxPageSize | 1000 | 最大分页大小 |
| MaxDepth | 5 | 嵌套查询最大深度 |
| MaxRows | 10000 | 最大返回行数 |
| MaxFields | 100 | 最大查询字段数 |

---

## 高级功能

### HAVING 子句

聚合后过滤，语法与 `where` 一致：

```json
{
  "from": "records",
  "select": ["data.status"],
  "groupBy": ["data.status"],
  "aggregate": [
    {"func": "count", "field": "*", "as": "total"}
  ],
  "having": {
    "and": [
      {"field": "total", "op": "gt", "value": 10}
    ]
  }
}
```

### 聚合函数

| 函数 | 说明 |
|------|------|
| `count` | 计数 |
| `count_distinct` | 去重计数 |
| `sum` | 求和 |
| `avg` | 平均值 |
| `min` | 最小值 |
| `max` | 最大值 |
| `stddev` | 标准差 |
| `stddev_pop` | 总体标准差 |
| `stddev_samp` | 样本标准差 |
| `variance` | 方差 |
| `var_pop` | 总体方差 |
| `var_samp` | 样本方差 |

### JOIN 类型

支持四种 JOIN 类型：

| 类型 | 说明 |
|------|------|
| `left` | LEFT JOIN |
| `right` | RIGHT JOIN |
| `inner` | INNER JOIN |
| `outer` | FULL OUTER JOIN |

### NOT 条件否定

任意条件可添加 `"not": true` 取反：

```json
{"field": "status", "op": "eq", "value": "deleted", "not": true}
```

### 嵌套 AND/OR 条件

条件可任意嵌套分组：

```json
{
  "and": [
    {"field": "table_id", "op": "eq", "value": "tbl_xxx"},
    {
      "or": [
        {"field": "data.status", "op": "eq", "value": "paid"},
        {"field": "data.status", "op": "eq", "value": "shipped"}
      ]
    }
  ]
}
```

### UNION / INTERSECT 集合查询

```json
{
  "from": "active_records",
  "select": ["id", "name"],
  "union": [
    {
      "from": "archived_records",
      "select": ["id", "name"]
    }
  ]
}
```

`intersect` 用法相同，替换 `union` 字段即可。

### JSON 路径字段语法

访问 JSONB 字段内部值，PostgreSQL 自动使用 `->>` `/`->` 语法，SQLite 自动转为 `JSON_EXTRACT`：

```json
{"field": "data->>status", "op": "eq", "value": "paid"}
```

---

## 查询校验与 Schema 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/query/validate` | 校验查询 DSL + 权限（不执行） |
| GET | `/api/v1/query/schema/:table` | 获取可查询表的字段 Schema |
