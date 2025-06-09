基于minio和https://github.com/pierrre/imageserver的图片服务器，支持图片各种操作

```markdown
gimg/
├── server.go               // ImageServer主入口
├── handler.go              // Image处理逻辑封装
├── cache.go                // 缓存封装与KeyGenerator实现
├── storage.go              // MinIO存储与loadImage实现
├── config.go               // 配置结构定义
├── http.go                 // http服务
└── utils.go                // 工具函数（如sanitizeImageKey）

```