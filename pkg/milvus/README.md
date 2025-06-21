# Milvus 向量数据库客户端

这是一个增强的Milvus向量数据库客户端封装，集成了zlog日志记录，提供了完整的向量数据库操作功能。

## 🚀 功能特性

- ✅ **集合管理**: 创建、删除、描述集合
- ✅ **向量操作**: 插入、搜索、删除向量数据
- ✅ **索引管理**: 创建和管理各种类型的索引
- ✅ **内存管理**: 加载/释放集合到内存
- ✅ **数据查询**: 支持表达式查询和向量搜索
- ✅ **批量操作**: 高效的批量数据插入
- ✅ **统计信息**: 获取集合统计和状态信息
- ✅ **自定义Schema**: 支持复杂的数据结构
- ✅ **集成zlog日志**: 详细的操作日志和性能监控
- ✅ **错误处理**: 完善的错误处理和重试机制

## 📦 安装依赖

```bash
go get github.com/milvus-io/milvus-sdk-go/v2
```

## 🛠️ 配置

### 配置结构

```go
type MilvusConf struct {
    Host     string `yaml:"host"`     // Milvus服务地址
    Port     string `yaml:"port"`     // Milvus服务端口
    Username string `yaml:"username"` // 用户名（可选）
    Password string `yaml:"password"` // 密码（可选）
    Database string `yaml:"database"` // 数据库名（可选）
}
```

### YAML配置示例

```yaml
milvus:
  host: "localhost"
  port: "19530"
  username: ""  # 如果启用了认证
  password: ""  # 如果启用了认证
  database: ""  # 如果使用了多数据库
```

## 📖 使用方法

### 1. 创建客户端

```go
config := MilvusConf{
    Host:     "localhost",
    Port:     "19530",
    Username: "", // 可选
    Password: "", // 可选
    Database: "", // 可选
}

client, err := NewMilvusClient(config)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### 2. 集合管理

#### 创建简单集合

```go
// 创建128维向量集合
err := client.CreateCollection(ctx, "my_collection", 128, "我的向量集合")
if err != nil {
    log.Fatal(err)
}
```

#### 创建自定义Schema集合

```go
schema := &entity.Schema{
    CollectionName: "custom_collection",
    Description:    "带有自定义字段的集合",
    Fields: []*entity.Field{
        {
            Name:       "id",
            DataType:   entity.FieldTypeInt64,
            PrimaryKey: true,
            AutoID:     true,
        },
        {
            Name:     "title",
            DataType: entity.FieldTypeVarChar,
            TypeParams: map[string]string{
                "max_length": "200",
            },
        },
        {
            Name:     "vector",
            DataType: entity.FieldTypeFloatVector,
            TypeParams: map[string]string{
                "dim": "128",
            },
        },
    },
}

err := client.CreateCollectionWithSchema(ctx, schema, 2)
if err != nil {
    log.Fatal(err)
}
```

#### 删除集合

```go
err := client.DropCollection(ctx, "my_collection")
if err != nil {
    log.Fatal(err)
}
```

### 3. 索引管理

#### 创建默认索引（IVF_FLAT）

```go
err := client.CreateDefaultIndex(ctx, "my_collection")
if err != nil {
    log.Fatal(err)
}
```

#### 创建HNSW索引（推荐）

```go
err := client.CreateHNSWIndex(ctx, "my_collection", 16, 200)
if err != nil {
    log.Fatal(err)
}
```

#### 创建自定义索引

```go
params := map[string]string{
    "nlist": "1024",
}
err := client.CreateIndex(ctx, "my_collection", "vector", entity.IvfFlat, entity.L2, params)
if err != nil {
    log.Fatal(err)
}
```

### 4. 内存管理

#### 加载集合到内存

```go
err := client.LoadCollection(ctx, "my_collection", false)
if err != nil {
    log.Fatal(err)
}
```

#### 释放集合内存

```go
err := client.ReleaseCollection(ctx, "my_collection")
if err != nil {
    log.Fatal(err)
}
```

### 5. 数据操作

#### 插入向量数据

```go
// 生成示例向量数据
vectors := [][]float32{
    {0.1, 0.2, 0.3, 0.4}, // 4维向量示例
    {0.5, 0.6, 0.7, 0.8},
    {0.9, 1.0, 1.1, 1.2},
}

insertResult, err := client.InsertVectors(ctx, "my_collection", vectors)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Inserted %d vectors\n", insertResult.Len())
```

#### 插入带额外字段的数据

```go
vectors := [][]float32{{0.1, 0.2}, {0.3, 0.4}}
titleColumn := entity.NewColumnVarChar("title", []string{"标题1", "标题2"})
categoryColumn := entity.NewColumnInt32("category", []int32{1, 2})

_, err := client.InsertVectors(ctx, "my_collection", vectors, titleColumn, categoryColumn)
if err != nil {
    log.Fatal(err)
}
```

### 6. 向量搜索

#### 基础向量搜索

```go
queryVectors := [][]float32{
    {0.1, 0.2, 0.3, 0.4},
}

searchResults, err := client.SearchVectors(ctx, "my_collection", queryVectors, 10, []string{"id"})
if err != nil {
    log.Fatal(err)
}

// 处理搜索结果
for i, results := range searchResults {
    fmt.Printf("Query %d results:\n", i)
    for j, result := range results {
        fmt.Printf("  Rank %d - ID: %v, Score: %f\n", j, result.ID, result.Score)
    }
}
```

### 7. 数据查询

#### 根据表达式查询

```go
queryResult, err := client.Query(ctx, "my_collection", "id > 100", []string{"id", "title"})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Query returned %d results\n", len(queryResult))
```

### 8. 数据删除

#### 根据ID删除

```go
ids := []int64{1, 2, 3, 4, 5}
err := client.DeleteByIds(ctx, "my_collection", ids)
if err != nil {
    log.Fatal(err)
}
```

#### 根据表达式删除

```go
err := client.DeleteByExpr(ctx, "my_collection", "category == 1")
if err != nil {
    log.Fatal(err)
}
```

### 9. 统计信息

#### 获取集合统计

```go
stats, err := client.GetCollectionStatistics(ctx, "my_collection")
if err != nil {
    log.Fatal(err)
}

for key, value := range stats {
    fmt.Printf("%s: %s\n", key, value)
}
```

#### 获取加载进度

```go
progress, err := client.GetLoadingProgress(ctx, "my_collection")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Loading progress: %d%%\n", progress)
```

## 🌐 Web应用集成

### Gin框架向量搜索API

```go
func searchVectorsHandler(c *gin.Context) {
    var request struct {
        Vector []float32 `json:"vector"`
        TopK   int       `json:"topK"`
    }

    if err := c.ShouldBindJSON(&request); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }

    // 执行向量搜索
    searchResults, err := milvusClient.SearchVectors(c, "my_collection", 
        [][]float32{request.Vector}, request.TopK, []string{"id", "title"})
    if err != nil {
        c.JSON(500, gin.H{"error": "Search failed"})
        return
    }

    // 格式化返回结果
    var response []map[string]interface{}
    if len(searchResults) > 0 {
        for _, result := range searchResults[0] {
            response = append(response, map[string]interface{}{
                "id":     result.ID,
                "score":  result.Score,
                "fields": result.Fields,
            })
        }
    }

    c.JSON(200, gin.H{
        "results": response,
        "count":   len(response),
    })
}
```

### 批量数据导入API

```go
func batchInsertHandler(c *gin.Context) {
    var request struct {
        Vectors [][]float32 `json:"vectors"`
        Titles  []string    `json:"titles"`
    }

    if err := c.ShouldBindJSON(&request); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }

    // 准备额外字段
    titleColumn := entity.NewColumnVarChar("title", request.Titles)

    // 批量插入
    insertResult, err := milvusClient.InsertVectors(c, "my_collection", 
        request.Vectors, titleColumn)
    if err != nil {
        c.JSON(500, gin.H{"error": "Insert failed"})
        return
    }

    c.JSON(200, gin.H{
        "message": "Batch insert successful",
        "count":   insertResult.Len(),
    })
}
```

## 📊 日志记录

客户端自动记录所有操作的详细日志：

- 操作类型和参数
- 执行时间统计
- 数据量统计
- 错误信息和堆栈
- 请求ID追踪

日志示例：
```
2025-01-15 10:30:15.123 INFO collection my_collection created successfully, dimension: 128, cost: 125ms
2025-01-15 10:30:16.456 INFO inserted 100 vectors to collection my_collection, cost: 89ms
2025-01-15 10:30:17.789 INFO searched 5 query vectors in collection my_collection, topK: 10, cost: 23ms
```

## ⚡ 性能优化

### 1. 索引选择建议

```go
// 高精度搜索 - HNSW索引
client.CreateHNSWIndex(ctx, collectionName, 16, 200)

// 大规模数据 - IVF索引
params := map[string]string{"nlist": "4096"}
client.CreateIndex(ctx, collectionName, "vector", entity.IvfFlat, entity.L2, params)
```

### 2. 批量操作优化

```go
// 推荐批量大小：1000-10000
const batchSize = 5000

for i := 0; i < totalVectors; i += batchSize {
    end := i + batchSize
    if end > totalVectors {
        end = totalVectors
    }
    
    batch := vectors[i:end]
    _, err := client.InsertVectors(ctx, collectionName, batch)
    if err != nil {
        log.Fatal(err)
    }
}
```

### 3. 内存管理策略

```go
// 搜索前加载集合
client.LoadCollection(ctx, collectionName, false)

// 搜索完成后释放内存（如果不经常使用）
defer client.ReleaseCollection(ctx, collectionName)
```

## 🔧 错误处理

```go
searchResults, err := client.SearchVectors(ctx, collectionName, queryVectors, topK, outputFields)
if err != nil {
    // 日志已自动记录，处理业务逻辑
    return fmt.Errorf("vector search failed: %w", err)
}
```

## 🎯 最佳实践

1. **索引策略**: 根据数据规模和精度要求选择合适的索引类型
2. **批量操作**: 大量数据使用批量插入，提高性能
3. **内存管理**: 合理使用Load/Release管理内存占用
4. **分片策略**: 大集合使用适当的分片数量
5. **监控告警**: 监控搜索延迟和成功率

## 📋 支持的数据类型

- **向量类型**: FloatVector, BinaryVector
- **标量类型**: Int64, Int32, Int16, Int8, Bool, Float, Double
- **字符串类型**: VarChar
- **JSON类型**: JSON（Milvus 2.1+）

## 🆘 常见问题

### Q: 如何选择合适的索引类型？
A: 
- 小数据集(< 100万)：使用FLAT索引
- 中等数据集：使用IVF_FLAT或IVF_PQ
- 大数据集且要求高精度：使用HNSW
- 内存受限：使用IVF_PQ

### Q: 如何处理大规模数据插入？
A: 使用批量插入，建议每批1000-10000条，并在插入后调用Flush确保数据持久化。

### Q: 搜索结果不准确怎么办？
A: 检查索引参数设置，增大nprobe值或使用更精确的索引类型如HNSW。

### Q: 如何监控Milvus性能？
A: 使用GetCollectionStatistics获取统计信息，结合zlog日志监控操作耗时。

## 🔗 相关链接

- [Milvus官方文档](https://milvus.io/docs)
- [Milvus Go SDK](https://github.com/milvus-io/milvus-sdk-go)
- [向量索引选择指南](https://milvus.io/docs/index.md) 