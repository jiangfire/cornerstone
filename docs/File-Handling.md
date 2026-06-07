[English](File-Handling.md) | [中文](File-Handling.zh.md)

# File Handling

> Complete guide for file upload, download, and management.

---

## Overview

Cornerstone supports uploading files and associating them with records. Files are stored on the local filesystem, with metadata and storage paths saved in the database.

---

## Storage Path

- **Default Path**: `./uploads` (relative to the working directory)
- **Security Validation**: All download/delete paths are validated through `ResolveSecureStoragePath` to ensure no path traversal outside `./uploads`
- **Configuration**: Custom storage paths are currently not supported; it is recommended to map the storage location via volume mounts

---

## Default Limits

| Limit Item | Default Value | Description |
|------------|---------------|-------------|
| Max Single File Size | 10MB | System-level limit, adjustable via the `max_upload_size_mb` database setting |
| Allowed File Types | `.jpg/.jpeg/.png/.gif/.pdf/.doc/.docx/.xls/.xlsx/.txt/.zip` | Default whitelist |
| Storage Path | `./uploads` | Relative to the working directory |

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

1. **Path Traversal Protection**: All file paths are validated through `ResolveSecureStoragePath` to ensure only files within `./uploads` can be accessed
2. **Filename Validation**: Checks for `/` or `\` to prevent uploading to unintended locations
3. **Permission Isolation**: File access is controlled by the permissions of the associated record's table
4. **Physical Deletion**: The delete API removes both the database record and the physical file

---

## Docker Deployment

In Docker environments, it is recommended to mount the `uploads` directory as a persistent volume:

```yaml
services:
  cornerstone:
    image: ghcr.io/jiangfire/cornerstone:v1.6.0
    volumes:
      - ./uploads:/app/uploads
      - ./cornerstone.db:/app/cornerstone.db
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

---

## Related Documentation

- [REST API](README.md#rest-api) - Complete list of API endpoints
- [Architecture](Architecture.md) - Where file storage fits in the system architecture
