# MinIO 对象存储客户端

这是一个增强的MinIO客户端封装，集成了zlog日志记录，提供了完整的对象存储操作功能。

## 🚀 功能特性

- ✅ 完整的CRUD操作（创建、读取、更新、删除）
- ✅ 预签名URL生成（上传/下载）
- ✅ 对象存在性检查
- ✅ 批量对象列表
- ✅ 对象复制
- ✅ 自动Content-Type检测
- ✅ 用户元数据支持
- ✅ 集成zlog日志记录
- ✅ 性能监控（请求耗时）
- ✅ 错误处理和重试机制

## 📦 安装依赖

```bash
go get github.com/minio/minio-go/v7
```

## 🛠️ 配置

### 配置结构

```go
type MinioConf struct {
    AK       string `yaml:"ak"`       // Access Key
    SK       string `yaml:"sk"`       // Secret Key  
    Endpoint string `yaml:"endpoint"` // MinIO服务端点
    UseSSL   bool   `yaml:"useSSL"`   // 是否使用SSL
    Region   string `yaml:"region"`   // 区域
}
```

### YAML配置示例

```yaml
minio:
  ak: "your-access-key"
  sk: "your-secret-key"
  endpoint: "http://localhost:9000"
  useSSL: false
  region: "us-east-1"
```

## 📖 使用方法

### 1. 创建客户端

```go
config := MinioConf{
    AK:       "your-access-key",
    SK:       "your-secret-key",
    Endpoint: "http://localhost:9000",
    UseSSL:   false,
    Region:   "us-east-1",
}

client, err := NewMinioClient(config)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### 2. 创建存储桶

```go
err := client.CreateBucket(ctx, "my-bucket", "")
if err != nil {
    log.Fatal(err)
}
```

### 3. 上传文件

#### 从内存上传

```go
content := []byte("Hello, MinIO!")
reader := bytes.NewReader(content)

uploadInfo, err := client.UploadFile(ctx, "my-bucket", "hello.txt", reader, int64(len(content)), &UploadOptions{
    ContentType: "text/plain",
    UserMeta: map[string]string{
        "uploaded-by": "myapp",
        "category":    "documents",
    },
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Upload successful, ETag: %s\n", uploadInfo.ETag)
```

#### 从本地文件上传

```go
uploadInfo, err := client.UploadFileFromPath(ctx, "my-bucket", "documents/file.pdf", "/path/to/local/file.pdf", &UploadOptions{
    ContentType: "application/pdf",
})
if err != nil {
    log.Fatal(err)
}
```

### 4. 下载文件

#### 下载到内存

```go
reader, downloadInfo, err := client.DownloadFile(ctx, "my-bucket", "hello.txt")
if err != nil {
    log.Fatal(err)
}
defer reader.Close()

content, err := io.ReadAll(reader)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Content: %s\n", string(content))
```

#### 下载到本地文件

```go
err := client.DownloadFileToPath(ctx, "my-bucket", "hello.txt", "/tmp/downloaded.txt")
if err != nil {
    log.Fatal(err)
}
```

### 5. 获取预签名URL

#### 获取下载URL

```go
// 获取24小时有效的下载URL
downloadURL, err := client.GetDownloadURL(ctx, "my-bucket", "hello.txt", 24*time.Hour)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Download URL: %s\n", downloadURL)
```

#### 获取上传URL

```go
// 获取1小时有效的上传URL
uploadURL, err := client.GetUploadURL(ctx, "my-bucket", "new-file.txt", time.Hour)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Upload URL: %s\n", uploadURL)
```

### 6. 对象管理

#### 检查对象是否存在

```go
exists, err := client.ObjectExists(ctx, "my-bucket", "hello.txt")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Object exists: %v\n", exists)
```

#### 获取对象信息

```go
info, err := client.GetObjectInfo(ctx, "my-bucket", "hello.txt")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Size: %d, Modified: %v\n", info.Size, info.LastModified)
```

#### 列出对象

```go
objects, err := client.ListObjects(ctx, "my-bucket", "documents/", false)
if err != nil {
    log.Fatal(err)
}

for _, obj := range objects {
    fmt.Printf("Object: %s, Size: %d\n", obj.Key, obj.Size)
}
```

#### 复制对象

```go
err := client.CopyObject(ctx, "my-bucket", "hello.txt", "my-bucket", "backup/hello.txt")
if err != nil {
    log.Fatal(err)
}
```

#### 删除对象

```go
err := client.DeleteFile(ctx, "my-bucket", "hello.txt")
if err != nil {
    log.Fatal(err)
}
```

## 🌐 Web应用集成

### Gin框架文件上传示例

```go
func uploadHandler(c *gin.Context) {
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(400, gin.H{"error": "Failed to get file"})
        return
    }
    defer file.Close()

    // 上传到MinIO
    uploadInfo, err := minioClient.UploadFile(c, "uploads", header.Filename, file, header.Size, &UploadOptions{
        ContentType: header.Header.Get("Content-Type"),
        UserMeta: map[string]string{
            "original-filename": header.Filename,
            "uploaded-at":      time.Now().Format(time.RFC3339),
        },
    })
    if err != nil {
        c.JSON(500, gin.H{"error": "Upload failed"})
        return
    }

    // 生成下载URL
    downloadURL, _ := minioClient.GetDownloadURL(c, "uploads", header.Filename, 24*time.Hour)

    c.JSON(200, gin.H{
        "message":     "Upload successful",
        "filename":    header.Filename,
        "size":        uploadInfo.Size,
        "downloadURL": downloadURL,
    })
}
```

## 📊 日志记录

客户端自动记录所有操作的日志，包括：

- 操作类型和参数
- 执行时间统计
- 错误信息
- 请求ID追踪

日志示例：
```
2025-01-15 10:30:15.123 INFO file uploaded successfully: my-bucket/hello.txt, size: 13, etag: "d41d8cd98f00b204e9800998ecf8427e", cost: 45ms
2025-01-15 10:30:16.456 INFO presigned URL generated: my-bucket/hello.txt, method: GET, expiry: 24h0m0s, cost: 12ms
```

## ⚡ 性能优化

### 连接池配置

客户端使用MinIO Go SDK的内置连接池，建议的配置：

```go
// 对于高并发场景
config := MinioConf{
    // ... 基础配置
    Region: "us-east-1", // 指定区域提高性能
}
```

### 批量操作

对于大量文件操作，建议使用批量接口：

```go
// 批量列出对象
objects, err := client.ListObjects(ctx, "my-bucket", "prefix/", true)
```

## 🔧 错误处理

客户端提供详细的错误信息和日志记录：

```go
uploadInfo, err := client.UploadFile(ctx, bucket, object, reader, size, opts)
if err != nil {
    // 日志已自动记录，可以直接处理业务逻辑
    return fmt.Errorf("failed to upload file: %w", err)
}
```

## 🎯 最佳实践

1. **资源管理**: 始终使用 `defer client.Close()` 和 `defer reader.Close()`
2. **错误处理**: 检查所有操作的错误返回值
3. **并发控制**: 对于高并发场景，考虑使用连接池
4. **缓存策略**: 对预签名URL进行适当缓存
5. **监控告警**: 监控上传/下载成功率和响应时间

## 📋 支持的文件类型

客户端自动检测以下文件类型的Content-Type：

- 图片: jpg, jpeg, png, gif
- 文档: pdf, txt, html, json, xml
- 压缩: zip
- 媒体: mp4, mp3
- 默认: application/octet-stream

## 🆘 常见问题

### Q: 如何处理大文件上传？
A: 对于大文件，建议使用分片上传或预签名URL让客户端直接上传。

### Q: 如何设置文件过期时间？
A: MinIO支持生命周期管理，需要在服务端配置存储桶策略。

### Q: 如何处理并发上传？
A: 客户端是线程安全的，可以在多个goroutine中并发使用。

### Q: 预签名URL的最大有效期是多少？
A: 默认最大7天，可以根据MinIO服务端配置调整。