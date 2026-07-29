package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/google/uuid"

	"github.com/xiangtao94/golib/pkg/zlog"
)

type ElasticConf struct {
	Addr          string `yaml:"addr"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	CaCertPath    string `yaml:"caCertPath"`
	MaxReqBodyLen int    `yaml:"maxReqBodyLen"`
	// response body 最大长度展示，0表示采用默认的10240，-1表示不打印。指定长度的时候需注意，返回的json可能被截断
	MaxRespBodyLen int `yaml:"maxRespBodyLen"`
}

type ElasticsearchClient struct {
	Client       *elasticsearch.TypedClient
	bulkMaxDocs  int
	bulkMaxBytes int
}

const (
	defaultBulkMaxDocs  = 3_000
	defaultBulkMaxBytes = 8 << 20
)

func InitESClient(conf ElasticConf) (*ElasticsearchClient, error) {
	endpointURL, err := url.Parse(conf.Addr)
	if err != nil {
		return nil, err
	}
	options := []elasticsearch.Option{
		elasticsearch.WithAddresses(endpointURL.String()),
		elasticsearch.WithLogger(newLogger(conf.MaxReqBodyLen, conf.MaxRespBodyLen)),
	}
	if conf.Username != "" || conf.Password != "" {
		options = append(options, elasticsearch.WithBasicAuth(conf.Username, conf.Password))
	}
	if len(conf.CaCertPath) > 0 {
		f, err := os.Open(conf.CaCertPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		options = append(options, elasticsearch.WithCACert(data))
	}
	typeClient, err := elasticsearch.NewTyped(options...)
	if err != nil {
		return nil, err
	}
	return &ElasticsearchClient{
		Client:       typeClient,
		bulkMaxDocs:  defaultBulkMaxDocs,
		bulkMaxBytes: defaultBulkMaxBytes,
	}, nil
}

// CheckIndex reports whether indexName exists.
func (ec *ElasticsearchClient) CheckIndex(ctx context.Context, indexName string) (bool, error) {
	return ec.Client.Indices.Exists(indexName).Do(ctx)
}

// CreateIndex 根据提供的 mapping 创建索引
func (ec *ElasticsearchClient) CreateIndex(ctx context.Context, indexName string, mapping *types.TypeMapping, setting *types.IndexSettings) error {
	res, err := ec.Client.Indices.Create(indexName).Mappings(mapping).Settings(setting).Do(ctx)
	if err != nil {
		return err
	}
	if !res.Acknowledged {
		return fmt.Errorf("failed to create index: %s", indexName)
	}
	return nil
}

// 删除索引
func (ec *ElasticsearchClient) DeleteIndex(ctx context.Context, indexName string) error {

	res, err := ec.Client.Indices.Delete(indexName).IgnoreUnavailable(true).Do(ctx)
	if err != nil {
		return err
	}
	if !res.Acknowledged {
		return fmt.Errorf("failed to delete index: %s", indexName)
	}
	return nil
}

// DocumentInsert writes documents in bounded bulk requests.
func (ec *ElasticsearchClient) DocumentInsert(ctx context.Context, indexName string, docs []any) error {
	maxDocs := ec.bulkMaxDocs
	if maxDocs <= 0 {
		maxDocs = defaultBulkMaxDocs
	}
	maxBytes := ec.bulkMaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultBulkMaxBytes
	}

	type pendingDocument struct {
		id  string
		doc json.RawMessage
	}
	batch := make([]pendingDocument, 0, min(len(docs), maxDocs))
	batchBytes := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		bulk := ec.Client.Bulk().Index(indexName)
		for _, pending := range batch {
			if err := bulk.CreateOp(
				types.CreateOperation{Index_: &indexName, Id_: &pending.id},
				pending.doc,
			); err != nil {
				return err
			}
		}
		response, err := bulk.Do(ctx)
		if err != nil {
			return err
		}
		if response.Errors {
			return fmt.Errorf("elasticsearch bulk request contains failed operations")
		}
		batch = batch[:0]
		batchBytes = 0
		return nil
	}

	for _, doc := range docs {
		id := uuid.NewString()
		encoded, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		operation, err := json.Marshal(types.CreateOperation{
			Index_: &indexName,
			Id_:    &id,
		})
		if err != nil {
			return err
		}
		estimatedBytes := len(`{"create":`) + len(operation) + len("}\n") +
			len(encoded) + len("\n")
		if estimatedBytes > maxBytes {
			return fmt.Errorf(
				"elasticsearch document size %d exceeds bulk byte limit %d",
				estimatedBytes,
				maxBytes,
			)
		}
		if len(batch) > 0 &&
			(len(batch) >= maxDocs || batchBytes+estimatedBytes > maxBytes) {
			if err := flush(); err != nil {
				return err
			}
		}
		batch = append(batch, pendingDocument{id: id, doc: encoded})
		batchBytes += estimatedBytes
	}
	return flush()
}

// DocumentDelete deletes every document matching query.
func (ec *ElasticsearchClient) DocumentDelete(
	ctx context.Context,
	indexName string,
	query *types.Query,
) error {
	_, err := ec.Client.DeleteByQuery(indexName).Query(query).Do(ctx)
	return err
}

// Search 混合查询
func (ec *ElasticsearchClient) Search(ctx context.Context, indexName string, query *search.Request) (*search.Response, error) {
	res, err := ec.Client.Search().Index(indexName).Request(query).Do(ctx)
	if err != nil {
		return nil, err
	}
	if res.TimedOut {
		return nil, fmt.Errorf("knn search time out")
	}
	return res, nil
}

type elasticLogger struct {
	requestLimit        int
	responseLimit       int
	requestBodyEnabled  bool
	responseBodyEnabled bool
}

func (e *elasticLogger) LogRoundTrip(request *http.Request, response *http.Response, err error, start time.Time, duration time.Duration) error {
	if request == nil {
		return nil
	}
	if err != nil {
		if !zlog.ErrorEnabled(request.Context()) {
			return nil
		}
	} else if !zlog.InfoEnabled(request.Context()) {
		return nil
	}

	fields := []zlog.Field{}

	// 只在有值时添加字段
	fields = append(fields, zlog.String("method", request.Method))

	if request.URL != nil {
		fields = append(fields, zlog.String("path", request.URL.Path))

	}

	var reqBody, respBody []byte
	limits := logLimits{
		request:  e.requestLimit,
		response: e.responseLimit,
	}
	if request.Body != nil && limits.request > 0 {
		reqBody, request.Body = captureBodyPrefix(request.Body, limits.request)
	}
	if response != nil && response.Body != nil && limits.response > 0 {
		respBody, response.Body = captureBodyPrefix(response.Body, limits.response)
	}

	requestData, respData := formatLogMsg(limits, reqBody, respBody)

	// requestParam 只在有内容时添加
	if len(requestData) > 0 {
		fields = append(fields, zlog.String("requestParam", string(requestData)))
	}

	if response != nil {
		fields = append(fields, zlog.Int("responseStatus", response.StatusCode))
	}

	// response 只在有内容时添加
	if len(respData) > 0 {
		fields = append(fields, zlog.String("response", string(respData)))
	}

	// 添加时间相关字段
	fields = append(fields, zlog.String("cost", fmt.Sprintf("%d%s", duration.Milliseconds(), "ms")))

	msg := "success"
	if err != nil {
		msg = err.Error()
		zlog.ErrorLogger(request.Context(), msg, fields...)
		return nil
	}
	zlog.InfoLogger(request.Context(), msg, fields...)
	return nil
}

func (e *elasticLogger) RequestBodyEnabled() bool {
	return e.requestBodyEnabled
}

func (e *elasticLogger) ResponseBodyEnabled() bool {
	return e.responseBodyEnabled
}

func newLogger(requestLimit, responseLimit int) *elasticLogger {
	requestLimit = normalizeLogLimit(requestLimit)
	responseLimit = normalizeLogLimit(responseLimit)
	return &elasticLogger{
		requestLimit:        requestLimit,
		responseLimit:       responseLimit,
		requestBodyEnabled:  requestLimit > 0,
		responseBodyEnabled: responseLimit > 0,
	}
}

type logLimits struct {
	request  int
	response int
}

func formatLogMsg(limits logLimits, requestParam, responseData []byte) (req, resp []byte) {
	maxReqBodyLen := limits.request
	maxRespBodyLen := limits.response

	if maxReqBodyLen != -1 {
		req = requestParam
		if len(requestParam) > maxReqBodyLen {
			req = req[:maxReqBodyLen]
		}
	}

	if maxRespBodyLen != -1 {
		resp = responseData
		if len(responseData) > maxRespBodyLen {
			resp = resp[:maxRespBodyLen]
		}
	}
	return req, resp
}

func captureBodyPrefix(body io.ReadCloser, limit int) ([]byte, io.ReadCloser) {
	prefix, _ := io.ReadAll(io.LimitReader(body, int64(limit)))
	restored := &readCloser{
		Reader: io.MultiReader(bytes.NewReader(prefix), body),
		Closer: body,
	}
	return prefix, restored
}

type readCloser struct {
	io.Reader
	io.Closer
}

func normalizeLogLimit(limit int) int {
	if limit <= 0 {
		return -1
	}
	return limit
}
