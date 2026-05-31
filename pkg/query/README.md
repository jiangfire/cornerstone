# Query DSL

通过 JSON 描述查询需求，无需手写 SQL。支持过滤、排序、聚合、JOIN。

---

## 接口

### 统一查询

```bash
# POST
curl -X POST http://localhost:8080/api/query \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", ...}'

# GET（查询参数编码）
curl "http://localhost:8080/api/query?q=%7B%22from%22%3A%22records%22%7D" \
  -H "Authorization: Bearer cs_your_token"
```

### 简化查询

```bash
curl "http://localhost:8080/api/query/simple?table=records&filter=%7B%7D&sort=-created_at&page=1&size=20" \
  -H "Authorization: Bearer cs_your_token"
```

### 批量查询

```bash
curl -X POST http://localhost:8080/api/query/batch \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"queries": {"q1": {"from": "records", ...}, "q2": {"from": "tables", ...}}}'
```

### 查询解释

```bash
curl -X POST http://localhost:8080/api/query/explain \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", ...}'
```

### 可访问表列表

```bash
curl http://localhost:8080/api/query/tables \
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
