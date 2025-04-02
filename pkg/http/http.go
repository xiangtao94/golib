package http

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/xiangtao94/golib/pkg/utils"
	"github.com/xiangtao94/golib/pkg/zlog"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	_defaultPrintRequestLen  = 10240
	_defaultPrintResponseLen = 10240
)

type HttpClientConf struct {
	Service         string        `yaml:"service"`
	Domain          string        `yaml:"domain"`
	Timeout         time.Duration `yaml:"timeout"`
	ConnectTimeout  time.Duration `yaml:"connectTimeout"`
	Retry           int           `yaml:"retry"`
	HttpStat        bool          `yaml:"httpStat"`
	Host            string        `yaml:"host"`
	Proxy           string        `yaml:"proxy"`
	MaxIdleConns    int           `yaml:"maxIdleConns"`
	IdleConnTimeout time.Duration `yaml:"idleConnTimeout"`
	// request body 最大长度展示，0表示采用默认的10240，-1表示不打印
	MaxReqBodyLen int `yaml:"maxReqBodyLen"`
	// response body 最大长度展示，0表示采用默认的10240，-1表示不打印。指定长度的时候需注意，返回的json可能被截断
	MaxRespBodyLen int `yaml:"maxRespBodyLen"`
	// 配置中设置了该值后当 err!=nil || httpCode >= retryHttpCode 时会重试（该策略优先级最低）
	RetryHttpCode int `yaml:"retryHttpCode"`
	// 重试策略，可不指定，默认使用`defaultRetryPolicy`(只有在`api.yaml`中指定retry>0 时生效)
	retryPolicy RetryPolicy  `json:"-"`
	HTTPClient  *http.Client `json:"-"`
	clientInit  sync.Once    `json:"-"`
	BasicAuth   struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"basicAuth"`
}

func (client *HttpClientConf) SetRetryPolicy(retry RetryPolicy) {
	client.retryPolicy = retry
}

func (client *HttpClientConf) GetTransPort() *http.Transport {
	connectTimeout := 10 * time.Second
	if client.ConnectTimeout != 0 {
		connectTimeout = client.ConnectTimeout
	}
	maxIdleConns := 100
	if client.MaxIdleConns != 0 {
		maxIdleConns = client.MaxIdleConns
	}
	idleConnTimeout := 300 * time.Second
	if client.IdleConnTimeout != 0 {
		idleConnTimeout = client.IdleConnTimeout
	}
	trans := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConns,
		IdleConnTimeout:     idleConnTimeout,
	}
	if client.Proxy != "" {
		trans.Proxy = func(_ *http.Request) (*url.URL, error) {
			return url.Parse(client.Proxy)
		}
	}
	return trans
}

func (client *HttpClientConf) makeRequest(ctx *gin.Context, method, url string, data io.Reader, opts HttpRequestOptions) (*http.Request, error) {
	req, err := http.NewRequest(method, url, data)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", opts.GetContentType())
	req.Header.Set("Request-Id", zlog.GetRequestID(ctx))
	if opts.Headers != nil {
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}
	}
	if client.Host != "" {
		req.Host = client.Host
	} else if h := req.Header.Get("host"); h != "" {
		req.Host = h
	}
	for k, v := range opts.Cookies {
		req.AddCookie(&http.Cookie{
			Name:  k,
			Value: v,
		})
	}
	if client.BasicAuth.Username != "" {
		req.SetBasicAuth(client.BasicAuth.Username, client.BasicAuth.Password)
	}
	return req, nil
}

func (client *HttpClientConf) HttpGet(ctx *gin.Context, path string, opts HttpRequestOptions) (*HttpResult, error) {
	// http request
	urlData, err := opts.getData()
	if err != nil {
		zlog.WarnLogger(ctx, "http client make data error: "+err.Error())
		return nil, err
	}

	var u string
	if urlData == nil {
		u = fmt.Sprintf("%s%s", client.Domain, path)
	} else {
		u = fmt.Sprintf("%s%s?%s", client.Domain, path, urlData)
	}
	req, err := client.makeRequest(ctx, "GET", u, nil, opts)
	if err != nil {
		zlog.WarnLogger(ctx, "http client makeRequest error: "+err.Error())
		return nil, err
	}
	body, err := client.httpDo(ctx, req, &opts, urlData)
	return &body, err
}

func (client *HttpClientConf) HttpDelete(ctx *gin.Context, path string, opts HttpRequestOptions) (*HttpResult, error) {
	// http request
	urlData, err := opts.getData()
	if err != nil {
		zlog.WarnLogger(ctx, "http client make data error: "+err.Error())
		return nil, err
	}
	var u string
	if urlData == nil {
		u = fmt.Sprintf("%s%s", client.Domain, path)
	} else {
		u = fmt.Sprintf("%s%s?%s", client.Domain, path, urlData)
	}
	req, err := client.makeRequest(ctx, "DELETE", u, nil, opts)
	if err != nil {
		zlog.WarnLogger(ctx, "http client makeRequest error: "+err.Error())
		return nil, err
	}
	body, err := client.httpDo(ctx, req, &opts, urlData)
	return &body, err
}

func (client *HttpClientConf) HttpPut(ctx *gin.Context, path string, opts HttpRequestOptions) (*HttpResult, error) {
	// http request
	urlData, err := opts.getData()
	if err != nil {
		zlog.WarnLogger(ctx, "http client make data error: "+err.Error())
		return nil, err
	}
	// 创建request
	req, err := client.makeRequest(ctx, "PUT", fmt.Sprintf("%s%s", client.Domain, path), bytes.NewReader(urlData), opts)
	if err != nil {
		zlog.WarnLogger(ctx, "http client makeRequest error: "+err.Error())
		return nil, err
	}
	body, err := client.httpDo(ctx, req, &opts, urlData)
	return &body, err
}

func (client *HttpClientConf) HttpPost(ctx *gin.Context, path string, opts HttpRequestOptions) (*HttpResult, error) {
	// http request
	urlData, err := opts.getData()
	if err != nil {
		zlog.WarnLogger(ctx, "http client make data error: "+err.Error())
		return nil, err
	}
	// 创建request
	req, err := client.makeRequest(ctx, "POST", fmt.Sprintf("%s%s", client.Domain, path), bytes.NewReader(urlData), opts)
	if err != nil {
		zlog.WarnLogger(ctx, "http client makeRequest error: "+err.Error())
		return nil, err
	}

	body, err := client.httpDo(ctx, req, &opts, urlData)
	if err != nil {
		zlog.Errorf(ctx, "ApiPostWithOpts failed, path:%s, err:%v", path, err.Error())
		return nil, err
	}
	return &body, err
}

func (client *HttpClientConf) HttpPostStream(ctx *gin.Context, path string, opts HttpRequestOptions, f func(data string) error) (err error) {
	// http request
	urlData, err := opts.getData()
	if err != nil {
		zlog.WarnLogger(ctx, "http client make data error: "+err.Error())
		return err
	}
	// 创建request
	req, err := client.makeRequest(ctx, "POST", fmt.Sprintf("%s%s", client.Domain, path), bytes.NewReader(urlData), opts)
	if err != nil {
		zlog.WarnLogger(ctx, "http client makeRequest error: "+err.Error())
		return err
	}
	err = client.DoStream(ctx, req, &opts, urlData, f)
	if err != nil {
		zlog.Errorf(ctx, "ApiPostWithOpts failed, path:%s, err:%v", path, err.Error())
		return err
	}
	return err
}

type HttpResult struct {
	HttpCode int
	Response []byte
	Header   http.Header
	Ctx      *gin.Context
}

func (client *HttpClientConf) GetRetryPolicy(opts *HttpRequestOptions) (retryPolicy RetryPolicy) {
	if opts != nil && opts.RetryPolicy != nil {
		// 接口维度超时策略
		retryPolicy = opts.RetryPolicy
	} else if client.retryPolicy != nil {
		// client维度超时策略(代码中指定的)
		retryPolicy = client.retryPolicy
	} else if client.RetryHttpCode > 0 {
		// 配置中指定的
		retryPolicy = func(resp *http.Response, err error) bool {
			return err != nil || resp == nil || resp.StatusCode >= client.RetryHttpCode
		}
	} else {
		// 默认超时策略
		retryPolicy = defaultRetryPolicy
	}
	return retryPolicy
}

func (client *HttpClientConf) httpDo(ctx *gin.Context, req *http.Request, opts *HttpRequestOptions, urlData []byte) (res HttpResult, err error) {
	t := beforeHttpStat(ctx, client, req)

	timeout := 3 * time.Second
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	} else if client.Timeout > 0 {
		timeout = client.Timeout
	}
	start := time.Now()
	fields := []zlog.Field{
		zlog.String("prot", "http"),
		zlog.String("method", req.Method),
		zlog.String("service", client.Service),
		zlog.String("domain", client.Domain),
		zlog.String("requestUri", req.URL.Path),
		zlog.Duration("timeout", timeout),
	}
	client.clientInit.Do(func() {
		if client.HTTPClient == nil {
			client.HTTPClient = &http.Client{
				Transport: client.GetTransPort(),
			}
		}
	})

	var (
		resp         *http.Response
		dataBuffer   *bytes.Reader
		maxAttempts  int
		attemptCount int
		doErr        error
		shouldRetry  bool
	)

	attemptCount, maxAttempts = 0, client.Retry

	// 策略选择优先级：option > client > default
	retryPolicy := client.GetRetryPolicy(opts)

	for {
		if req.GetBody != nil {
			bodyReadCloser, _ := req.GetBody()
			req.Body = bodyReadCloser
		} else if req.Body != nil {
			if dataBuffer == nil {
				data, err := io.ReadAll(req.Body)
				_ = req.Body.Close()
				if err != nil {
					return res, err
				}
				dataBuffer = bytes.NewReader(data)
				req.ContentLength = int64(dataBuffer.Len())
				req.Body = io.NopCloser(dataBuffer)
			}
			_, _ = dataBuffer.Seek(0, io.SeekStart)
		}

		attemptCount++

		c, _ := context.WithTimeout(context.Background(), timeout)
		req = req.WithContext(c)

		resp, doErr = client.HTTPClient.Do(req)

		shouldRetry = retryPolicy(resp, doErr)
		if !shouldRetry {
			break
		}

		msg := "hit retry policy attemptCount: " + strconv.Itoa(attemptCount)
		if doErr != nil {
			msg += ", error: " + doErr.Error()
		}
		zlog.WarnLogger(ctx, msg, fields...)

		if attemptCount > maxAttempts {
			break
		}

		// 符合retry条件...
		if doErr == nil {
			drainAndCloseBody(resp, 16384)
		}
	}

	if resp != nil {
		res.HttpCode = resp.StatusCode
		res.Response, err = io.ReadAll(resp.Body)
		res.Header = resp.Header
		_ = resp.Body.Close()
	}

	if shouldRetry {
		msg := "hit retry policy"
		if doErr != nil {
			msg += ", error: " + doErr.Error()
		}
		err = fmt.Errorf("giving up after %d attempt(s): %s", attemptCount, msg)
	}

	fields = append(fields,
		zlog.Int("respCode", res.HttpCode),
	)
	requestData, respData := client.formatLogMsg(urlData, res.Response)
	fields = append(fields, zlog.ByteString("reqParam", requestData))
	fields = append(fields, zlog.ByteString("respBody", respData))
	fields = append(fields, zlog.AppendCostTime(start, time.Now())...)
	msg := "http request success"
	if err != nil {
		msg = err.Error()
	}
	zlog.InfoLogger(ctx, msg, fields...)
	afterHttpStat(ctx, client, req.URL.Scheme, t)
	return res, err
}

func (client *HttpClientConf) DoStream(ctx *gin.Context, req *http.Request, opts *HttpRequestOptions, urlData []byte, f func(data string) error) (err error) {
	timeout := 3 * time.Second
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	} else if client.Timeout > 0 {
		timeout = client.Timeout
	}
	start := time.Now()
	fields := []zlog.Field{
		zlog.String("prot", "http"),
		zlog.String("method", req.Method),
		zlog.String("service", client.Service),
		zlog.String("domain", client.Domain),
		zlog.String("requestUri", req.URL.Path),
		zlog.Duration("timeout", timeout),
	}
	client.clientInit.Do(func() {
		if client.HTTPClient == nil {
			client.HTTPClient = &http.Client{
				Transport: client.GetTransPort(),
			}
		}
	})
	resp, doErr := client.HTTPClient.Do(req)
	if doErr != nil {
		return doErr
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	stopChan := make(chan bool)
	defer close(stopChan)
	gopool.Go(func() {
		for scanner.Scan() {
			ticker.Reset(timeout)
			data := scanner.Text()
			errA := f(data)
			if errA != nil {
				zlog.Errorf(ctx, "handler post stream data error: %v", errA)
				break
			}
		}
		utils.SafeSendBool(stopChan, true)
	})
	select {
	case <-ticker.C:
		// 超时处理逻辑
		zlog.Errorf(ctx, "streaming timeout")
	case <-stopChan:
		// 正常结束
	}
	drainAndCloseBody(resp, 16384)
	fields = append(fields,
		zlog.Int("respCode", resp.StatusCode),
	)
	requestData, _ := client.formatLogMsg(urlData, nil)
	fields = append(fields, zlog.ByteString("reqParam", requestData))
	fields = append(fields, zlog.AppendCostTime(start, time.Now())...)
	msg := "http request success"
	if doErr != nil {
		msg = err.Error()
	}
	zlog.InfoLogger(ctx, msg, fields...)
	return err
}

func (client *HttpClientConf) formatLogMsg(requestParam, responseData []byte) (req, resp []byte) {
	maxReqBodyLen := client.MaxReqBodyLen
	if maxReqBodyLen == 0 {
		maxReqBodyLen = _defaultPrintRequestLen
	}

	maxRespBodyLen := client.MaxRespBodyLen
	if maxRespBodyLen == 0 {
		maxRespBodyLen = _defaultPrintResponseLen
	}

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

func drainAndCloseBody(resp *http.Response, maxBytes int64) {
	if resp != nil {
		_, _ = io.CopyN(io.Discard, resp.Body, maxBytes)
		_ = resp.Body.Close()
	}
}

// retry 原因
type RetryPolicy func(resp *http.Response, err error) bool

// 默认重试策略，仅当底层返回error时重试。不解析http包
var defaultRetryPolicy = func(resp *http.Response, err error) bool {
	return err != nil
}
