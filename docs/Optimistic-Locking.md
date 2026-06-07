[English](Optimistic-Locking.md) | [中文](Optimistic-Locking.zh.md)

# Optimistic Locking

> Prevent concurrent overwrites using version numbers.

---

## What is Optimistic Locking

When two users edit the same record simultaneously, the user who saves last overwrites the previous user's changes. Optimistic locking prevents this through a version number mechanism:

1. Read the record and obtain the current version number `version`
2. Include this version number when sending the update request
3. The server checks whether the current version matches the requested version
4. If they do not match (the record has been modified by another user), an error is returned and the user must re-read before updating

---

## Version Number Workflow

```
User A                          User B
  │                                │
  ├─> Read record (version=1)     │
  │                                ├─> Read record (version=1)
  │                                │
  │                                ├─> Submit update (version=1)
  │                                │     → Success, version becomes 2
  │                                │
  ├─> Submit update (version=1)   │
  │     → Failed! Record modified  │
  │                                │
  ├─> Re-read (version=2)        │
  ├─> Merge changes              │
  ├─> Submit update (version=2)   │
  │     → Success, version becomes 3│
```

---

## API Usage

### Pass Version Number When Updating Records

```bash
curl -X PUT http://localhost:8080/api/v1/records/rec_xxx \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {"name": "New Name"},
    "version": 2
  }'
```

### CLI Update

```bash
cornerstone record update rec_xxx '{"name":"New Name"}' --version 2
```

---

## Version Conflict Response

When the version numbers do not match:

```json
{
  "error": "record was modified by another user (current version: 3, requested version: 2)"
}
```

---

## Implementation Details

- **Version field**: `records.version`, default value 1
- **Increment logic**: `version = version + 1` after each successful update
- **Atomic update**: Uses `WHERE version = ?` condition to ensure updates only occur when versions match
- **Batch updates**: Bulk updates currently do not support optimistic locking; update records individually

---

## Best Practices

1. **Always read before updating**: First `GET` the record and its current version, then `PUT` the update
2. **Handle version conflicts**: When the frontend receives a 409 error, prompt the user that the record has been modified and should be reloaded
3. **Optional**: The `version` parameter is optional; omitting it skips version checks (dangerous, recommended for internal scripts only)
4. **Deletion unchecked**: Delete operations do not check version numbers; proceed with caution

---

## Related Documentation

- [REST API](README.md#rest-api) - Record update endpoint
- [Architecture](Architecture.md) - Version control in the data model
