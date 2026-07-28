package http

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	defaultSseMaxBufSize = 100 * 1024 * 1024 // 500MB
)

// ClientConf 是 HTTP 客户端配置，包括基础 URL、重试策略等。
type ClientConf struct {
	Service          string                   `yaml:"service"`          // api服务名
	Domain           string                   `yaml:"domain"`           // api domain
	Domains          []string                 `yaml:"domains"`          // api domain
	Timeout          time.Duration            `yaml:"timeout"`          // 请求超时时间
	ConnectTimeout   time.Duration            `yaml:"connectTimeout"`   // 连接超时时间
	MaxReqBodyLen    int                      `yaml:"maxReqBodyLen"`    // 正数时记录有界请求正文；默认和负数不记录
	MaxRespBodyLen   int                      `yaml:"maxRespBodyLen"`   // 正数时记录有界响应正文；默认和负数不记录
	HttpStat         bool                     `yaml:"httpStat"`         // http 分析，默认关闭
	RetryTimes       int                      `yaml:"retryTimes"`       // 最大重试次数
	RetryWaitTime    time.Duration            `yaml:"retryWaitTime"`    // 重试等待间隔
	RetryMaxWaitTime time.Duration            `yaml:"retryMaxWaitTime"` // 最大重试等待
	Proxy            string                   `yaml:"proxy"`
	RetryPolicy      resty.RetryConditionFunc // 自定义重试条件

	Transport    http.RoundTripper  `json:"-"` // 可选的自定义 Transport
	LoadBalancer resty.LoadBalancer `json:"-"`

	HTTPClient *resty.Client `json:"-"`
	once       sync.Once
	initErr    error
}

func (c *ClientConf) selectBaseURL(ctx context.Context) (string, error) {
	if len(c.Domains) == 0 {
		return c.Domain, nil
	}
	if c.LoadBalancer != nil {
		return c.LoadBalancer.NextWithContext(ctx)
	}
	return c.HTTPClient.LoadBalancer().NextWithContext(ctx)
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
	Timeout      time.Duration       // 单次请求超时时间（若为零则使用客户端配置）
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

// initClient 初始化 resty.Client，仅执行一次
func (c *ClientConf) initClient() error {
	c.once.Do(func() {
		// 设置默认值
		if c.Timeout == 0 {
			c.Timeout = 5 * time.Second // 默认超时时间 5 秒
		}
		if c.RetryTimes == 0 {
			c.RetryTimes = 3 // 默认重试次数为 3 次
		}
		if c.RetryWaitTime == 0 {
			c.RetryWaitTime = 500 * time.Millisecond // 默认首次重试等待时间
		}
		if c.RetryMaxWaitTime == 0 {
			c.RetryMaxWaitTime = 2 * time.Second // 默认最大重试等待时间
		}
		if c.MaxReqBodyLen == 0 {
			c.MaxReqBodyLen = -1
		}
		if c.MaxRespBodyLen == 0 {
			c.MaxRespBodyLen = -1
		}

		client := resty.New()
		client.SetTimeout(c.Timeout)
		client.SetRetryCount(c.RetryTimes)
		client.SetRetryWaitTime(c.RetryWaitTime)
		client.SetRetryMaxWaitTime(c.RetryMaxWaitTime)

		// 优化连接池设置
		if c.Transport == nil {
			c.Transport = &http.Transport{
				MaxIdleConns:        100,              // 最大空闲连接数
				MaxIdleConnsPerHost: 10,               // 每个host的最大空闲连接数
				IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
				DisableKeepAlives:   false,            // 启用keep-alive
			}
		}
		client.SetTransport(c.Transport)

		if c.Proxy != "" {
			client.SetProxy(c.Proxy)
		}
		if len(c.Domains) > 0 {
			rr, err := resty.NewRoundRobin(c.Domains...)
			if err != nil {
				c.initErr = err
				return
			}
			client.SetLoadBalancer(rr)
		}
		client.SetLogger(GetHttpLogger().Sugar())
		c.HTTPClient = client
	})
	if c.initErr != nil {
		return fmt.Errorf("http client init error: %w", c.initErr)
	}
	return nil
}

func GetHttpLogger() *zap.Logger {
	return zlog.NewLoggerWithSkip(2)
}

// GET 方法
func (c *ClientConf) Get(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodGet, opts)
}

// GET 方法
func (c *ClientConf) GetStream(ctx context.Context, opts RequestOptions, f func(data []byte) error) (*Result, error) {
	return c.doStream(ctx, http.MethodGet, opts, f)
}

// Head 方法
func (c *ClientConf) Head(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodHead, opts)
}

// Patch 方法
func (c *ClientConf) Patch(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodPatch, opts)
}

// POST 方法
func (c *ClientConf) Post(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodPost, opts)
}

// POST 方法
func (c *ClientConf) PostStream(ctx context.Context, opts RequestOptions, f func(data []byte) error) (*Result, error) {
	return c.doStream(ctx, http.MethodPost, opts, f)
}

// PUT 方法
func (c *ClientConf) Put(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodPut, opts)
}

// DELETE 方法
func (c *ClientConf) Delete(ctx context.Context, opts RequestOptions) (*Result, error) {
	return c.do(ctx, http.MethodDelete, opts)
}

// do 执行通用请求方法
func (c *ClientConf) do(ctx context.Context, method string, opts RequestOptions) (res *Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, _ = zlog.EnsureRequestID(ctx)
	var timeoutCtx context.Context
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	} else {
		timeoutCtx = ctx
	}
	// 记录开始时间
	req, err := c.buildRequest(ctx, method, opts)
	if err != nil {
		return nil, err
	}
	req.WithContext(timeoutCtx)

	start := time.Now()
	defer func() { // 不能省略这个闭包函数， 否则req和err传入不进去
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

func (c *ClientConf) logHttpInvoke(ctx context.Context, req *resty.Request, res *Result, err error, start time.Time, opts RequestOptions) {
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
		zlog.String("service", c.Service),
		zlog.String("method", req.Method),
		zlog.String("requestUrl", req.URL),
		zlog.Int("attempts", req.Attempt),
		zlog.Int("status", status),
		zlog.String("request", truncateString(c.getReqBodyStr(opts), c.MaxReqBodyLen)),
		zlog.String("response", truncateString(respBodyStr, c.MaxRespBodyLen)),
		zlog.String("cost", fmt.Sprintf("%v%s", zlog.GetRequestCost(start, time.Now()), "ms")),
	}
	logger := zlog.LoggerWithContext(GetHttpLogger(), ctx)
	if err != nil {
		logger.Error(msg, fields...)
	} else {
		logger.Info(msg, fields...)
	}
}

func (c *ClientConf) doStream(ctx context.Context, method string, opts RequestOptions, f func(data []byte) error) (res *Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, _ = zlog.EnsureRequestID(ctx)
	var timeoutCtx context.Context
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	} else {
		timeoutCtx = ctx
	}
	req, err := c.buildRequest(ctx, method, opts)
	if err != nil {
		return nil, err
	}
	req.WithContext(timeoutCtx)
	start := time.Now()
	defer func() { // 不能省略这个闭包函数， 否则req和err传入不进去
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
func (c *ClientConf) doRequestSetBody(req *resty.Request, opts RequestOptions) error {
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

func (c *ClientConf) buildRequest(ctx context.Context, method string, opts RequestOptions) (*resty.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	err := c.initClient()
	if err != nil {
		return nil, err
	}
	// 构造完整 URL
	urlStr, err := c.selectBaseURL(ctx)
	if err != nil {
		return nil, err
	}
	urlStr = strings.TrimRight(urlStr, "/") + opts.Path
	req := c.HTTPClient.R() // 设置请求上下文
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
	return req, nil
}

func (c *ClientConf) getReqBodyStr(opts RequestOptions) string {
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
func (c *ClientConf) Close() {
	if c.HTTPClient != nil {
		// 如果使用了自定义Transport，需要关闭空闲连接
		if transport, ok := c.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}
