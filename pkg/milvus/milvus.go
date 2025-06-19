// Package milvus -----------------------------
// @file      : milvus.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/1/15
// Description: Milvus向量数据库客户端封装
// -------------------------------------------
package milvus

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/xiangtao94/golib/pkg/zlog"
)

// MilvusConf Milvus配置
type MilvusConf struct {
	Host     string `yaml:"host"`     // Milvus服务地址
	Port     string `yaml:"port"`     // Milvus服务端口
	Username string `yaml:"username"` // 用户名（可选）
	Password string `yaml:"password"` // 密码（可选）
	Database string `yaml:"database"` // 数据库名（可选）
}

// MilvusClient Milvus客户端封装
type MilvusClient struct {
	client client.Client
	config MilvusConf
}

// SearchResult 搜索结果
type SearchResult struct {
	ID     interface{}            // 主键ID
	Score  float32                // 相似度分数
	Fields map[string]interface{} // 其他字段
}

// CollectionInfo 集合信息
type CollectionInfo struct {
	Name        string
	Description string
	ShardsNum   int32
	Schema      *entity.Schema
}

// IndexInfo 索引信息
type IndexInfo struct {
	FieldName  string
	IndexType  entity.IndexType
	MetricType entity.MetricType
	Params     map[string]string
}

// NewMilvusClient 创建Milvus客户端
func NewMilvusClient(config MilvusConf) (*MilvusClient, error) {
	connectParam := client.Config{
		Address: fmt.Sprintf("%s:%s", config.Host, config.Port),
	}

	if config.Username != "" {
		connectParam.Username = config.Username
		connectParam.Password = config.Password
	}

	if config.Database != "" {
		connectParam.DBName = config.Database
	}

	c, err := client.NewClient(context.Background(), connectParam)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus client: %w", err)
	}

	return &MilvusClient{
		client: c,
		config: config,
	}, nil
}

// CreateCollection 创建集合
func (mc *MilvusClient) CreateCollection(ctx *gin.Context, collectionName string, dimension int, description string) error {
	start := time.Now()

	// 检查集合是否已存在
	exists, err := mc.client.HasCollection(ctx, collectionName)
	if err != nil {
		zlog.Errorf(ctx, "failed to check collection exists %s: %v", collectionName, err)
		return fmt.Errorf("failed to check collection exists: %w", err)
	}

	if exists {
		zlog.Infof(ctx, "collection %s already exists", collectionName)
		return nil
	}

	// 创建schema
	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    description,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeInt64,
				PrimaryKey: true,
				AutoID:     true,
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", dimension),
				},
			},
		},
	}

	err = mc.client.CreateCollection(ctx, schema, 1)
	if err != nil {
		zlog.Errorf(ctx, "failed to create collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to create collection: %w", err)
	}

	zlog.Infof(ctx, "collection %s created successfully, dimension: %d, cost: %v",
		collectionName, dimension, time.Since(start))
	return nil
}

// CreateCollectionWithSchema 使用自定义schema创建集合
func (mc *MilvusClient) CreateCollectionWithSchema(ctx *gin.Context, schema *entity.Schema, shardsNum int32) error {
	start := time.Now()

	// 检查集合是否已存在
	exists, err := mc.client.HasCollection(ctx, schema.CollectionName)
	if err != nil {
		zlog.Errorf(ctx, "failed to check collection exists %s: %v", schema.CollectionName, err)
		return fmt.Errorf("failed to check collection exists: %w", err)
	}

	if exists {
		zlog.Infof(ctx, "collection %s already exists", schema.CollectionName)
		return nil
	}

	err = mc.client.CreateCollection(ctx, schema, shardsNum)
	if err != nil {
		zlog.Errorf(ctx, "failed to create collection %s: %v", schema.CollectionName, err)
		return fmt.Errorf("failed to create collection: %w", err)
	}

	zlog.Infof(ctx, "collection %s created successfully with custom schema, shards: %d, cost: %v",
		schema.CollectionName, shardsNum, time.Since(start))
	return nil
}

// DropCollection 删除集合
func (mc *MilvusClient) DropCollection(ctx *gin.Context, collectionName string) error {
	start := time.Now()

	err := mc.client.DropCollection(ctx, collectionName)
	if err != nil {
		zlog.Errorf(ctx, "failed to drop collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to drop collection: %w", err)
	}

	zlog.Infof(ctx, "collection %s dropped successfully, cost: %v", collectionName, time.Since(start))
	return nil
}

// InsertVectors 插入向量数据
func (mc *MilvusClient) InsertVectors(ctx *gin.Context, collectionName string, vectors [][]float32, extraFields ...entity.Column) (entity.Column, error) {
	start := time.Now()

	// 准备向量数据
	vectorColumn := entity.NewColumnFloatVector("vector", len(vectors[0]), vectors)
	columns := []entity.Column{vectorColumn}

	// 添加额外字段
	columns = append(columns, extraFields...)

	result, err := mc.client.Insert(ctx, collectionName, "", columns...)
	if err != nil {
		zlog.Errorf(ctx, "failed to insert vectors to collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to insert vectors: %w", err)
	}

	zlog.Infof(ctx, "inserted %d vectors to collection %s, cost: %v",
		len(vectors), collectionName, time.Since(start))
	return result, nil
}

// SearchVectors 向量搜索
func (mc *MilvusClient) SearchVectors(ctx *gin.Context, collectionName string, queryVectors [][]float32, topK int, outputFields []string) ([][]SearchResult, error) {
	start := time.Now()

	searchParam, err := entity.NewIndexIvfFlatSearchParam(1024)
	if err != nil {
		zlog.Errorf(ctx, "failed to create search param: %v", err)
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}
	vectors := make([]entity.Vector, len(queryVectors))
	for _, vector := range queryVectors {
		vectors = append(vectors, entity.FloatVector(vector))
	}
	searchResult, err := mc.client.Search(
		ctx,
		collectionName,
		nil, // 分区名，nil表示搜索所有分区
		"",  // 表达式过滤条件
		outputFields,
		vectors,
		"vector",
		entity.L2,
		topK,
		searchParam,
	)
	if err != nil {
		zlog.Errorf(ctx, "failed to search vectors in collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// 转换搜索结果
	results := make([][]SearchResult, len(queryVectors))
	for i, result := range searchResult {
		results[i] = make([]SearchResult, result.ResultCount)
		for j := 0; j < result.ResultCount; j++ {
			searchRes := SearchResult{
				Score:  result.Scores[j],
				Fields: make(map[string]interface{}),
			}

			// 获取ID
			if result.IDs != nil {
				id, _ := result.IDs.Get(j)
				searchRes.ID = id
			}

			// 获取其他字段
			for _, field := range result.Fields {
				if value, err := field.Get(j); err == nil {
					searchRes.Fields[field.Name()] = value
				}
			}

			results[i][j] = searchRes
		}
	}

	zlog.Infof(ctx, "searched %d query vectors in collection %s, topK: %d, cost: %v",
		len(queryVectors), collectionName, topK, time.Since(start))
	return results, nil
}

// CreateIndex 创建索引
func (mc *MilvusClient) CreateIndex(ctx *gin.Context, collectionName, fieldName string, indexType entity.IndexType, metricType entity.MetricType, params map[string]string) error {
	start := time.Now()

	idx, err := NewIndexByType(indexType, metricType, params)
	if err != nil {
		zlog.Errorf(ctx, "failed to create index config: %v", err)
		return fmt.Errorf("failed to create index config: %w", err)
	}

	err = mc.client.CreateIndex(ctx, collectionName, fieldName, idx, false)
	if err != nil {
		zlog.Errorf(ctx, "failed to create index on %s.%s: %v", collectionName, fieldName, err)
		return fmt.Errorf("failed to create index: %w", err)
	}

	zlog.Infof(ctx, "index created successfully on %s.%s, type: %s, metric: %s, cost: %v",
		collectionName, fieldName, indexType, metricType, time.Since(start))
	return nil
}

// LoadCollection 加载集合到内存
func (mc *MilvusClient) LoadCollection(ctx *gin.Context, collectionName string, async bool) error {
	start := time.Now()

	err := mc.client.LoadCollection(ctx, collectionName, async)
	if err != nil {
		zlog.Errorf(ctx, "failed to load collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to load collection: %w", err)
	}

	zlog.Infof(ctx, "collection %s loaded successfully, async: %v, cost: %v",
		collectionName, async, time.Since(start))
	return nil
}

// ReleaseCollection 释放集合从内存
func (mc *MilvusClient) ReleaseCollection(ctx *gin.Context, collectionName string) error {
	start := time.Now()

	err := mc.client.ReleaseCollection(ctx, collectionName)
	if err != nil {
		zlog.Errorf(ctx, "failed to release collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to release collection: %w", err)
	}

	zlog.Infof(ctx, "collection %s released successfully, cost: %v", collectionName, time.Since(start))
	return nil
}

// GetCollectionStatistics 获取集合统计信息
func (mc *MilvusClient) GetCollectionStatistics(ctx *gin.Context, collectionName string) (map[string]string, error) {
	start := time.Now()

	stats, err := mc.client.GetCollectionStatistics(ctx, collectionName)
	if err != nil {
		zlog.Errorf(ctx, "failed to get collection statistics %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to get collection statistics: %w", err)
	}

	zlog.Infof(ctx, "got collection statistics for %s, cost: %v", collectionName, time.Since(start))
	return stats, nil
}

// DeleteByIds 根据ID删除数据
func (mc *MilvusClient) DeleteByIds(ctx *gin.Context, collectionName string, ids []int64) error {
	start := time.Now()

	// 构建删除表达式
	expr := fmt.Sprintf("id in %v", ids)

	err := mc.client.Delete(ctx, collectionName, "", expr)
	if err != nil {
		zlog.Errorf(ctx, "failed to delete data from collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to delete data: %w", err)
	}

	zlog.Infof(ctx, "deleted %d records from collection %s, cost: %v",
		len(ids), collectionName, time.Since(start))
	return nil
}

// DeleteByExpr 根据表达式删除数据
func (mc *MilvusClient) DeleteByExpr(ctx *gin.Context, collectionName string, expr string) error {
	start := time.Now()

	err := mc.client.Delete(ctx, collectionName, "", expr)
	if err != nil {
		zlog.Errorf(ctx, "failed to delete data from collection %s with expr %s: %v", collectionName, expr, err)
		return fmt.Errorf("failed to delete data: %w", err)
	}

	zlog.Infof(ctx, "deleted data from collection %s with expr: %s, cost: %v",
		collectionName, expr, time.Since(start))
	return nil
}

// ListCollections 列出所有集合
func (mc *MilvusClient) ListCollections(ctx *gin.Context) ([]*entity.Collection, error) {
	start := time.Now()

	collections, err := mc.client.ListCollections(ctx)
	if err != nil {
		zlog.Errorf(ctx, "failed to list collections: %v", err)
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	zlog.Infof(ctx, "listed %d collections, cost: %v", len(collections), time.Since(start))
	return collections, nil
}

// DescribeCollection 获取集合详细信息
func (mc *MilvusClient) DescribeCollection(ctx *gin.Context, collectionName string) (*entity.Collection, error) {
	start := time.Now()

	collection, err := mc.client.DescribeCollection(ctx, collectionName)
	if err != nil {
		zlog.Errorf(ctx, "failed to describe collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to describe collection: %w", err)
	}

	zlog.Infof(ctx, "described collection %s, cost: %v", collectionName, time.Since(start))
	return collection, nil
}

// Flush 强制刷新集合数据到持久化存储
func (mc *MilvusClient) Flush(ctx *gin.Context, collectionName string) error {
	start := time.Now()

	err := mc.client.Flush(ctx, collectionName, false)
	if err != nil {
		zlog.Errorf(ctx, "failed to flush collections %v: %v", collectionName, err)
		return fmt.Errorf("failed to flush collections: %w", err)
	}

	zlog.Infof(ctx, "flushed collections %v successfully, cost: %v", collectionName, time.Since(start))
	return nil
}

// GetLoadingProgress 获取集合加载进度
func (mc *MilvusClient) GetLoadingProgress(ctx *gin.Context, collectionName string) (int64, error) {
	start := time.Now()

	progress, err := mc.client.GetLoadingProgress(ctx, collectionName, nil)
	if err != nil {
		zlog.Errorf(ctx, "failed to get loading progress for collection %s: %v", collectionName, err)
		return 0, fmt.Errorf("failed to get loading progress: %w", err)
	}

	zlog.Infof(ctx, "got loading progress for collection %s: %d%%, cost: %v",
		collectionName, progress, time.Since(start))
	return progress, nil
}

// Query 查询数据
func (mc *MilvusClient) Query(ctx *gin.Context, collectionName string, expr string, outputFields []string) ([]entity.Column, error) {
	start := time.Now()

	result, err := mc.client.Query(ctx, collectionName, nil, expr, outputFields)
	if err != nil {
		zlog.Errorf(ctx, "failed to query data from collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to query data: %w", err)
	}

	zlog.Infof(ctx, "queried data from collection %s with expr: %s, result count: %d, cost: %v",
		collectionName, expr, len(result), time.Since(start))
	return result, nil
}

// Close 关闭客户端连接
func (mc *MilvusClient) Close() error {
	if mc.client != nil {
		return mc.client.Close()
	}
	return nil
}

// CreateDefaultIndex 创建默认的IVF_FLAT索引
func (mc *MilvusClient) CreateDefaultIndex(ctx *gin.Context, collectionName string) error {
	params := map[string]string{
		"nlist": "1024",
	}
	return mc.CreateIndex(ctx, collectionName, "vector", entity.IvfFlat, entity.L2, params)
}

// CreateHNSWIndex 创建HNSW索引（推荐用于高精度搜索）
func (mc *MilvusClient) CreateHNSWIndex(ctx *gin.Context, collectionName string, M int, efConstruction int) error {
	params := map[string]string{
		"M":              fmt.Sprintf("%d", M),
		"efConstruction": fmt.Sprintf("%d", efConstruction),
	}
	return mc.CreateIndex(ctx, collectionName, "vector", entity.HNSW, entity.L2, params)
}

// NewIndexByType 根据索引类型创建索引对象
func NewIndexByType(indexType entity.IndexType, metricType entity.MetricType, params map[string]string) (entity.Index, error) {
	// 添加度量类型到参数中
	if params == nil {
		params = make(map[string]string)
	}
	params["metric_type"] = string(metricType)

	// 使用通用索引创建方法，适用于所有索引类型
	idx := entity.NewGenericIndex("", indexType, params)
	return idx, nil
}
