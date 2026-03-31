# Query DSL 使用指南

## 概述

Cornerstone Query DSL 是一个强大且安全的查询语言，允许前端通过 JSON 描述查询需求，后端安全解析并执行。无需手写 SQL，同时保证安全性。

### 核心特性

- ✅ **安全优先**：表/字段白名单、SQL 注入防护、权限自动过滤
- ✅ **数据库兼容**：支持 PostgreSQL 和 SQLite
- ✅ **JSON 字段支持**：原生支持 JSON/JSONB 字段查询
- ✅ **灵活查询**：支持 JOIN、聚合、分组、排序
- ✅ **权限控制**：自动根据用户权限过滤数据
- ✅ **简化语法**：提供简化查询语法，适合常见场景

---

## 快速开始

### 1. 基础查询

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

### 2. 简化语法（推荐用于简单查询）

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

---

## API 接口

### 统一查询接口

#### POST /api/query
```bash
curl -X POST http://localhost:8080/api/query \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "records",
    "select": ["id", "data"],
    "page": 1,
    "size": 20
  }'
```

#### GET /api/query?q={json_string}
```bash
curl -X GET "http://localhost:8080/api/query?q=%7B%22from%22%3A%22records%22%2C%22select%22%3A%5B%22id%22%2C%22data%22%5D%2C%22page%22%3A1%2C%22size%22%3A20%7D" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 简化查询接口

#### GET /api/query/simple
```bash
curl -X GET "http://localhost:8080/api/query/simple?table=records&filter=%7B%22status%22%3A%22paid%22%7D&sort=-created_at&page=1&size=20" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

参数说明：
- `table` - 表名（必需）
- `filter` - 过滤条件（JSON 字符串，可选）
- `sort` - 排序字段（支持 `-` 前缀降序，可选）
- `page` - 页码（默认 1）
- `size` - 每页大小（默认 20，最大 1000）

### 其他接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/query/batch` | POST | 批量查询 |
| `/api/query/explain` | POST | 查询解释（返回 SQL，用于调试） |
| `/api/query/validate` | POST | 验证查询权限 |
| `/api/query/tables` | GET | 获取可访问的表列表 |
| `/api/query/schema/:table` | GET | 获取表结构信息 |

---

## 查询语法详解

### 1. 基础查询结构

```json
{
  "from": "records",           // 表名（必需）
  "select": ["id", "data"],    // 要查询的字段（可选，默认 *）
  "where": {                   // 过滤条件（可选）
    "field": "table_id",
    "op": "eq",
    "value": "tbl_xxx"
  },
  "orderBy": [                 // 排序（可选）
    {"field": "created_at", "dir": "desc"}
  ],
  "page": 1,                   // 页码（默认 1）
  "size": 20                   // 每页大小（默认 20）
}
```

### 2. WHERE 条件

#### 简单条件
```json
{
  "where": {
    "field": "status",
    "op": "eq",
    "value": "paid"
  }
}
```

#### AND 条件
```json
{
  "where": {
    "and": [
      {"field": "status", "op": "eq", "value": "paid"},
      {"field": "amount", "op": "gte", "value": 100}
    ]
  }
}
```

#### OR 条件
```json
{
  "where": {
    "or": [
      {"field": "status", "op": "eq", "value": "paid"},
      {"field": "status", "op": "eq", "value": "shipped"}
    ]
  }
}
```

#### 嵌套条件
```json
{
  "where": {
    "and": [
      {"field": "table_id", "op": "eq", "value": "tbl_xxx"},
      {
        "or": [
          {"field": "status", "op": "eq", "value": "paid"},
          {"field": "status", "op": "eq", "value": "shipped"}
        ]
      }
    ]
  }
}
```

### 3. JSON 字段查询

#### 查询 JSON 字段
```json
{
  "where": {
    "field": "data.status",
    "op": "eq",
    "value": "paid"
  }
}
```

#### JSON 字段范围查询
```json
{
  "where": {
    "field": "data.amount",
    "op": "gte",
    "value": 100
  }
}
```

#### JSON 字段排序
```json
{
  "orderBy": [
    {"field": "data.amount", "dir": "desc"}
  ]
}
```

### 4. JOIN 查询

```json
{
  "from": "records",
  "select": ["records.id", "data", "u.username"],
  "join": [
    {
      "type": "left",                    // left, right, inner
      "table": "users",
      "as": "u",                          // 别名
      "on": "records.created_by = u.id",
      "select": ["u.username", "u.email"] // 要选择的 JOIN 表字段
    }
  ],
  "where": {
    "and": [
      {"field": "table_id", "op": "eq", "value": "tbl_xxx"},
      {"field": "u.username", "op": "like", "value": "zhang"}
    ]
  }
}
```

### 5. 聚合查询

```json
{
  "from": "records",
  "select": ["data.status"],
  "groupBy": ["data.status"],
  "aggregate": [
    {"func": "count", "field": "*", "as": "total"},
    {"func": "sum", "field": "data.amount", "as": "total_amount"},
    {"func": "avg", "field": "data.amount", "as": "avg_amount"},
    {"func": "min", "field": "data.amount", "as": "min_amount"},
    {"func": "max", "field": "data.amount", "as": "max_amount"}
  ],
  "where": {
    "field": "table_id",
    "op": "eq",
    "value": "tbl_xxx"
  }
}
```

### 6. 批量查询

```json
{
  "queries": {
    "total_records": {
      "from": "records",
      "select": ["id"],
      "where": {"field": "table_id", "op": "eq", "value": "tbl_xxx"}
    },
    "paid_records": {
      "from": "records",
      "select": ["id"],
      "where": {
        "and": [
          {"field": "table_id", "op": "eq", "value": "tbl_xxx"},
          {"field": "data.status", "op": "eq", "value": "paid"}
        ]
      }
    }
  }
}
```

---

## 操作符列表

| 操作符 | 说明 | 示例 | 支持类型 |
|--------|------|------|----------|
| `eq` | 等于 | `{"field": "status", "op": "eq", "value": "paid"}` | 所有类型 |
| `ne` | 不等于 | `{"field": "status", "op": "ne", "value": "deleted"}` | 所有类型 |
| `gt` | 大于 | `{"field": "amount", "op": "gt", "value": 100}` | 数字、日期 |
| `gte` | 大于等于 | `{"field": "amount", "op": "gte", "value": 100}` | 数字、日期 |
| `lt` | 小于 | `{"field": "amount", "op": "lt", "value": 500}` | 数字、日期 |
| `lte` | 小于等于 | `{"field": "amount", "op": "lte", "value": 500}` | 数字、日期 |
| `like` | 模糊查询 | `{"field": "name", "op": "like", "value": "zhang"}` | 字符串 |
| `in` | IN 查询 | `{"field": "status", "op": "in", "value": ["paid", "shipped"]}` | 所有类型 |
| `between` | 范围查询 | `{"field": "created_at", "op": "between", "value": ["2024-01-01", "2024-12-31"]}` | 数字、日期 |
| `is_null` | 为空判断 | `{"field": "deleted_at", "op": "is_null", "value": true}` | 所有类型 |

---

## 简化语法详解

简化语法适合简单查询，更简洁直观：

### 基础过滤
```json
{
  "table": "records",
  "filter": {
    "table_id": "tbl_xxx",
    "status": "paid"
  }
}
```

### 操作符过滤
```json
{
  "table": "records",
  "filter": {
    "status": {"in": ["paid", "shipped"]},
    "amount": {"gte": 100, "lt": 500},
    "created_at": {"gt": "2024-01-01"}
  }
}
```

### 排序
```json
{
  "table": "records",
  "filter": {"status": "paid"},
  "sort": "-created_at"  // - 表示降序
}
```

多字段排序：
```json
{
  "sort": "-created_at,id"  // 先按 created_at 降序，再按 id 升序
}
```

---

## 实际应用示例

### 1. 获取订单列表
```json
{
  "from": "records",
  "select": ["id", "data", "created_at"],
  "where": {
    "and": [
      {"field": "table_id", "op": "eq", "value": "tbl_orders"},
      {"field": "data.status", "op": "in", "value": ["paid", "shipped"]}
    ]
  },
  "orderBy": [{"field": "created_at", "dir": "desc"}],
  "page": 1,
  "size": 20
}
```

### 2. 统计各状态订单数量
```json
{
  "from": "records",
  "select": ["data.status"],
  "groupBy": ["data.status"],
  "aggregate": [
    {"func": "count", "field": "*", "as": "count"}
  ],
  "where": {
    "field": "table_id",
    "op": "eq",
    "value": "tbl_orders"
  }
}
```

### 3. 查询用户及其订单
```json
{
  "from": "users",
  "select": ["id", "username", "email"],
  "join": [
    {
      "type": "left",
      "table": "records",
      "as": "r",
      "on": "users.id = r.created_by",
      "select": ["r.id", "r.data"]
    }
  ],
  "where": {
    "field": "username",
    "op": "like",
    "value": "zhang"
  }
}
```

### 4. 时间范围查询
```json
{
  "from": "records",
  "select": ["id", "data", "created_at"],
  "where": {
    "and": [
      {"field": "created_at", "op": "gte", "value": "2024-01-01"},
      {"field": "created_at", "op": "lt", "value": "2024-02-01"}
    ]
  },
  "orderBy": [{"field": "created_at", "dir": "desc"}]
}
```

### 5. 简化语法示例
```json
{
  "table": "records",
  "filter": {
    "table_id": "tbl_orders",
    "data.status": {"in": ["paid", "shipped"]},
    "data.amount": {"gte": 100}
  },
  "sort": "-created_at",
  "page": 1,
  "size": 20
}
```

---

## 安全特性

### 1. 表/字段白名单
只能访问预定义的表和字段：
- 自动过滤用户无权访问的表
- 管理员表（users, database_access 等）仅管理员可访问
- 敏感字段（如 password）自动排除

### 2. SQL 注入防护
- 所有参数使用占位符
- 不支持原始 SQL 片段
- 严格的类型检查

### 3. 查询复杂度限制
| 限制项 | 默认值 | 说明 |
|--------|--------|------|
| MaxJoins | 3 | 最多 JOIN 表数 |
| MaxPageSize | 1000 | 最大分页大小 |
| MaxDepth | 5 | 嵌套查询最大深度 |
| MaxRows | 10000 | 最大返回行数 |
| MaxFields | 100 | 最大查询字段数 |

### 4. 权限自动过滤
根据用户权限自动添加过滤条件：
- 表级别：只返回用户可访问的表
- 行级别：自动过滤用户无权访问的数据
- 字段级别：自动隐藏敏感字段

---

## 数据库适配

Query DSL 自动检测数据库类型并生成对应的 SQL：

| 功能 | PostgreSQL | SQLite |
|------|------------|--------|
| JSON 字段查询 | `data->>'field'` | `JSON_EXTRACT(data, '$.field')` |
| 数组类型 | 原生支持 | JSON 模拟 |
| 全文搜索 | GIN 索引 | LIKE 查询 |
| 物化视图 | 支持 | 不支持（自动跳过） |

### JSON 字段示例

假设 records 表有 data 字段（JSON 类型）：
```json
{
  "id": "rec_1",
  "data": {
    "status": "paid",
    "amount": 100,
    "customer": {
      "name": "张三",
      "email": "zhang@example.com"
    }
  }
}
```

查询示例：
```json
{
  "where": {
    "and": [
      {"field": "data.status", "op": "eq", "value": "paid"},
      {"field": "data.amount", "op": "gte", "value": 100},
      {"field": "data.customer.name", "op": "like", "value": "张"}
    ]
  }
}
```

---

## 响应格式

### 成功响应
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "data": [
      {"id": "rec_1", "data": "{...}", "created_at": "2024-01-01"},
      {"id": "rec_2", "data": "{...}", "created_at": "2024-01-02"}
    ],
    "total": 100,
    "page": 1,
    "size": 20,
    "has_more": true
  }
}
```

### 错误响应
```json
{
  "code": 400,
  "message": "查询格式错误: 必须指定表名",
  "data": null
}
```

### 权限错误
```json
{
  "code": 403,
  "message": "您没有访问表 'users' 的权限",
  "data": null
}
```

---

## 最佳实践

### 1. 使用简化语法
对于简单查询，优先使用简化语法：
```json
// 推荐
{
  "table": "records",
  "filter": {"status": "paid"},
  "sort": "-created_at"
}

// 不推荐（过于复杂）
{
  "from": "records",
  "where": {"field": "status", "op": "eq", "value": "paid"},
  "orderBy": [{"field": "created_at", "dir": "desc"}]
}
```

### 2. 合理使用分页
```json
{
  "table": "records",
  "page": 1,
  "size": 20  // 建议 20-100 之间
}
```

### 3. 避免过深嵌套
```json
// 推荐
{
  "where": {
    "and": [
      {"field": "status", "op": "eq", "value": "paid"},
      {"field": "amount", "op": "gte", "value": 100}
    ]
  }
}

// 避免（过于复杂）
{
  "where": {
    "and": [
      {
        "or": [
          {
            "and": [
              {"field": "status", "op": "eq", "value": "paid"},
              {"field": "amount", "op": "gte", "value": 100}
            ]
          }
        ]
      }
    ]
  }
}
```

### 4. 使用查询解释调试
```bash
curl -X POST http://localhost:8080/api/query/explain \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "records",
    "where": {"field": "status", "op": "eq", "value": "paid"}
  }'
```

响应：
```json
{
  "code": 200,
  "data": {
    "sql": "SELECT id, data, created_at FROM records WHERE status = ? AND table_id IN (?)",
    "params": ["paid", "tbl_xxx"]
  }
}
```

### 5. 利用批量查询
减少网络请求：
```json
{
  "queries": {
    "summary": {
      "from": "records",
      "select": ["data.status"],
      "aggregate": [{"func": "count", "field": "*", "as": "count"}]
    },
    "details": {
      "from": "records",
      "select": ["id", "data"],
      "page": 1,
      "size": 10
    }
  }
}
```

---

## 常见问题

### Q1: 如何查询 JSON 字段？
```json
{
  "where": {
    "field": "data.status",
    "op": "eq",
    "value": "paid"
  }
}
```

### Q2: 如何实现分页？
```json
{
  "page": 1,
  "size": 20
}
```

响应中的 `has_more` 字段表示是否有更多数据。

### Q3: 如何排序？
```json
{
  "orderBy": [
    {"field": "created_at", "dir": "desc"}
  ]
}
```

简化语法：
```json
{
  "sort": "-created_at"
}
```

### Q4: 如何处理权限问题？
确保用户有访问表的权限：
1. 检查用户是否被授予数据库访问权限
2. 检查角色权限（viewer, editor, admin, owner）
3. 使用 `/api/query/tables` 查看可访问的表列表

### Q5: 如何调试查询？
使用 explain 接口：
```bash
POST /api/query/explain
```

这会返回生成的 SQL 和参数，便于调试。

### Q6: 支持哪些聚合函数？
支持：`count`, `sum`, `avg`, `min`, `max`

### Q7: 如何查询嵌套 JSON 字段？
```json
{
  "where": {
    "field": "data.customer.name",
    "op": "like",
    "value": "张"
  }
}
```

### Q8: 限制最大返回行数？
通过 `size` 参数控制：
```json
{
  "size": 100
}
```

最大值由 `MaxPageSize` 配置决定（默认 1000）。

---

## 前端集成示例

### React + Axios

```typescript
import axios from 'axios';

const api = axios.create({
  baseURL: 'http://localhost:8080/api',
  headers: {
    'Authorization': `Bearer ${getToken()}`
  }
});

// 基础查询
export async function queryRecords(page = 1, size = 20) {
  const response = await api.post('/query', {
    from: 'records',
    select: ['id', 'data', 'created_at'],
    page,
    size
  });
  return response.data.data;
}

// 简化查询
export async function simpleQuery(filter: any, sort = '-created_at') {
  const response = await api.get('/query/simple', {
    params: {
      table: 'records',
      filter: JSON.stringify(filter),
      sort
    }
  });
  return response.data.data;
}

// 使用示例
const records = await queryRecords(1, 20);
const paidRecords = await simpleQuery({
  'data.status': 'paid'
});
```

### Vue + Fetch

```javascript
// 基础查询
async function queryRecords(page = 1, size = 20) {
  const response = await fetch('/api/query', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${getToken()}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      from: 'records',
      select: ['id', 'data', 'created_at'],
      page,
      size
    })
  });
  return response.json();
}

// 简化查询
async function simpleQuery(filter, sort = '-created_at') {
  const params = new URLSearchParams({
    table: 'records',
    filter: JSON.stringify(filter),
    sort
  });
  const response = await fetch(`/api/query/simple?${params}`, {
    headers: {
      'Authorization': `Bearer ${getToken()}`
    }
  });
  return response.json();
}
```

---

## 性能优化建议

1. **合理使用索引**：确保常用查询字段有索引
2. **限制返回字段**：只查询需要的字段
3. **使用分页**：避免一次性查询大量数据
4. **避免过深嵌套**：保持 WHERE 条件简洁
5. **利用聚合**：使用聚合函数减少数据传输
6. **缓存结果**：对不常变化的数据进行缓存

---

## 更多资源

- [API 文档](./API.md)
- [权限系统说明](./PERMISSION-SYSTEM.md)
- [数据库配置](./DATABASE-SETUP.md)
- [MCP 集成](./MCP-INTEGRATION.md)

如有问题，请提交 Issue 或联系技术支持。
