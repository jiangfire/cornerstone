[English](File-Handling.md) | [中文](File-Handling.zh.md)

# 文件处理

> 文件上传、下载与管理的完整指南。

---

## 概述

Cornerstone 支持上传文件并将其与记录关联。文件可以存储在**本地文件系统**或 **S3 兼容的对象存储**（AWS S3、MinIO 等）中，元数据和存储路径保存在数据库中。

---

## 存储路径

### 本地存储（默认）

- **默认路径**：`./uploads`（相对于工作目录）
- **自定义路径**：通过 `FILE_STORAGE_LOCAL_DIR` 环境变量设置
- **安全校验**：所有下载/删除路径都进行校验，确保不会发生存储目录外的路径遍历
- **配置**：通过 `FILE_STORAGE_LOCAL_DIR` 环境变量自定义存储路径

### S3 兼容对象存储

设置 `FILE_STORAGE_TYPE=s3` 并配置以下环境变量：

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `FILE_STORAGE_S3_ENDPOINT` | 是 | - | S3 端点（如 `s3.amazonaws.com` 或 `minio.example.com:9000`） |
| `FILE_STORAGE_S3_BUCKET` | 是 | - | 存储桶名称 |
| `FILE_STORAGE_S3_REGION` | 否 | `us-east-1` | 区域 |
| `FILE_STORAGE_S3_ACCESS_KEY` | 是 | - | 访问密钥 ID |
| `FILE_STORAGE_S3_SECRET_KEY` | 是 | - | 密钥 |
| `FILE_STORAGE_S3_SECURE` | 否 | `true` | 使用 HTTPS（`true`）或 HTTP（`false`）。本地 MinIO 请设为 `false` |

AWS S3 示例 `.env`：

```env
FILE_STORAGE_TYPE=s3
FILE_STORAGE_S3_ENDPOINT=s3.amazonaws.com
FILE_STORAGE_S3_BUCKET=my-cornerstone-files
FILE_STORAGE_S3_REGION=us-east-1
FILE_STORAGE_S3_ACCESS_KEY=AKIA...
FILE_STORAGE_S3_SECRET_KEY=...
FILE_STORAGE_S3_SECURE=true
```

本地 MinIO 示例 `.env`：

```env
FILE_STORAGE_TYPE=s3
FILE_STORAGE_S3_ENDPOINT=localhost:9000
FILE_STORAGE_S3_BUCKET=cornerstone
FILE_STORAGE_S3_REGION=us-east-1
FILE_STORAGE_S3_ACCESS_KEY=minioadmin
FILE_STORAGE_S3_SECRET_KEY=minioadmin
FILE_STORAGE_S3_SECURE=false
```

---

## 默认限制

| 限制项 | 默认值 | 说明 |
|------------|---------------|-------------|
| 单文件最大大小 | 10MB | 系统级限制，可通过数据库中的 `max_upload_size_mb` 设置调整 |
| 允许的文件类型 | `.jpg/.jpeg/.png/.gif/.pdf/.doc/.docx/.xls/.xlsx/.txt/.zip` | 默认白名单 |
| 存储路径 | `./uploads` | 相对于工作目录（仅本地存储） |

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

### 本地存储

1. **路径遍历防护**：所有文件路径都进行校验，确保只能访问配置的存储目录内的文件
2. **文件名校验**：检查 `/` 或 `\`，防止上传到非预期位置
3. **权限隔离**：文件访问由关联记录所在表的权限控制
4. **物理删除**：删除 API 会同时移除数据库记录和物理文件

### S3 存储

1. **默认 HTTPS**：S3 连接默认使用 HTTPS，除非设置 `FILE_STORAGE_S3_SECURE=false`
2. **存储桶必须存在**：启动时会验证存储桶是否存在
3. **预签名 URL**：S3 存储支持预签名下载 URL，可直接在浏览器中下载（1 小时有效期）
4. **凭证管理**：将 S3 凭证存储在环境变量或 `.env` 文件中，切勿提交到版本控制

---

## Docker 部署

### 本地存储

在 Docker 环境中，将 `uploads` 目录挂载为持久化卷：

```yaml
services:
  cornerstone:
    image: ghcr.io/jiangfire/cornerstone:latest
    volumes:
      - ./uploads:/app/uploads
      - ./cornerstone.db:/app/cornerstone.db
```

### S3 存储

文件不需要卷挂载，通过环境变量配置 S3：

```yaml
services:
  cornerstone:
    image: ghcr.io/jiangfire/cornerstone:latest
    environment:
      FILE_STORAGE_TYPE: s3
      FILE_STORAGE_S3_ENDPOINT: minio.example.com:9000
      FILE_STORAGE_S3_BUCKET: cornerstone
      FILE_STORAGE_S3_REGION: us-east-1
      FILE_STORAGE_S3_ACCESS_KEY: ${S3_ACCESS_KEY}
      FILE_STORAGE_S3_SECRET_KEY: ${S3_SECRET_KEY}
      FILE_STORAGE_S3_SECURE: "true"
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
| `failed to create S3 client` | S3 凭证或端点无效 | 检查 `FILE_STORAGE_S3_*` 环境变量 |
| `S3 bucket does not exist` | 启动时找不到存储桶 | 在启动服务器之前创建存储桶 |
| `connection refused`（S3） | S3 端点不可达或 `SECURE` 配置不匹配 | 检查端点 URL，确保 `FILE_STORAGE_S3_SECURE` 与端点协议一致 |

---

## 相关文档

- [REST API](README.md#rest-api) - API 端点完整列表
- [Architecture](Architecture.md) - 文件存储在系统架构中的位置
