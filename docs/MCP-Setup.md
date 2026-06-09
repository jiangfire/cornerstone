[English](MCP-Setup.md) | [中文](MCP-Setup.zh.md)

# MCP Client Setup

> Integrate Cornerstone into AI clients that support the MCP protocol.

---

## Supported Clients

- [Claude Desktop](https://claude.ai/download)
- [Cline](https://github.com/cline/cline) (VS Code extension)
- [Other SSE MCP clients](https://modelcontextprotocol.io/clients)

---

## Connection Methods

Cornerstone provides two transport methods:

| Method | Endpoint | Description |
|--------|----------|-------------|
| **SSE Event Stream** | `GET /mcp` | Long-lived connection, suitable for real-time interaction |
| **JSON-RPC** | `POST /mcp` | Request/response, suitable for simple calls |

Authentication: All requests must carry the `Authorization: Bearer <token>` header.

---

## Claude Desktop Configuration

Add the following to Claude Desktop's `claude_desktop_config.json`:

### macOS

```json
{
  "mcpServers": {
    "cornerstone": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sse"],
      "env": {
        "SSE_URL": "http://localhost:8080/mcp",
        "AUTH_TOKEN": "cs_your_token"
      }
    }
  }
}
```

Config file path: `~/Library/Application Support/Claude/claude_desktop_config.json`

### Windows

```json
{
  "mcpServers": {
    "cornerstone": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sse"],
      "env": {
        "SSE_URL": "http://localhost:8080/mcp",
        "AUTH_TOKEN": "cs_your_token"
      }
    }
  }
}
```

Config file path: `%APPDATA%\Claude\claude_desktop_config.json`

### Restart Claude Desktop

After saving the configuration, restart Claude Desktop. A **hammer icon** should appear in the left sidebar — click it to see the Cornerstone tool list.

---

## Cline (VS Code) Configuration

Add the following to Cline's MCP settings:

```json
{
  "mcpServers": [
    {
      "name": "cornerstone",
      "transport": "sse",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer cs_your_token"
      }
    }
  ]
}
```

---

## Available Tools

After connecting, the AI client can call the following tools:

### Database Management
- `create_database` - Create a database
- `list_databases` - List databases
- `get_database` - Get database details
- `update_database` - Update a database
- `delete_database` - Delete a database
- `create_database_with_tables` - Create database + tables + fields in one go

### Table Management
- `create_table` - Create a table
- `list_tables` - List tables
- `get_table` - Get table details
- `update_table` - Update a table
- `delete_table` - Delete a table

### Field Management
- `create_field` - Create a field
- `list_fields` - List fields
- `update_field` - Update a field
- `delete_field` - Delete a field

### Record Management
- `insert_record` - Insert a record
- `list_records` - List records (paginated)
- `get_record` - Get a single record
- `update_record` - Update a record
- `delete_record` - Delete a record
- `batch_insert_records` - Batch insert records
- `generate_test_data` - Generate test data

### Query
- `query_data` - Query DSL query
- `get_table_schema` - Get system table field schema

---

## JSON-RPC Request Examples

All MCP tools are called via JSON-RPC over HTTP POST:

### Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-03-26",
    "capabilities": {},
    "clientInfo": {"name": "my-client", "version": "1.0"}
  }
}
```

### List Tools

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

### Call Tool - Create Database

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "create_database",
    "arguments": {
      "name": "my_project",
      "description": "Project database"
    }
  }
}
```

### Call Tool - Query Data (Query DSL)

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "query_data",
    "arguments": {
      "from": "records",
      "table": "tbl_abc123",
      "page": 1,
      "size": 10
    }
  }
}
```

> **Note**: `query_data` accepts Query DSL fields **directly** in `arguments` (same structure as the REST API `/api/v1/query` endpoint). There is no extra `query` wrapper.

### Call Tool - JOIN Query

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "query_data",
    "arguments": {
      "from": "records",
      "select": ["records.id", "records.data"],
      "join": [
        {
          "type": "left",
          "table": "tables",
          "as": "t",
          "on": {"left": "records.table_id", "op": "=", "right": "t.id"},
          "select": ["t.name"]
        }
      ],
      "where": {
        "and": [{"field": "table_id", "op": "eq", "value": "tbl_abc123"}]
      },
      "page": 1,
      "size": 10
    }
  }
}
```

> **Tip**: When using JOIN, always use qualified names like `records.id` in `select` to avoid ambiguous column errors.

---

## SSE Stream Features

### Keepalive

The SSE stream sends a keepalive comment every 25 seconds to ensure the connection is not timed out by proxies/gateways.

Adjustable via environment variable:

```bash
MCP_SSE_KEEPALIVE_SEC=25
```

### Reconnection

Supports reconnection and message replay via the `Last-Event-ID` header:

```
GET /mcp
Accept: text/event-stream
Last-Event-ID: <event-id>
```

Replay buffer defaults to 128 messages, adjustable via environment variable:

```bash
MCP_SSE_REPLAY_BUFFER=128
```

### Retry Interval

SSE stream retry interval defaults to 3000ms, adjustable via environment variable:

```bash
MCP_SSE_RETRY_MS=3000
```

---

## CORS Configuration

If the AI client and Cornerstone run on different domains, configure allowed origins:

```bash
MCP_ALLOWED_ORIGINS=https://claude.ai,https://app.claude.ai
```

Leave empty to allow any origin (development only).

---

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| Client cannot connect | Service not started or port blocked | Ensure `cornerstone serve` is running, check firewall |
| 401 Unauthorized | Invalid or missing token | Verify `Authorization: Bearer <token>` is correct |
| Empty tool list | SSE stream not established correctly | Check if `Accept: text/event-stream` header is correct |
| Cannot perform operation | Insufficient token permissions | Check if the token's Scope includes the target resource |
| SSE stream frequently disconnects | Proxy timeout | Increase `MCP_SSE_KEEPALIVE_SEC`, ensure proxy doesn't close long-lived connections |

---

## Related Documentation

- [AI Assistant](AI-Assistant.md) - Call AI via HTTP API
- [Token Scopes](TokenScopes.md) - Control AI client access permissions
- [Architecture](Architecture.md) - MCP protocol's position in the system
