package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
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
	Client *elasticsearch.TypedClient
}

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
		Client: typeClient,
	}, nil
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

// BulkInsert 批量插入数据，批量限制为 3000 条
func (ec *ElasticsearchClient) DocumentInsert(ctx context.Context, indexName string, docs []any) (err error) {
	bulk := ec.Client.Bulk().Index(indexName)
	for _, doc := range docs {
		// 获取当前时间戳（秒级）
		timestamp := time.Now().UnixMicro()
		id := uuid.NewString()
		// 将时间戳与文档内容连接
		combined := fmt.Sprintf("%s%d", id, timestamp)
		// 生成SHA256哈希
		hash := sha256.Sum256([]byte(combined))
		// Base64编码哈希值
		uniqueID := base64.StdEncoding.EncodeToString(hash[:])
		err = bulk.CreateOp(types.CreateOperation{Index_: &indexName, Id_: &uniqueID}, doc)
		if err != nil {
			return err
		}
	}
	resp, err := bulk.Do(ctx)
	if err != nil {
		return err
	}
	if resp.Errors {
		return fmt.Errorf("elastic search error: %v", resp.Errors)
	}
	return nil
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
