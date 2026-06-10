[English](File-Handling.md) | [中文](File-Handling.zh.md)

# File Handling

> Complete guide for file upload, download, and management.

---

## Overview

Cornerstone supports uploading files and associating them with records. Files can be stored on the **local filesystem** or in **S3-compatible object storage** (AWS S3, MinIO, etc.), with metadata and storage paths saved in the database.

---

## Storage Path

### Local Storage (default)

- **Default Path**: `./uploads` (relative to the working directory)
- **Custom Path**: Set `FILE_STORAGE_LOCAL_DIR` environment variable
- **Security Validation**: All download/delete paths are validated to ensure no path traversal outside the configured directory
- **Configuration**: Custom storage paths via `FILE_STORAGE_LOCAL_DIR` environment variable

### S3-Compatible Object Storage

Set `FILE_STORAGE_TYPE=s3` and configure the following environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `FILE_STORAGE_S3_ENDPOINT` | Yes | - | S3 endpoint (e.g., `s3.amazonaws.com` or `minio.example.com:9000`) |
| `FILE_STORAGE_S3_BUCKET` | Yes | - | Bucket name |
| `FILE_STORAGE_S3_REGION` | No | `us-east-1` | Region |
| `FILE_STORAGE_S3_ACCESS_KEY` | Yes | - | Access key ID |
| `FILE_STORAGE_S3_SECRET_KEY` | Yes | - | Secret access key |
| `FILE_STORAGE_S3_SECURE` | No | `true` | Use HTTPS (`true`) or HTTP (`false`). Set to `false` for local MinIO |

Example `.env` for AWS S3:

```env
FILE_STORAGE_TYPE=s3
FILE_STORAGE_S3_ENDPOINT=s3.amazonaws.com
FILE_STORAGE_S3_BUCKET=my-cornerstone-files
FILE_STORAGE_S3_REGION=us-east-1
FILE_STORAGE_S3_ACCESS_KEY=AKIA...
FILE_STORAGE_S3_SECRET_KEY=...
FILE_STORAGE_S3_SECURE=true
```

Example `.env` for local MinIO:

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

## Default Limits

| Limit Item | Default Value | Description |
|------------|---------------|-------------|
| Max Single File Size | 10MB | System-level limit, adjustable via the `max_upload_size_mb` database setting |
| Allowed File Types | `.jpg/.jpeg/.png/.gif/.pdf/.doc/.docx/.xls/.xlsx/.txt/.zip` | Default whitelist |
| Storage Path | `./uploads` | Relative to the working directory (local storage only) |

---

## Field-Level Limits

You can specify finer restrictions in the configuration of `file` type fields:

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

| Configuration Item | Type | Default Value | Description |
|--------------------|------|---------------|-------------|
| `max_file_size_mb` | int | System limit | Maximum file size allowed for this field (MB) |
| `allowed_types` | string[] | System whitelist | Allowed file extensions |
| `multiple` | bool | false | Whether multiple file uploads are allowed |

---

## API Usage

### Upload File

```bash
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer cs_your_token" \
  -F "record_id=rec_xxx" \
  -F "field_id=fld_yyy" \
  -F "file=@/path/to/document.pdf"
```

Parameters:
- `record_id` - Associated record ID (optional, but recommended)
- `field_id` - Associated file field ID (optional)
- `file` - File content

### Get File Info

```bash
curl http://localhost:8080/api/v1/files/fil_xxx \
  -H "Authorization: Bearer cs_your_token"
```

### Download File

```bash
curl http://localhost:8080/api/v1/files/fil_xxx/download \
  -H "Authorization: Bearer cs_your_token" \
  -O
```

### Delete File

```bash
curl -X DELETE http://localhost:8080/api/v1/files/fil_xxx \
  -H "Authorization: Bearer cs_your_token"
```

### List Record-Associated Files

```bash
curl http://localhost:8080/api/v1/records/rec_xxx/files \
  -H "Authorization: Bearer cs_your_token"
```

---

## Security Considerations

### Local Storage

1. **Path Traversal Protection**: All file paths are validated to ensure only files within the configured storage directory can be accessed
2. **Filename Validation**: Checks for `/` or `\` to prevent uploading to unintended locations
3. **Permission Isolation**: File access is controlled by the permissions of the associated record's table
4. **Physical Deletion**: The delete API removes both the database record and the physical file

### S3 Storage

1. **HTTPS by Default**: S3 connections use HTTPS unless `FILE_STORAGE_S3_SECURE=false`
2. **Bucket Must Exist**: The application validates bucket existence on startup
3. **Presigned URLs**: S3 storage supports presigned download URLs for direct browser downloads (1-hour expiry)
4. **Credential Management**: Store S3 credentials in environment variables or `.env` files; never commit them to version control

---

## Docker Deployment

### Local Storage

In Docker environments, mount the `uploads` directory as a persistent volume:

```yaml
services:
  cornerstone:
    image: ghcr.io/jiangfire/cornerstone:latest
    volumes:
      - ./uploads:/app/uploads
      - ./cornerstone.db:/app/cornerstone.db
```

### S3 Storage

No volume mount needed for files. Configure S3 via environment variables:

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

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| `file size exceeds limit` | File exceeds system or field limit | Compress the file or adjust `max_file_size_mb` |
| `unsupported file type` | File type not in whitelist | Check the file extension, or add the type to the field configuration |
| `illegal file name` | Filename contains illegal characters | Rename the file |
| `file not found` | File has been deleted or path tampered with | Check if the file exists, or verify the database record |
| `permission denied` | Token does not have access to the record | Check the Token Scope |
| `failed to create S3 client` | Invalid S3 credentials or endpoint | Verify `FILE_STORAGE_S3_*` environment variables |
| `S3 bucket does not exist` | Bucket not found on startup | Create the bucket before starting the server |
| `connection refused` (S3) | S3 endpoint unreachable or `SECURE` mismatch | Check endpoint URL and ensure `FILE_STORAGE_S3_SECURE` matches the endpoint scheme |

---

## Related Documentation

- [REST API](README.md#rest-api) - Complete list of API endpoints
- [Architecture](Architecture.md) - Where file storage fits in the system architecture
