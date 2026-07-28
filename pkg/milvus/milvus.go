// Package milvus -----------------------------
// @file      : milvus.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/1/15
// Description: Milvus向量数据库客户端封装
// -------------------------------------------
package milvus

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	milvusindex "github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
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
	client *milvusclient.Client
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
	IndexType  milvusindex.IndexType
	MetricType entity.MetricType
	Params     map[string]string
}

// NewMilvusClient 创建Milvus客户端
func NewMilvusClient(config MilvusConf) (*MilvusClient, error) {
	connectParam := &milvusclient.ClientConfig{
		Address:  fmt.Sprintf("%s:%s", config.Host, config.Port),
		Username: config.Username,
		Password: config.Password,
		DBName:   config.Database,
	}

	c, err := milvusclient.New(context.Background(), connectParam)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus client: %w", err)
	}

	return &MilvusClient{
		client: c,
		config: config,
	}, nil
}

func validateVectors(vectors [][]float32) (int, error) {
	if len(vectors) == 0 {
		return 0, fmt.Errorf("vectors must not be empty")
	}
	dimension := len(vectors[0])
	if dimension == 0 {
		return 0, fmt.Errorf("vector dimension must be greater than zero")
	}
	for i := 1; i < len(vectors); i++ {
		if len(vectors[i]) != dimension {
			return 0, fmt.Errorf("vector %d has dimension %d, want %d", i, len(vectors[i]), dimension)
		}
	}
	return dimension, nil
}

// CreateCollection 创建集合
func (mc *MilvusClient) CreateCollection(ctx context.Context, collectionName string, dimension int, description string) error {
	start := time.Now()
	if dimension <= 0 {
		return fmt.Errorf("collection dimension must be greater than zero")
	}

	schema := entity.NewSchema().
		WithName(collectionName).
		WithDescription(description).
		WithField(entity.NewField().
			WithName("id").
			WithDataType(entity.FieldTypeInt64).
			WithIsPrimaryKey(true).
			WithIsAutoID(true)).
		WithField(entity.NewField().
			WithName("vector").
			WithDataType(entity.FieldTypeFloatVector).
			WithDim(int64(dimension)))

	if err := mc.ensureCollection(ctx, schema, 1); err != nil {
		return err
	}

	zlog.Infof(ctx, "collection %s created successfully, dimension: %d, cost: %v",
		collectionName, dimension, time.Since(start))
	return nil
}

// CreateCollectionWithSchema 使用自定义schema创建集合
func (mc *MilvusClient) CreateCollectionWithSchema(ctx context.Context, schema *entity.Schema, shardsNum int32) error {
	start := time.Now()
	if schema == nil {
		return fmt.Errorf("collection schema must not be nil")
	}

	if err := mc.ensureCollection(ctx, schema, shardsNum); err != nil {
		return err
	}

	zlog.Infof(ctx, "collection %s created successfully with custom schema, shards: %d, cost: %v",
		schema.CollectionName, shardsNum, time.Since(start))
	return nil
}

func (mc *MilvusClient) ensureCollection(ctx context.Context, expected *entity.Schema, shards int32) error {
	exists, err := mc.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(expected.CollectionName))
	if err != nil {
		return fmt.Errorf("check collection %q existence: %w", expected.CollectionName, err)
	}
	if exists {
		collection, describeErr := mc.client.DescribeCollection(
			ctx,
			milvusclient.NewDescribeCollectionOption(expected.CollectionName),
		)
		if describeErr != nil {
			return fmt.Errorf("describe existing collection %q: %w", expected.CollectionName, describeErr)
		}
		return validateCollection(collection, expected, shards)
	}

	createErr := mc.client.CreateCollection(
		ctx,
		milvusclient.NewCreateCollectionOption(expected.CollectionName, expected).WithShardNum(shards),
	)
	if createErr == nil {
		return nil
	}

	// Another creator may win between HasCollection and CreateCollection.
	// Re-describe and accept the race only when the resulting schema matches.
	collection, describeErr := mc.client.DescribeCollection(
		ctx,
		milvusclient.NewDescribeCollectionOption(expected.CollectionName),
	)
	if describeErr != nil {
		return fmt.Errorf("create collection %q: %w", expected.CollectionName, createErr)
	}
	if validationErr := validateCollection(collection, expected, shards); validationErr != nil {
		return errors.Join(
			fmt.Errorf("create collection %q: %w", expected.CollectionName, createErr),
			validationErr,
		)
	}
	return nil
}

func validateCollection(actual *entity.Collection, expected *entity.Schema, shards int32) error {
	if actual == nil || actual.Schema == nil {
		return errors.New("milvus: existing collection has no schema")
	}
	if actual.Name != expected.CollectionName || actual.Schema.CollectionName != expected.CollectionName {
		return fmt.Errorf("milvus: collection name mismatch: got %q, want %q", actual.Name, expected.CollectionName)
	}
	if shards > 0 && actual.ShardNum != shards {
		return fmt.Errorf("milvus: collection %q shard count is %d, want %d", expected.CollectionName, actual.ShardNum, shards)
	}
	if actual.Schema.AutoID != expected.AutoID ||
		actual.Schema.EnableDynamicField != expected.EnableDynamicField {
		return fmt.Errorf("milvus: collection %q schema flags do not match", expected.CollectionName)
	}

	actualFields := make(map[string]*entity.Field, len(actual.Schema.Fields))
	for _, field := range actual.Schema.Fields {
		actualFields[field.Name] = field
	}
	if len(actualFields) != len(expected.Fields) {
		return fmt.Errorf("milvus: collection %q has %d fields, want %d", expected.CollectionName, len(actualFields), len(expected.Fields))
	}
	for _, expectedField := range expected.Fields {
		actualField, ok := actualFields[expectedField.Name]
		if !ok {
			return fmt.Errorf("milvus: collection %q is missing field %q", expected.CollectionName, expectedField.Name)
		}
		if actualField.DataType != expectedField.DataType ||
			actualField.PrimaryKey != expectedField.PrimaryKey ||
			actualField.AutoID != expectedField.AutoID ||
			actualField.ElementType != expectedField.ElementType ||
			actualField.Nullable != expectedField.Nullable ||
			!reflect.DeepEqual(actualField.TypeParams, expectedField.TypeParams) {
			return fmt.Errorf("milvus: collection %q field %q does not match expected schema", expected.CollectionName, expectedField.Name)
		}
	}
	return nil
}

// DropCollection 删除集合
func (mc *MilvusClient) DropCollection(ctx context.Context, collectionName string) error {
	start := time.Now()

	err := mc.client.DropCollection(ctx, milvusclient.NewDropCollectionOption(collectionName))
	if err != nil {
		zlog.Errorf(ctx, "failed to drop collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to drop collection: %w", err)
	}

	zlog.Infof(ctx, "collection %s dropped successfully, cost: %v", collectionName, time.Since(start))
	return nil
}

// InsertVectors 插入向量数据
func (mc *MilvusClient) InsertVectors(ctx context.Context, collectionName string, vectors [][]float32, extraFields ...column.Column) (column.Column, error) {
	start := time.Now()

	dimension, err := validateVectors(vectors)
	if err != nil {
		return nil, err
	}
	vectorColumn := column.NewColumnFloatVector("vector", dimension, vectors)
	columns := make([]column.Column, 0, 1+len(extraFields))
	columns = append(columns, vectorColumn)

	// 添加额外字段
	columns = append(columns, extraFields...)

	result, err := mc.client.Insert(
		ctx,
		milvusclient.NewColumnBasedInsertOption(collectionName, columns...),
	)
	if err != nil {
		zlog.Errorf(ctx, "failed to insert vectors to collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to insert vectors: %w", err)
	}

	zlog.Infof(ctx, "inserted %d vectors to collection %s, cost: %v",
		len(vectors), collectionName, time.Since(start))
	return result.IDs, nil
}

type SearchOptions struct {
	ANNField     string
	OutputFields []string
	MetricType   entity.MetricType
	ANNParam     milvusindex.AnnParam
	Filter       string
}

// SearchVectors 向量搜索
func (mc *MilvusClient) SearchVectors(ctx context.Context, collectionName string, queryVectors [][]float32, topK int, options SearchOptions) ([][]SearchResult, error) {
	start := time.Now()

	if topK <= 0 {
		return nil, fmt.Errorf("topK must be greater than zero")
	}
	if _, err := validateVectors(queryVectors); err != nil {
		return nil, err
	}
	vectors := make([]entity.Vector, len(queryVectors))
	for i, vector := range queryVectors {
		vectors[i] = entity.FloatVector(vector)
	}
	option := buildSearchOption(collectionName, topK, vectors, options)
	searchResult, err := mc.client.Search(ctx, option)
	if err != nil {
		zlog.Errorf(ctx, "failed to search vectors in collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// 转换搜索结果
	results := make([][]SearchResult, len(searchResult))
	for i, result := range searchResult {
		if result.Err != nil {
			return nil, fmt.Errorf("search query %d failed: %w", i, result.Err)
		}
		if len(result.Scores) < result.ResultCount {
			return nil, fmt.Errorf("search query %d returned %d scores for %d results", i, len(result.Scores), result.ResultCount)
		}
		results[i] = make([]SearchResult, result.ResultCount)
		for j := 0; j < result.ResultCount; j++ {
			searchRes := SearchResult{
				Score:  result.Scores[j],
				Fields: make(map[string]interface{}),
			}

			// 获取ID
			if result.IDs != nil {
				id, err := result.IDs.Get(j)
				if err != nil {
					return nil, fmt.Errorf("read search query %d result %d id: %w", i, j, err)
				}
				searchRes.ID = id
			}

			// 获取其他字段
			for _, field := range result.Fields {
				value, err := field.Get(j)
				if err != nil {
					return nil, fmt.Errorf("read search query %d result %d field %q: %w", i, j, field.Name(), err)
				}
				searchRes.Fields[field.Name()] = value
			}

			results[i][j] = searchRes
		}
	}

	zlog.Infof(ctx, "searched %d query vectors in collection %s, topK: %d, cost: %v",
		len(queryVectors), collectionName, topK, time.Since(start))
	return results, nil
}

func buildSearchOption(collectionName string, topK int, vectors []entity.Vector, options SearchOptions) milvusclient.SearchOption {
	if options.ANNField == "" {
		options.ANNField = "vector"
	}
	if options.MetricType == "" {
		options.MetricType = entity.L2
	}
	option := milvusclient.NewSearchOption(collectionName, topK, vectors).
		WithANNSField(options.ANNField).
		WithOutputFields(options.OutputFields...).
		WithSearchParam(milvusindex.MetricTypeKey, string(options.MetricType))
	if options.ANNParam != nil {
		option.WithAnnParam(options.ANNParam)
	}
	if options.Filter != "" {
		option.WithFilter(options.Filter)
	}
	return option
}

// CreateIndex 创建索引
func (mc *MilvusClient) CreateIndex(ctx context.Context, collectionName, fieldName string, indexType milvusindex.IndexType, metricType entity.MetricType, params map[string]string) error {
	start := time.Now()

	idx, err := NewIndexByType(indexType, metricType, params)
	if err != nil {
		zlog.Errorf(ctx, "failed to create index config: %v", err)
		return fmt.Errorf("failed to create index config: %w", err)
	}

	task, err := mc.client.CreateIndex(
		ctx,
		milvusclient.NewCreateIndexOption(collectionName, fieldName, idx),
	)
	if err != nil {
		zlog.Errorf(ctx, "failed to create index on %s.%s: %v", collectionName, fieldName, err)
		return fmt.Errorf("failed to create index: %w", err)
	}
	if err := task.Await(ctx); err != nil {
		zlog.Errorf(ctx, "failed waiting for index on %s.%s: %v", collectionName, fieldName, err)
		return fmt.Errorf("failed waiting for index: %w", err)
	}

	zlog.Infof(ctx, "index created successfully on %s.%s, type: %s, metric: %s, cost: %v",
		collectionName, fieldName, indexType, metricType, time.Since(start))
	return nil
}

// LoadCollection 加载集合到内存
func (mc *MilvusClient) LoadCollection(ctx context.Context, collectionName string, async bool) error {
	start := time.Now()

	task, err := mc.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collectionName))
	if err != nil {
		zlog.Errorf(ctx, "failed to load collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to load collection: %w", err)
	}
	if !async {
		if err := task.Await(ctx); err != nil {
			zlog.Errorf(ctx, "failed waiting for collection %s to load: %v", collectionName, err)
			return fmt.Errorf("failed waiting for collection to load: %w", err)
		}
	}

	zlog.Infof(ctx, "collection %s loaded successfully, async: %v, cost: %v",
		collectionName, async, time.Since(start))
	return nil
}

// ReleaseCollection 释放集合从内存
func (mc *MilvusClient) ReleaseCollection(ctx context.Context, collectionName string) error {
	start := time.Now()

	err := mc.client.ReleaseCollection(ctx, milvusclient.NewReleaseCollectionOption(collectionName))
	if err != nil {
		zlog.Errorf(ctx, "failed to release collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to release collection: %w", err)
	}

	zlog.Infof(ctx, "collection %s released successfully, cost: %v", collectionName, time.Since(start))
	return nil
}

// GetCollectionStatistics 获取集合统计信息
func (mc *MilvusClient) GetCollectionStatistics(ctx context.Context, collectionName string) (map[string]string, error) {
	start := time.Now()

	stats, err := mc.client.GetCollectionStats(ctx, milvusclient.NewGetCollectionStatsOption(collectionName))
	if err != nil {
		zlog.Errorf(ctx, "failed to get collection statistics %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to get collection statistics: %w", err)
	}

	zlog.Infof(ctx, "got collection statistics for %s, cost: %v", collectionName, time.Since(start))
	return stats, nil
}

// DeleteByIds 根据ID删除数据
func (mc *MilvusClient) DeleteByIds(ctx context.Context, collectionName string, ids []int64) error {
	start := time.Now()

	if len(ids) == 0 {
		return nil
	}

	_, err := mc.client.Delete(
		ctx,
		milvusclient.NewDeleteOption(collectionName).WithInt64IDs("id", ids),
	)
	if err != nil {
		zlog.Errorf(ctx, "failed to delete data from collection %s: %v", collectionName, err)
		return fmt.Errorf("failed to delete data: %w", err)
	}

	zlog.Infof(ctx, "deleted %d records from collection %s, cost: %v",
		len(ids), collectionName, time.Since(start))
	return nil
}

// DeleteByExpr 根据表达式删除数据
func (mc *MilvusClient) DeleteByExpr(ctx context.Context, collectionName string, expr string) error {
	start := time.Now()

	_, err := mc.client.Delete(
		ctx,
		milvusclient.NewDeleteOption(collectionName).WithExpr(expr),
	)
	if err != nil {
		zlog.Errorf(ctx, "failed to delete data from collection %s with expr %s: %v", collectionName, expr, err)
		return fmt.Errorf("failed to delete data: %w", err)
	}

	zlog.Infof(ctx, "deleted data from collection %s with expr: %s, cost: %v",
		collectionName, expr, time.Since(start))
	return nil
}

// ListCollections 列出所有集合
func (mc *MilvusClient) ListCollections(ctx context.Context) ([]string, error) {
	start := time.Now()

	collections, err := mc.client.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		zlog.Errorf(ctx, "failed to list collections: %v", err)
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	zlog.Infof(ctx, "listed %d collections, cost: %v", len(collections), time.Since(start))
	return collections, nil
}

// DescribeCollection 获取集合详细信息
func (mc *MilvusClient) DescribeCollection(ctx context.Context, collectionName string) (*entity.Collection, error) {
	start := time.Now()

	collection, err := mc.client.DescribeCollection(ctx, milvusclient.NewDescribeCollectionOption(collectionName))
	if err != nil {
		zlog.Errorf(ctx, "failed to describe collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to describe collection: %w", err)
	}

	zlog.Infof(ctx, "described collection %s, cost: %v", collectionName, time.Since(start))
	return collection, nil
}

// Flush 强制刷新集合数据到持久化存储
func (mc *MilvusClient) Flush(ctx context.Context, collectionName string) error {
	start := time.Now()

	task, err := mc.client.Flush(ctx, milvusclient.NewFlushOption(collectionName))
	if err != nil {
		zlog.Errorf(ctx, "failed to flush collections %v: %v", collectionName, err)
		return fmt.Errorf("failed to flush collections: %w", err)
	}
	if err := task.Await(ctx); err != nil {
		zlog.Errorf(ctx, "failed waiting to flush collection %v: %v", collectionName, err)
		return fmt.Errorf("failed waiting to flush collection: %w", err)
	}

	zlog.Infof(ctx, "flushed collections %v successfully, cost: %v", collectionName, time.Since(start))
	return nil
}

// GetLoadingProgress 获取集合加载进度
func (mc *MilvusClient) GetLoadingProgress(ctx context.Context, collectionName string) (int64, error) {
	start := time.Now()

	state, err := mc.client.GetLoadState(ctx, milvusclient.NewGetLoadStateOption(collectionName))
	if err != nil {
		zlog.Errorf(ctx, "failed to get loading progress for collection %s: %v", collectionName, err)
		return 0, fmt.Errorf("failed to get loading progress: %w", err)
	}
	progress := state.Progress
	if state.State == entity.LoadStateLoaded {
		progress = 100
	}

	zlog.Infof(ctx, "got loading progress for collection %s: %d%%, cost: %v",
		collectionName, progress, time.Since(start))
	return progress, nil
}

// Query 查询数据
func (mc *MilvusClient) Query(ctx context.Context, collectionName string, expr string, outputFields []string) ([]column.Column, error) {
	start := time.Now()

	result, err := mc.client.Query(
		ctx,
		milvusclient.NewQueryOption(collectionName).
			WithFilter(expr).
			WithOutputFields(outputFields...),
	)
	if err != nil {
		zlog.Errorf(ctx, "failed to query data from collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to query data: %w", err)
	}

	zlog.Infof(ctx, "queried data from collection %s with expr: %s, result count: %d, cost: %v",
		collectionName, expr, result.ResultCount, time.Since(start))
	return []column.Column(result.Fields), nil
}

// Close 关闭客户端连接
func (mc *MilvusClient) Close() error {
	if mc.client != nil {
		return mc.client.Close(context.Background())
	}
	return nil
}

// CreateDefaultIndex 创建默认的IVF_FLAT索引
func (mc *MilvusClient) CreateDefaultIndex(ctx context.Context, collectionName string) error {
	params := map[string]string{
		"nlist": "1024",
	}
	return mc.CreateIndex(ctx, collectionName, "vector", milvusindex.IvfFlat, entity.L2, params)
}

// CreateHNSWIndex 创建HNSW索引（推荐用于高精度搜索）
func (mc *MilvusClient) CreateHNSWIndex(ctx context.Context, collectionName string, M int, efConstruction int) error {
	params := map[string]string{
		"M":              fmt.Sprintf("%d", M),
		"efConstruction": fmt.Sprintf("%d", efConstruction),
	}
	return mc.CreateIndex(ctx, collectionName, "vector", milvusindex.HNSW, entity.L2, params)
}

// NewIndexByType 根据索引类型创建索引对象
func NewIndexByType(indexType milvusindex.IndexType, metricType entity.MetricType, params map[string]string) (milvusindex.Index, error) {
	complete := make(map[string]string, len(params)+2)
	for key, value := range params {
		complete[key] = value
	}
	complete[milvusindex.IndexTypeKey] = string(indexType)
	complete[milvusindex.MetricTypeKey] = string(metricType)

	idx := milvusindex.NewGenericIndex("", complete)
	return idx, nil
}
