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

需要完整控制时使用 `CreateIndex` 显式传入字段、metric 和参数；`CreateDefaultIndex`
与 `CreateHNSWIndex` 是有意提供的默认策略快捷入口。`MilvusClient` 作为 Facade
统一构造 SDK options、补充操作上下文错误，并保留 `Driver` 作为高级逃生口。
创建连接时优先使用 `NewMilvusClientContext(ctx, config)`；连接始终受
`connectTimeout` 约束，未配置时默认为 10 秒。旧的 `NewMilvusClient(config)`
保留兼容并使用相同的默认超时。
关闭连接时传入调用方拥有的 context：

```go
err := client.Close(ctx)
```
