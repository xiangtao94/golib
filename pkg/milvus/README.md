# Milvus adapter

独立 module：`github.com/xiangtao94/golib/pkg/milvus`。

所有操作接收 `context.Context`。`CreateCollection` 和 `CreateCollectionWithSchema` 是幂等但不盲目成功：

- 同名 collection 已存在时会 Describe 并验证名称、分片、schema flags、字段类型、主键/AutoID、nullable 和 type params（包括向量维度）。
- Has/Create 之间发生并发创建时，会重新 Describe；只有最终 schema 匹配才视为成功。

搜索必须显式描述 ANN 契约：

```go
results, err := client.SearchVectors(
    ctx,
    "documents",
    vectors,
    20,
    milvus.SearchOptions{
        ANNField:     "embedding",
        OutputFields: []string{"title"},
        MetricType:   entity.COSINE,
        ANNParam:     index.NewHNSWAnnParam(64),
        Filter:       `active == true`,
    },
)
```

不要依赖库内硬编码的 metric、vector 字段或 HNSW/IVF 参数。
