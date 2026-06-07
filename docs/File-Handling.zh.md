[English](File-Handling.md) | [中文](File-Handling.zh.md)

# 文件处理

> 文件上传、下载与管理的完整指南。

---

## 概述

Cornerstone 支持上传文件并将其与记录关联。文件存储在本地文件系统中，元数据和存储路径保存在数据库中。

---

## 存储路径

- **默认路径**：`./uploads`（相对于工作目录）
- **安全校验**：所有下载/删除路径都通过 `ResolveSecureStoragePath` 进行校验，确保不会发生 `./uploads` 目录外的路径遍历
- **配置**：目前不支持自定义存储路径；建议通过卷挂载映射存储位置

---

## 默认限制

| 限制项 | 默认值 | 说明 |
|------------|---------------|-------------|
| 单文件最大大小 | 10MB | 系统级限制，可通过数据库中的 `max_upload_size_mb` 设置调整 |
| 允许的文件类型 | `.jpg/.jpeg/.png/.gif/.pdf/.doc/.docx/.xls/.xlsx/.txt/.zip` | 默认白名单 |
| 存储路径 | `./uploads` | 相对于工作目录 |

---

## 字段级限制

你可以在 `file` 类型字段的配置中指定更细粒度的限制：

```json
{
  "type": "file",
  "options": {
    "max_file_size_mb": 5,
    "allowed_types": [".jpg", ".png"],
    "multiple": false
  }
}
```

| 配置项 | 类型 | 默认值 | 说明 |
|--------------------|------|---------------|-------------|
| `max_file_size_mb` | int | 系统限制 | 该字段允许的最大文件大小（MB） |
| `allowed_types` | string[] | 系统白名单 | 允许的文件扩展名 |
| `multiple` | bool | false | 是否允许多文件上传 |

---

## API 使用

### 上传文件

```bash
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer cs_your_token" \
  -F "record_id=rec_xxx" \
  -F "field_id=fld_yyy" \
  -F "file=@/path/to/document.pdf"
```

参数：
- `record_id` - 关联的记录 ID（可选，但建议填写）
- `field_id` - 关联的文件字段 ID（可选）
- `file` - 文件内容

### 获取文件信息

```bash
curl http://localhost:8080/api/v1/files/fil_xxx \
  -H "Authorization: Bearer cs_your_token"
```

### 下载文件

```bash
curl http://localhost:8080/api/v1/files/fil_xxx/download \
  -H "Authorization: Bearer cs_your_token" \
  -O
```

### 删除文件

```bash
curl -X DELETE http://localhost:8080/api/v1/files/fil_xxx \
  -H "Authorization: Bearer cs_your_token"
```

### 列出记录关联的文件

```bash
curl http://localhost:8080/api/v1/records/rec_xxx/files \
  -H "Authorization: Bearer cs_your_token"
```

---

## 安全注意事项

1. **路径遍历防护**：所有文件路径都通过 `ResolveSecureStoragePath` 进行校验，确保只能访问 `./uploads` 内的文件
2. **文件名校验**：检查 `/` 或 `\`，防止上传到非预期位置
3. **权限隔离**：文件访问由关联记录所在表的权限控制
4. **物理删除**：删除 API 会同时移除数据库记录和物理文件

---

## Docker 部署

在 Docker 环境中，建议将 `uploads` 目录挂载为持久化卷：

```yaml
services:
  cornerstone:
    image: ghcr.io/jiangfire/cornerstone:v1.6.0
    volumes:
      - ./uploads:/app/uploads
      - ./cornerstone.db:/app/cornerstone.db
```

---

## 故障排查

| 问题 | 原因 | 解决方案 |
|-------|-------|----------|
| `file size exceeds limit` | 文件超出系统或字段限制 | 压缩文件或调整 `max_file_size_mb` |
| `unsupported file type` | 文件类型不在白名单中 | 检查文件扩展名，或将其添加到字段配置中 |
| `illegal file name` | 文件名包含非法字符 | 重命名文件 |
| `file not found` | 文件已被删除或路径被篡改 | 检查文件是否存在，或验证数据库记录 |
| `permission denied` | Token 没有访问该记录的权限 | 检查 Token 的作用范围 |

---

## 相关文档

- [REST API](README.md#rest-api) - API 端点完整列表
- [Architecture](Architecture.md) - 文件存储在系统架构中的位置
