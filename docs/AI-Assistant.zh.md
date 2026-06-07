[English](AI-Assistant.md) | [中文](AI-Assistant.zh.md)

# AI 助手

> 使用自然语言与 Cornerstone 数据进行交互。

---

## 启用 AI 助手

在 `.env` 中配置 LLM：

```bash
LLM_API_KEY=sk-your-api-key
LLM_MODEL=gpt-4o
# LLM_BASE_URL=https://api.openai.com/v1  # 可选：自定义 API 端点
```

重启服务器后，AI 助手即生效。

---

## 聊天 API

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "List all databases",
    "context": {}
  }'
```

响应：

```json
{
  "type": "result",
  "reply": "Here are your databases:\n- db_abc123: Project DB\n- db_def456: Test DB",
  "context": {}
}
```

### 参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `message` | string | ✅ | 用户消息 |
| `context` | object | ❌ | 可选上下文；传递额外信息以引导 AI |

---

## AI 工具列表

AI 助手内部调用以下工具与数据交互。权限与普通令牌相同（受作用域限制）：

| 工具 | 说明 |
|------|------|
| `list_databases` | 列出所有数据库 |
| `list_tables` | 列出指定数据库中的表 |
| `get_schema` | 获取表结构（字段定义） |
| `create_database` | 创建数据库 |
| `create_table` | 创建表 |
| `create_field` | 创建字段 |
| `execute_query` | 执行 Query DSL 查询 |
| `insert_records` | 插入记录 |
| `update_record` | 更新记录 |
| `delete_record` | 删除记录 |
| `generate_test_data` | 生成测试数据 |

---

## 使用示例

### 创建数据库和表

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_master_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Create a database named CRM with a customers table containing name, email, phone fields"}'
```

### 查询数据

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Find all customers whose email contains @gmail.com"}'
```

### 生成测试数据

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Generate 50 test customers in the CRM database"}'
```

---

## 错误处理

| 状态码 | 说明 |
|--------|------|
| `400` | 请求错误（缺少 `message` 字段） |
| `401` | 令牌无效或缺失 |
| `503` | AI 服务不可用（`LLM_API_KEY` 未配置） |
| `500` | LLM API 调用失败 |

---

## 注意事项

1. **权限隔离**：AI 助手使用当前令牌的权限上下文，无法执行未授权的操作。
2. **无状态**：每次聊天相互独立；AI 不会记住之前的对话（除非通过 `context` 传入）。
3. **透明工具调用**：AI 的所有数据操作均通过内部工具执行，等同于普通 API 调用。
4. **LLM 费用**：每次聊天都会调用 LLM API，请注意使用频率。

---

## 相关文档

- [MCP 设置](MCP-Setup.md) - 将 Cornerstone 连接到 MCP 客户端，如 Claude Desktop / Cline
- [令牌作用域](TokenScopes.md) - 控制 AI 助手的访问权限
- [Query DSL](Query.md) - AI 内部使用的查询语言
