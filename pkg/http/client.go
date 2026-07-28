package http

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"resty.dev/v3"

	"github.com/xiangtao94/golib/pkg/zlog"
)

const (
	EncodeJson           = "_json"
	EncodeForm           = "_form"
	EncodeRaw            = "_raw"
	EncodeRawByte        = "_raw_byte"
	EncodeFile           = "_file"
	defaultSseMaxBufSize = 100 * 1024 * 1024 // 100 MiB
)

var ErrNilContext = errors.New("http: nil context")
var ErrNilStreamHandler = errors.New("http: nil stream handler")
var ErrIdempotencyKeyRequired = errors.New("http: idempotency key required for non-idempotent retry")

const HeaderIdempotencyKey = "Idempotency-Key"

type RetryCondition func(response *http.Response, err error) bool

type TransportMiddleware func(next http.RoundTripper) http.RoundTripper

type EndpointSelector interface {
	Next(context.Context) (string, error)
}

type ClientConfig struct {
	Service          string         `yaml:"service"`          // api服务名
	Domain           string         `yaml:"domain"`           // api domain
	Domains          []string       `yaml:"domains"`          // api domain
	Timeout          time.Duration  `yaml:"timeout"`          // 单次尝试的超时时间
	ConnectTimeout   time.Duration  `yaml:"connectTimeout"`   // 连接超时时间
	MaxReqBodyLen    int            `yaml:"maxReqBodyLen"`    // 正数时记录有界请求正文；默认和负数不记录
	MaxRespBodyLen   int            `yaml:"maxRespBodyLen"`   // 正数时记录有界响应正文；默认和负数不记录
	TraceEnabled     bool           `yaml:"traceEnabled"`     // 记录 DNS/连接/TLS/首字节等 trace
	RetryCount       int            `yaml:"retryCount"`       // 最大重试次数；0 表示不重试
	RetryWaitTime    time.Duration  `yaml:"retryWaitTime"`    // 重试等待间隔
	RetryMaxWaitTime time.Duration  `yaml:"retryMaxWaitTime"` // 最大重试等待
	Proxy            string         `yaml:"proxy"`
	RetryCondition   RetryCondition // 自定义重试条件

	Transport           http.RoundTripper     `json:"-"` // 可选的自定义 Transport
	TransportMiddleware []TransportMiddleware `json:"-"`
	EndpointSelector    EndpointSelector      `json:"-"`
}

type Client struct {
	config    ClientConfig
	client    *resty.Client
	selector  EndpointSelector
	transport http.RoundTripper
	closeOnce sync.Once
	closeErr  error
}

func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:          5 * time.Second,
		ConnectTimeout:   5 * time.Second,
		RetryCount:       0,
		RetryWaitTime:    500 * time.Millisecond,
		RetryMaxWaitTime: 2 * time.Second,
	}
}

func NewClient(config ClientConfig) (*Client, error) {
	if config.RetryCount < 0 {
		return nil, errors.New("http: retry count cannot be negative")
	}
	client := resty.New()
	if config.Timeout > 0 {
		client.SetTimeout(config.Timeout)
	}
	client.SetRetryCount(config.RetryCount)
	if config.RetryWaitTime > 0 {
		client.SetRetryWaitTime(config.RetryWaitTime)
	}
	if config.RetryMaxWaitTime > 0 {
		client.SetRetryMaxWaitTime(config.RetryMaxWaitTime)
	}
	client.SetTrace(config.TraceEnabled)

	transport := config.Transport
	if transport == nil {
		transport = defaultTransport(config.ConnectTimeout)
	}
	wrappedTransport := transport
	for index := len(config.TransportMiddleware) - 1; index >= 0; index-- {
		middleware := config.TransportMiddleware[index]
		if middleware == nil {
			continue
		}
		wrappedTransport = middleware(wrappedTransport)
		if wrappedTransport == nil {
			return nil, fmt.Errorf("http: transport middleware %d returned nil", index)
		}
	}
	client.SetTransport(wrappedTransport)

	if config.Proxy != "" {
		client.SetProxy(config.Proxy)
	}
	selector := config.EndpointSelector
	if selector == nil && len(config.Domains) > 0 {
		loadBalancer, err := resty.NewRoundRobin(config.Domains...)
		if err != nil {
			return nil, fmt.Errorf("http client init error: %w", err)
		}
		selector = restyEndpointSelector{loadBalancer: loadBalancer}
	}
	if config.RetryCondition != nil {
		client.AddRetryConditions(func(response *resty.Response, err error) bool {
			if response == nil {
				return config.RetryCondition(nil, err)
			}
			return config.RetryCondition(response.RawResponse, err)
		})
	}
	client.SetLogger(GetHttpLogger().Sugar())
	return &Client{
		config:    config,
		client:    client,
		selector:  selector,
		transport: transport,
	}, nil
}

type restyEndpointSelector struct {
	loadBalancer resty.LoadBalancer
}

func (selector restyEndpointSelector) Next(ctx context.Context) (string, error) {
	return selector.loadBalancer.NextWithContext(ctx)
}

func defaultTransport(connectTimeout time.Duration) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second
	if connectTimeout > 0 {
		dialer := &net.Dialer{
			Timeout:   connectTimeout,
			KeepAlive: 30 * time.Second,
		}
		transport.DialContext = dialer.DialContext
	}
	return transport
}

func (c *Client) selectBaseURL(ctx context.Context) (string, error) {
	if c.selector != nil {
		return c.selector.Next(ctx)
	}
	return c.config.Domain, nil
}

type RetryMode uint8

const (
	RetryNone RetryMode = iota
	RetrySafeMethods
	RetryWithIdempotencyKey
)

type RetryPolicy struct {
	Mode        RetryMode
	MaxRetries  int
	MinWaitTime time.Duration
	MaxWaitTime time.Duration
}

// RequestOptions 是单个请求可选参数
type RequestOptions struct {
	Path         string              // 请求路径（相对于 BaseURL）
	Encode       string              // EncodeJson EncodeForm EncodeRaw EncodeRawByte EncodeFile
	RequestBody  any                 // body 数据
	RequestFiles map[string][]string // EncodeFile 模式下的表单数据 key是表单字段名，value是多个本地文件路径
	QueryParams  map[string]string   // 查询参数
	Headers      map[string]string   // 自定义请求头
	Cookies      map[string]string   // 自定义 Cookie (键值对)
	Budget       time.Duration       // 包含所有尝试和退避等待的总调用预算
	Retry        *RetryPolicy        // nil 时继承客户端的安全方法重试配置
}

type Result struct {
	HttpCode int
	Response []byte
	Header   http.Header
	Ctx      context.Context
}

// truncateString 截断超长字符串，避免日志过长
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return "(omitted)"
	}
	if len(s) > maxLen {
		return s[:maxLen] + "...(truncated)"
	}
	return s
}

func GetHttpLogger() *zap.Logger {
	return zlog.NewLoggerWithSkip(2)
}

// GET 方法
func (c *Client) Get(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodGet, opts)
}

// GET 方法
func (c *Client) GetStream(ctx context.Context, opts RequestOptions, f func(data []byte) error) (*Result, error) {
	return c.doStream(ctx, http.MethodGet, opts, f)
}

// Head 方法
func (c *Client) Head(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodHead, opts)
}

// Patch 方法
func (c *Client) Patch(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodPatch, opts)
}

// POST 方法
func (c *Client) Post(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodPost, opts)
}

// POST 方法
func (c *Client) PostStream(ctx context.Context, opts RequestOptions, f func(data []byte) error) (*Result, error) {
	return c.doStream(ctx, http.MethodPost, opts, f)
}

// PUT 方法
func (c *Client) Put(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodPut, opts)
}

// DELETE 方法
func (c *Client) Delete(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodDelete, opts)
}

func (c *Client) prepareRequest(
	ctx context.Context,
	method string,
	opts RequestOptions,
) (context.Context, *resty.Request, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, nil, ErrNilContext
	}
	ctx, _ = zlog.EnsureRequestID(ctx)
	requestContext := ctx
	cancel := func() {}
	if opts.Budget > 0 {
		requestContext, cancel = context.WithTimeout(ctx, opts.Budget)
	}
	req, err := c.buildRequest(ctx, method, opts)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	req = req.WithContext(requestContext)
	return ctx, req, cancel, nil
}

// do 执行通用请求方法
func (c *Client) do(ctx context.Context, method string, opts RequestOptions) (res *Result, err error) {
	ctx, req, cancel, err := c.prepareRequest(ctx, method, opts)
	if err != nil {
		return nil, err
	}
	defer cancel()

	start := time.Now()
	defer func() {
		c.logHttpInvoke(ctx, req, res, err, start, opts)
	}()
	// 执行请求
	resp, err := req.Send()
	if err != nil {
		return nil, err
	}
	res = &Result{
		Ctx: ctx,
	}
	if resp != nil {
		res.HttpCode = resp.StatusCode()
		res.Response = resp.Bytes()
		res.Header = resp.Header()
	}
	return res, nil
}

func (c *Client) logHttpInvoke(ctx context.Context, req *resty.Request, res *Result, err error, start time.Time, opts RequestOptions) {
	msg := "http invoke"
	if err != nil {
		msg = err.Error()
	}
	var status int
	var respBodyStr string
	if res != nil {
		status = res.HttpCode
		respBodyStr = string(res.Response)
	}
	fields := []zap.Field{
		zlog.String("service", c.config.Service),
		zlog.String("method", req.Method),
		zlog.String("requestUrl", req.URL),
		zlog.Int("attempts", req.Attempt),
		zlog.Int("status", status),
		zlog.String("request", truncateString(c.getReqBodyStr(opts), c.config.MaxReqBodyLen)),
		zlog.String("response", truncateString(respBodyStr, c.config.MaxRespBodyLen)),
		zlog.String("cost", fmt.Sprintf("%v%s", zlog.GetRequestCost(start, time.Now()), "ms")),
	}
	if c.config.TraceEnabled {
		fields = append(fields, zlog.String("trace", req.TraceInfo().String()))
	}
	logger := zlog.LoggerWithContext(GetHttpLogger(), ctx)
	if err != nil {
		logger.Error(msg, fields...)
	} else {
		logger.Info(msg, fields...)
	}
}

func (c *Client) doStream(ctx context.Context, method string, opts RequestOptions, f func(data []byte) error) (res *Result, err error) {
	if f == nil {
		return nil, ErrNilStreamHandler
	}
	ctx, req, cancel, err := c.prepareRequest(ctx, method, opts)
	if err != nil {
		return nil, err
	}
	defer cancel()
	start := time.Now()
	defer func() {
		c.logHttpInvoke(ctx, req, res, err, start, opts)
	}()
	// 通过自定义执行方式以获取 response.RawBody()
	resp, err := req.SetResponseDoNotParse(true).Send()
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		return nil, errors.New("http stream returned an empty response")
	}
	defer resp.Body.Close()
	if resp.IsStatusFailure() {
		return nil, fmt.Errorf("http response code %v, error: %s", resp.StatusCode(), resp.String())
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, slices.Min([]int{4096, defaultSseMaxBufSize})), defaultSseMaxBufSize)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		// 业务自行打印结果
		err = f(scanner.Bytes())
		if err != nil {
			return nil, err
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	res = &Result{
		Ctx:      ctx,
		HttpCode: resp.StatusCode(),
	}
	return
}
func (c *Client) doRequestSetBody(req *resty.Request, opts RequestOptions) error {
	// 处理请求体
	switch strings.ToLower(opts.Encode) {
	case EncodeJson:
		req.SetBody(opts.RequestBody)
	case EncodeRaw:
		req.SetBody(opts.RequestBody)
	case EncodeRawByte:
		req.SetBody(opts.RequestBody)
	case EncodeForm:
		if opts.RequestBody != nil {
			values, err := getFormRequestData(opts.RequestBody)
			if err != nil {
				return fmt.Errorf("failed to marshal form body: %v", err)
			}
			req.SetFormDataFromValues(values)
		}
	case EncodeFile:
		// 支持文件上传，FormData和Files可以同时存在实现multipart/form-data
		// opts.FormData: map[string]string，普通字段
		// opts.RequestFiles: map[string][]string，key=表单字段名，value=本地文件路径
		if opts.RequestBody != nil {
			values, err := getFormRequestData(opts.RequestBody)
			if err != nil {
				return fmt.Errorf("failed to marshal form body: %v", err)
			}
			req.SetFormDataFromValues(values)
		}
		for field, paths := range opts.RequestFiles {
			for _, path := range paths {
				req.SetFile(field, path)
			}
		}
	default:
		req.SetBody(opts.RequestBody)
	}
	return nil
}

func (c *Client) buildRequest(ctx context.Context, method string, opts RequestOptions) (*resty.Request, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	// 构造完整 URL
	urlStr, err := c.selectBaseURL(ctx)
	if err != nil {
		return nil, err
	}
	urlStr = strings.TrimRight(urlStr, "/") + opts.Path
	req := c.client.R() // 设置请求上下文
	req.URL = urlStr
	req.Method = method
	// 处理查询参数
	if len(opts.QueryParams) > 0 {
		req.SetQueryParams(opts.QueryParams)
	}
	// 处理 Headers
	for k, v := range opts.Headers {
		req.SetHeader(k, v)
	}
	req.Header.Set(zlog.HeaderRequestID, zlog.GetRequestID(ctx))
	// 处理 Cookies
	for name, val := range opts.Cookies {
		cookie := &http.Cookie{Name: name, Value: val}
		req.SetCookie(cookie)
	}
	err = c.doRequestSetBody(req, opts)
	if err != nil {
		return nil, err
	}
	if err := applyRetryPolicy(req, opts.Retry); err != nil {
		return nil, err
	}
	return req, nil
}

func applyRetryPolicy(request *resty.Request, policy *RetryPolicy) error {
	if policy == nil {
		return nil
	}
	if policy.MaxRetries < 0 {
		return errors.New("http: retry count cannot be negative")
	}
	if policy.MinWaitTime < 0 || policy.MaxWaitTime < 0 {
		return errors.New("http: retry wait cannot be negative")
	}

	switch policy.Mode {
	case RetryNone:
		request.SetRetryCount(0)
		request.SetRetryAllowNonIdempotent(false)
	case RetrySafeMethods:
		request.SetRetryCount(policy.MaxRetries)
		request.SetRetryAllowNonIdempotent(false)
	case RetryWithIdempotencyKey:
		if strings.TrimSpace(request.Header.Get(HeaderIdempotencyKey)) == "" {
			return ErrIdempotencyKeyRequired
		}
		request.SetRetryCount(policy.MaxRetries)
		request.SetRetryAllowNonIdempotent(true)
	default:
		return fmt.Errorf("http: unsupported retry mode %d", policy.Mode)
	}
	if policy.MinWaitTime > 0 {
		request.SetRetryWaitTime(policy.MinWaitTime)
	}
	if policy.MaxWaitTime > 0 {
		request.SetRetryMaxWaitTime(policy.MaxWaitTime)
	}
	return nil
}

func (c *Client) getReqBodyStr(opts RequestOptions) string {
	// 处理请求体
	var reqBodyStr string
	switch strings.ToLower(opts.Encode) {
	case EncodeJson:
		if opts.RequestBody != nil {
			// 记录请求体内容（JSON 序列化）
			b, _ := json.Marshal(opts.RequestBody)
			reqBodyStr = string(b)
		}
	case EncodeForm:
		if opts.RequestBody != nil {
			values, _ := getFormRequestData(opts.RequestBody)
			reqBodyStr = values.Encode()
		}
	case EncodeRaw:
		if bodyStr, ok := opts.RequestBody.(string); ok {
			reqBodyStr = bodyStr
		} else if b, ok2 := opts.RequestBody.([]byte); ok2 {
			reqBodyStr = string(b)
		}
	case EncodeRawByte:
		if b, ok := opts.RequestBody.([]byte); ok {
			reqBodyStr = string(b)
		}
	case EncodeFile:
		// 无法完整记录请求体字符串，只能简单提示
		reqBodyStr = "[multipart form data with files]"
	default:
		if opts.RequestBody != nil {
			// 记录请求体内容（JSON 序列化）
			b, _ := json.Marshal(opts.RequestBody)
			reqBodyStr = string(b)
		}
	}
	return reqBodyStr
}

func getFormRequestData(requestBody any) (url.Values, error) {
	v := url.Values{}

	if data, ok := requestBody.(map[string]string); ok {
		for key, value := range data {
			v.Add(key, value)
		}
		return v, nil
	}

	if data, ok := requestBody.(map[string]interface{}); ok {
		for key, value := range data {
			var vStr string
			switch value.(type) {
			case string:
				vStr = value.(string)
			default:
				if tmp, err := json.Marshal(value); err != nil {
					return nil, err
				} else {
					vStr = string(tmp)
				}
			}

			v.Add(key, vStr)
		}
		return v, nil
	}

	return nil, errors.New("unSupport RequestBody type")
}

// Close 关闭HTTP客户端并释放连接池资源
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		if transport, ok := c.transport.(interface{ CloseIdleConnections() }); ok {
			transport.CloseIdleConnections()
		}
		c.closeErr = c.client.Close()
	})
	return c.closeErr
}
