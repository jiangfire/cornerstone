[English](TokenScopes.md) | [中文](TokenScopes.zh.md)

# Token Scopes

> Controls Token access permissions to databases, tables, and fields.

---

## Overview

Cornerstone's permission system is based on the **Token + Scope** model:

- **Master Token**: Has full permissions; auto-generated at startup or preset via `MASTER_TOKEN`
- **Regular Token**: Created via API or CLI; must be configured with a Scope to limit its access range

Scope is a JSON object that defines which databases and tables a Token can access, along with its role on those resources.

---

## Scope Format

```json
{
  "databases": {
    "db_xxx": "admin",
    "db_yyy": "editor"
  },
  "tables": {
    "tbl_aaa": {
      "role": "viewer",
      "fields": {
        "fld_name": ["read"],
        "fld_email": ["read", "write"]
      }
    }
  }
}
```

### Field Descriptions

| Field | Type | Description |
|------|------|------|
| `databases` | `map[string]string` | Database ID -> Role. Role can be `viewer`/`editor`/`admin` |
| `tables` | `map[string]TableScope` | Table ID -> Table-level permission config |
| `tables[table_id].role` | `string` | Role on this table |
| `tables[table_id].fields` | `map[string][]string` | Field-level permissions (optional); field ID/name -> list of actions |

### Role Permissions

| Role | read | write | delete | manage |
|------|:----:|:-----:|:------:|:------:|
| `viewer` | ✅ | ❌ | ❌ | ❌ |
| `editor` | ✅ | ✅ | ❌ | ❌ |
| `admin` | ✅ | ✅ | ✅ | ✅ |

> `manage` includes updating/deleting the resource itself (e.g., modifying table structure, deleting databases).

---

## Permission Inheritance Rules

1. **Master Token** always has full permissions
2. **Table permissions** can be set independently; if not set, they inherit from the parent database's permissions
3. **Field permissions** can further refine access; if not set, they inherit from the table's permissions
4. **Action matching** is case-insensitive; `read`/`READ`/`Read` are equivalent

### Inheritance Example

```json
{
  "databases": {
    "db_project": "editor"
  },
  "tables": {
    "tbl_users": {
      "role": "viewer"
    }
  }
}
```

- All tables under `db_project`: default `editor` (read/write)
- `tbl_users`: explicitly set to `viewer` (read-only), overriding the database-level `editor`

---

## Creating a Token with Scope

### CLI

```bash
# Write JSON scope directly (note outer quotes)
cornerstone token create "dev-team" \
  -s '{"databases":{"db_xxx":"editor"}}'

# Or read from a file
cornerstone token create "dev-team" -s "$(cat scope.json)"
```

### REST API

```bash
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "Authorization: Bearer cs_master_token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dev-team",
    "scopes": "{\"databases\":{\"db_xxx\":\"editor\"}}"
  }'
```

> In the API, the `scopes` field is a **string** (JSON-serialized Scope object), not an object.

---

## Field-Level Permissions

To restrict a Token to access only specific fields:

```json
{
  "tables": {
    "tbl_users": {
      "role": "viewer",
      "fields": {
        "fld_name": ["read"],
        "fld_email": ["read"]
      }
    }
  }
}
```

This Token's queries on `tbl_users` will only return `fld_name` and `fld_email`; all other fields are invisible.

Field keys can be either a **field ID** (e.g., `fld_xxx`) or a **field name** (e.g., `name`).

---

## Best Practices

1. **Principle of Least Privilege**: Only grant the minimum permissions a Token needs to complete its task
2. **Database-Level Defaults**: Assign a default role at the database level first, then downgrade sensitive tables
3. **Field-Level Masking**: Restrict fields containing sensitive information (e.g., phone numbers, ID numbers) individually
4. **Token Rotation**: Regularly delete old Tokens and create new ones
5. **Expiration Time**: Set `expires_at` for temporary/scenario-specific Tokens to avoid long-term validity

---

## Troubleshooting

| Issue | Cause | Solution |
|------|------|------|
| `permission denied: cannot access this database` | Token does not have a scope for this database | Check whether `scopes.databases` includes the target database ID |
| `permission denied: cannot access this table` | Token does not have a scope for this table, and database-level permissions are insufficient | Add it to `scopes.tables` or `scopes.databases` |
| `field 'xxx' is not in the allowed list` | The Query DSL requests a field that is not authorized | Check whether the field is in the `fields` whitelist of the scope |
| `master token required for this operation` | The operation requires a Master Token (e.g., creating a database) | Use a Master Token or promote the target Token to Master (not recommended) |

---

## Related Documentation

- [Query DSL](Query.md) - Permission validation logic during queries
- [REST API](README.md#rest-api) - Token management endpoints
