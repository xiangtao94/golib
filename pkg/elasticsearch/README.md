# Elasticsearch 客户端

基于 Elasticsearch 官方 Go 客户端 `go-elasticsearch/v8` 封装的高级客户端，提供类型安全的 API 接口和完整的日志记录功能。

## 功能特性

- ✅ **类型安全**: 基于 TypedClient 提供完整的类型安全支持
- ✅ **索引管理**: 支持索引的创建、删除、存在性检查
- ✅ **文档操作**: 支持批量插入、删除文档
- ✅ **搜索查询**: 支持混合查询、KNN 搜索等
- ✅ **日志记录**: 集成 zlog 提供详细的请求/响应日志
- ✅ **配置灵活**: 支持用户名密码、CA 证书认证

## 快速开始

### 1. 配置结构体

```go
type ElasticConf struct {
    Addr           string `yaml:"addr"`           // Elasticsearch 地址
    Username       string `yaml:"username"`       // 用户名
    Password       string `yaml:"password"`       // 密码
    CaCertPath     string `yaml:"caCertPath"`     // CA 证书路径
    MaxReqBodyLen  int    `yaml:"maxReqBodyLen"`  // 请求体最大展示长度
    MaxRespBodyLen int    `yaml:"maxRespBodyLen"` // 响应体最大展示长度
}
```

### 2. 初始化客户端

```go
package main

import (
    "github.com/xiangtao94/golib/pkg/elasticsearch"
)

func main() {
    conf := elasticsearch.ElasticConf{
        Addr:           "https://localhost:9200",
        Username:       "elastic",
        Password:       "your-password",
        CaCertPath:     "/path/to/ca.crt", // 可选
        MaxReqBodyLen:  1024,
        MaxRespBodyLen: 10240,
    }
    
    client, err := elasticsearch.InitESClient(conf)
    if err != nil {
        log.Fatal(err)
    }
}
```

## 使用示例

### 索引管理

```go
// 检查索引是否存在
exists, err := client.CheckIndex(ctx, "my-index")
if err != nil {
    log.Fatal(err)
}

// 创建索引
mapping := &types.TypeMapping{
    Properties: map[string]types.Property{
        "title": types.TextProperty{
            Type: "text",
        },
        "content": types.TextProperty{
            Type: "text",
        },
    },
}

setting := &types.IndexSettings{
    NumberOfShards:   1,
    NumberOfReplicas: 0,
}

err = client.CreateIndex(ctx, "my-index", mapping, setting)
if err != nil {
    log.Fatal(err)
}

// 删除索引
err = client.DeleteIndex(ctx, "my-index")
if err != nil {
    log.Fatal(err)
}
```

### 文档操作

```go
// 批量插入文档
docs := []any{
    map[string]interface{}{
        "title":   "文档1",
        "content": "这是第一个文档的内容",
    },
    map[string]interface{}{
        "title":   "文档2", 
        "content": "这是第二个文档的内容",
    },
}

err = client.DocumentInsert(ctx, "my-index", docs)
if err != nil {
    log.Fatal(err)
}

// 批量删除文档
query := &types.Query{
    Term: map[string]types.TermQuery{
        "title.keyword": {Value: "文档1"},
    },
}

err = client.DocumentDelete(ctx, "my-index", query)
if err != nil {
    log.Fatal(err)
}
```

### 搜索查询

```go
// 构建搜索请求
searchReq := &search.Request{
    Query: &types.Query{
        Match: map[string]types.MatchQuery{
            "content": {Query: "文档"},
        },
    },
    Size: func() *int { i := 10; return &i }(),
    From: func() *int { i := 0; return &i }(),
}

// 执行搜索
resp, err := client.Search(ctx, "my-index", searchReq)
if err != nil {
    log.Fatal(err)
}

// 处理搜索结果
for _, hit := range resp.Hits.Hits {
    fmt.Printf("文档ID: %s, 分数: %f\n", hit.Id_, *hit.Score_)
    // 处理 hit.Source_ 中的文档内容
}
```

## 日志配置

客户端会自动记录所有请求和响应的详细信息，可以通过环境变量控制日志输出长度：

```bash
export ES_LOG_MAX_REQ_LEN=1024    # 请求体的最大展示长度
export ES_LOG_MAX_RESP_LEN=10240  # 响应体的最大展示长度
```

- 设置为 0 使用默认长度
- 设置为 -1 不打印对应内容

## 注意事项

- 批量插入限制为 3000 条文档
- 所有操作都会自动生成唯一的文档ID（基于时间戳和UUID的SHA256哈希）
- 客户端会自动处理超时检测和错误处理
- 支持 Gin 框架的上下文传递，自动记录请求ID 