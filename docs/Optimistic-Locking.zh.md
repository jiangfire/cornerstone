[English](Optimistic-Locking.md) | [中文](Optimistic-Locking.zh.md)

# Optimistic Locking

> 通过版本号防止并发覆盖写入。

---

## 什么是乐观锁

当两个用户同时编辑同一条记录时，后保存的用户会覆盖前一个用户的修改。乐观锁通过版本号机制避免这种情况：

1. 读取记录时获取当前版本号 `version`
2. 更新时携带该版本号发起请求
3. 服务端检查当前版本号是否与请求版本号一致
4. 如果不一致（记录已被其他用户修改），返回错误，用户需要重新读取后再更新

---

## 版本号工作流程

```
用户 A                          用户 B
  │                                │
  ├─> 读取记录 (version=1)       │
  │                                ├─> 读取记录 (version=1)
  │                                │
  │                                ├─> 提交更新 (version=1)
  │                                │     → 成功，version 变为 2
  │                                │
  ├─> 提交更新 (version=1)       │
  │     → 失败！记录已被修改    │
  │                                │
  ├─> 重新读取 (version=2)      │
  ├─> 合并修改                  │
  ├─> 提交更新 (version=2)       │
  │     → 成功，version 变为 3   │
```

---

## API 使用

### 更新记录时传入版本号

```bash
curl -X PUT http://localhost:8080/api/v1/records/rec_xxx \
  -H "Authorization: Bearer cs_your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {"name": "New Name"},
    "version": 2
  }'
```

### CLI 更新

```bash
cornerstone record update rec_xxx '{"name":"New Name"}' --version 2
```

---

## 版本冲突响应

当版本号不匹配时：

```json
{
  "error": "record was modified by another user (current version: 3, requested version: 2)"
}
```

---

## 实现细节

- **版本号字段**：`records.version`，默认值 1
- **自增逻辑**：每次更新成功后 `version = version + 1`
- **原子更新**：使用 `WHERE version = ?` 条件，确保只有版本匹配时才更新
- **批量更新**：目前不支持批量更新时使用乐观锁，建议逐条更新

---

## 最佳实践

1. **总是读取后更新**：先 `GET` 获取记录和当前版本号，再 `PUT` 更新
2. **处理版本冲突**：前端收到 409 错误时，提示用户记录已被修改，请重新加载
3. **非必填**：`version` 参数可送不传，不传则不做版本检查（危险，仅建议内部脚本使用）
4. **删除不检查**：删除操作不检查版本号，请谨慎操作

---

## 相关文档

- [REST API](README.md#rest-api) - 记录更新端点
- [Architecture](Architecture.md) - 数据模型中的版本控制
