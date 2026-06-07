[English](AI-Assistant.md) | [中文](AI-Assistant.zh.md)

# AI Assistant

> Interact with Cornerstone data using natural language.

---

## Enabling the AI Assistant

Configure the LLM in `.env`:

```bash
LLM_API_KEY=sk-your-api-key
LLM_MODEL=gpt-4o
# LLM_BASE_URL=https://api.openai.com/v1  # Optional: custom API endpoint
```

Restart the server, and the AI assistant will be active.

---

## Chat API

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "List all databases",
    "context": {}
  }'
```

Response:

```json
{
  "type": "result",
  "reply": "Here are your databases:\n- db_abc123: Project DB\n- db_def456: Test DB",
  "context": {}
}
```

### Parameters

| Field | Type | Required | Description |
|------|------|:----:|------|
| `message` | string | ✅ | User message |
| `context` | object | ❌ | Optional context; pass additional information to guide the AI |

---

## AI Tool List

The AI assistant internally invokes the following tools to interact with data. Permissions are the same as a regular token (subject to Scope restrictions):

| Tool | Description |
|------|------|
| `list_databases` | List all databases |
| `list_tables` | List tables in a specified database |
| `get_schema` | Get table structure (field definitions) |
| `create_database` | Create a database |
| `create_table` | Create a table |
| `create_field` | Create a field |
| `execute_query` | Execute a Query DSL query |
| `insert_records` | Insert records |
| `update_record` | Update a record |
| `delete_record` | Delete a record |
| `generate_test_data` | Generate test data |

---

## Usage Examples

### Create a Database and Table

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_master_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Create a database named CRM with a customers table containing name, email, phone fields"}'
```

### Query Data

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Find all customers whose email contains @gmail.com"}'
```

### Generate Test Data

```bash
curl -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Generate 50 test customers in the CRM database"}'
```

---

## Error Handling

| Status Code | Description |
|--------|------|
| `400` | Bad request (missing `message` field) |
| `401` | Invalid or missing token |
| `503` | AI service unavailable (`LLM_API_KEY` not configured) |
| `500` | LLM API call failed |

---

## Notes

1. **Permission Isolation**: The AI assistant uses the current token's permission context and cannot perform unauthorized operations.
2. **Stateless**: Each chat is independent; the AI does not remember previous conversations (unless passed via `context`).
3. **Transparent Tool Calls**: All data operations by the AI are executed through internal tools, equivalent to normal API calls.
4. **LLM Cost**: Each chat invokes the LLM API; be mindful of usage frequency.

---

## Related Documentation

- [MCP Setup](MCP-Setup.md) - Connect Cornerstone to MCP clients such as Claude Desktop / Cline
- [Token Scopes](TokenScopes.md) - Control the AI assistant's access permissions
- [Query DSL](Query.md) - The query language used internally by the AI
