[English](Query.md) | [中文](Query.zh.md)

# Query DSL

Describe queries via JSON without writing SQL by hand. Supports filtering, sorting, aggregation, and JOIN.

---

## Endpoints

### Unified Query

```bash
# POST
curl -X POST http://localhost:8080/api/v1/query \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", ...}'

# GET (query params encoded)
curl "http://localhost:8080/api/v1/query?q=%7B%22from%22%3A%22records%22%7D" \
  -H "Authorization: Bearer cs_your_token"
```

### Simplified Query

```bash
curl "http://localhost:8080/api/v1/query/simple?table=records&filter=%7B%7D&sort=-created_at&page=1&size=20" \
  -H "Authorization: Bearer cs_your_token"
```

### Batch Query

```bash
curl -X POST http://localhost:8080/api/v1/query/batch \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"queries": {"q1": {"from": "records", ...}, "q2": {"from": "tables", ...}}}'
```

### Query Explain

```bash
curl -X POST http://localhost:8080/api/v1/query/explain \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"from": "records", ...}'
```

### Accessible Table List

```bash
curl http://localhost:8080/api/v1/query/tables \
  -H "Authorization: Bearer cs_your_token"
```

---

## Query Syntax

### Basic Query

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

### Simplified Syntax

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

### JOIN Query

```json
{
  "from": "records",
  "select": ["records.id", "records.data"],
  "join": [
    {
      "type": "left",
      "table": "tables",
      "as": "t",
      "on": {"left": "records.table_id", "op": "eq", "right": "t.id"},
      "select": ["t.name", "t.description"]
    }
  ],
  "where": {
    "and": [
      {"field": "table_id", "value": "tbl_xxx"},
      {"field": "t.name", "op": "like", "value": "user"}
    ]
  }
}
```

> **Note on JOIN select fields**: When using JOIN, always use **qualified field names** (`table_alias.field_name` or `table_name.field_name`) in `select` to avoid `ambiguous column name` errors. For example, use `records.id` instead of `id`, and `t.name` instead of `name`.

### Aggregate Query

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

## Operators

| Operator | Description | Example |
|----------|-------------|---------|
| eq | Equal to | `{"field": "status", "op": "eq", "value": "paid"}` |
| ne | Not equal to | `{"field": "status", "op": "ne", "value": "deleted"}` |
| gt | Greater than | `{"field": "total", "op": "gt", "value": 100}` |
| gte | Greater than or equal to | `{"field": "total", "op": "gte", "value": 100}` |
| lt | Less than | `{"field": "total", "op": "lt", "value": 500}` |
| lte | Less than or equal to | `{"field": "total", "op": "lte", "value": 500}` |
| like | Fuzzy search | `{"field": "name", "op": "like", "value": "zhang"}` |
| in | IN query | `{"field": "status", "op": "in", "value": ["paid", "shipped"]}` |
| between | Range query | `{"field": "created_at", "op": "between", "value": ["2024-01-01", "2024-12-31"]}` |
| is_null | Null check | `{"field": "deleted_at", "op": "is_null", "value": true}` |

---

## Query Limits

| Limit | Default | Description |
|-------|---------|-------------|
| MaxJoins | 3 | Maximum number of JOIN tables |
| MaxPageSize | 1000 | Maximum page size |
| MaxDepth | 5 | Maximum nesting depth for nested queries |
| MaxRows | 10000 | Maximum number of returned rows |
| MaxFields | 100 | Maximum number of query fields |

---

## Advanced Features

### HAVING Clause

Filter after aggregation. Syntax is identical to `where`:

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

### Aggregate Functions

| Function | Description |
|----------|-------------|
| `count` | Count |
| `count_distinct` | Distinct count |
| `sum` | Sum |
| `avg` | Average |
| `min` | Minimum |
| `max` | Maximum |
| `stddev` | Standard deviation |
| `stddev_pop` | Population standard deviation |
| `stddev_samp` | Sample standard deviation |
| `variance` | Variance |
| `var_pop` | Population variance |
| `var_samp` | Sample variance |

### JOIN Types

Four JOIN types are supported:

| Type | Description |
|------|-------------|
| `left` | LEFT JOIN |
| `right` | RIGHT JOIN |
| `inner` | INNER JOIN |
| `outer` | FULL OUTER JOIN |

### NOT Condition Negation

Any condition can be negated by adding `"not": true`:

```json
{"field": "status", "op": "eq", "value": "deleted", "not": true}
```

### Nested AND/OR Conditions

Conditions can be nested and grouped arbitrarily:

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

### UNION / INTERSECT Set Operations

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

`intersect` works the same way; just replace the `union` field.

### JSON Path Field Syntax

Access values inside JSONB fields. PostgreSQL automatically uses `->>` / `->` syntax, while SQLite automatically converts to `JSON_EXTRACT`:

```json
{"field": "data->>status", "op": "eq", "value": "paid"}
```

---

## Query Validation & Schema Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/query/validate` | Validate query DSL + permissions (does not execute) |
| GET | `/api/v1/query/schema/:table` | Get queryable field schema for a table |
