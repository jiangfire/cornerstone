# 迭代 3 计划：S3 文件存储支持

## 目标

将文件存储从本地目录扩展为支持 S3 兼容的对象存储，同时保持本地存储作为默认选项。

---

## 现状

- `FileService` 硬编码写入 `./uploads` 本地目录。
- `StorageURL` 存的是本地绝对路径。
- 上传、下载、删除均直接操作本地文件系统。

---

## 实施方案

### 1. 抽象存储接口

```go
type StorageProvider interface {
    Upload(ctx context.Context, key string, reader io.Reader, size int64) (string, error)
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
}
```

### 2. 两种实现

#### LocalStorageProvider
- 保持现有 `./uploads` 行为。
- `Upload` 返回本地相对路径作为 key。
- `Download` 打开本地文件。
- `Delete` 删除本地文件。

#### S3StorageProvider
- 基于 AWS SDK for Go v2（`github.com/aws/aws-sdk-go-v2`）或 MinIO client。
- 配置参数：
  - `Endpoint`
  - `Bucket`
  - `Region`
  - `AccessKey`
  - `SecretKey`
- `Upload` 上传至 S3，返回 object key。
- `Download` 从 S3 下载。
- `Delete` 删除 S3 对象。

### 3. 配置扩展

在 `internal/config/config.go` 中增加：

```go
FileStorage struct {
    Type        string // "local" | "s3"
    LocalDir    string // 默认 "./uploads"
    S3Endpoint  string
    S3Bucket    string
    S3Region    string
    S3AccessKey string
    S3SecretKey string
}
```

环境变量映射示例：
- `FILE_STORAGE_TYPE`
- `FILE_STORAGE_S3_ENDPOINT`
- `FILE_STORAGE_S3_BUCKET`
- `FILE_STORAGE_S3_REGION`
- `FILE_STORAGE_S3_ACCESS_KEY`
- `FILE_STORAGE_S3_SECRET_KEY`

### 4. Service 层适配

- `FileService` 持有 `StorageProvider` 实例（通过配置初始化）。
- `UploadFile`：调用 `provider.Upload`，将返回的 key 存入 `StorageURL`。
- `DeleteFile`：调用 `provider.Delete`。

### 5. Download Handler 适配

- **本地模式**：继续使用 `c.FileAttachment(safePath, file.FileName)`。
- **S3 模式**：
  - 方案 A（推荐）：返回 `302 Redirect` 到 S3 预签名 URL，减少服务器流量。
  - 方案 B（备选）：代理下载，从 S3 读取后流式返回给客户端。

---

## 涉及文件

### 新增
- `internal/services/storage_local.go`
- `internal/services/storage_s3.go`

### 修改
- `internal/config/config.go`
- `internal/services/file.go`
- `internal/handlers/file.go`

---

## 验证清单

- [ ] `make test` 通过
- [ ] `make fmt` / `make vet` / `make lint` 无新增警告
- [ ] 本地模式：上传、下载、删除文件行为与之前一致
- [ ] S3 模式：文件成功上传至 S3，下载返回预签名 URL，删除清理 S3 对象
- [ ] 配置切换：修改 `FILE_STORAGE_TYPE` 后重启服务，存储后端正确切换
