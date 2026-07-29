# 通用 S3 对象存储客户端

该模块基于 AWS SDK for Go v2，为 AWS S3 和兼容 S3 协议的对象存储提供统一的
Go API。公开接口只使用本模块的配置和结果类型，不暴露 AWS、MinIO、GCS 或 OSS
SDK 类型。

## 安装

```bash
go get github.com/xiangtao94/golib/pkg/s3@<version>
```

## 配置

```go
client, err := s3.NewClient(ctx, s3.Config{
    Endpoint:        "https://storage.example.com",
    Region:          "us-east-1",
    AccessKeyID:     accessKey,
    SecretAccessKey: secretKey,
    UsePathStyle:    true,
})
if err != nil {
    return err
}
defer client.Close()
```

- `Endpoint` 为空时使用 AWS SDK 的标准 S3 endpoint。
- `AccessKeyID` 和 `SecretAccessKey` 同时为空时使用 AWS 默认凭证链，包括环境变量、
  shared config、Web Identity 和实例角色。
- HTTP endpoint 默认拒绝，只能为本地开发显式设置 `AllowHTTP`。
- 自定义 `TLSConfig` 会被复制，模块不会修改调用方持有的配置；TLS 默认最低为 1.2。
- `UsePathStyle` 必须按服务能力配置。

常见配置：

| 服务 | Endpoint | UsePathStyle | 凭证 |
|---|---|---:|---|
| AWS S3 | 留空 | `false` | AWS 默认凭证链或静态凭证 |
| MinIO / RustFS | 服务地址 | 通常为 `true` | Access Key / Secret Key |
| Google Cloud Storage XML API | `https://storage.googleapis.com` | 按桶域名配置 | GCS HMAC Key |
| 阿里云 OSS S3 API | 对应地域 OSS endpoint | `false` | 阿里云 AccessKey |

GCS 与 OSS 只保证各自声明兼容的 S3 API。使用前应在目标服务上运行建桶、上传、
下载、复制、分页列举和预签名 URL 的集成测试。

## 使用

```go
result, err := client.PutObject(
    ctx,
    "documents",
    "reports/july.txt",
    strings.NewReader("hello"),
    5,
    &s3.UploadOptions{
        ContentType: "text/plain",
        Metadata: map[string]string{
            "owner": "finance",
        },
    },
)
if err != nil {
    return err
}

object, err := client.GetObject(ctx, "documents", result.Key)
if err != nil {
    return err
}
defer object.Body.Close()

for object, err := range client.ListObjects(
    ctx,
    "documents",
    s3.ListOptions{Prefix: "reports/"},
) {
    if err != nil {
        return err
    }
    fmt.Println(object.Key)
}

downloadURL, err := client.PresignGetObject(
    ctx,
    "documents",
    result.Key,
    15*time.Minute,
)
```

主要操作：

- `CreateBucket`
- `PutObject` / `PutFile`
- `GetObject` / `GetFile`
- `StatObject` / `ObjectExists`
- `ListObjects`
- `CopyObject` / `DeleteObject`
- `PresignGetObject` / `PresignPutObject`

`PutObject` 使用 AWS multipart uploader；大对象会自动分片。`GetFile` 先写入目标目录
中的临时文件，完整写入并同步成功后才替换目标路径，失败不会留下半文件。
`ListObjects` 返回惰性迭代器，调用方停止遍历后不会继续请求后续分页。

## 错误语义

```go
_, err := client.StatObject(ctx, bucket, key)
switch {
case errors.Is(err, s3.ErrNotFound):
    // bucket 或 object 不存在
case errors.Is(err, s3.ErrAccessDenied):
    // 凭证或策略拒绝访问
case err != nil:
    // 其他 provider/网络错误
}
```

错误保留底层 AWS SDK error，可通过 `errors.As` 读取 `OperationError` 或 provider
返回的具体错误。模块不记录请求、凭证、预签名 URL，也不新增任何业务存储。
