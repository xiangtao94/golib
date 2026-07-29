// Package httpclient builds a configured Resty client without hiding Resty's
// request, response, streaming, or middleware APIs.
package httpclient

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
	"resty.dev/v3"

	"github.com/xiangtao94/golib/pkg/zlog"
)

const HeaderIdempotencyKey = "Idempotency-Key"

var ErrIdempotencyKeyRequired = errors.New("httpclient: idempotency key required for non-idempotent retry")
var ErrInsecureTransport = errors.New("httpclient: HTTP transport is not allowed")

type TransportMiddleware func(http.RoundTripper) http.RoundTripper

type Config struct {
	Service          string        `yaml:"service"`
	BaseURL          string        `yaml:"baseURL"`
	BaseURLs         []string      `yaml:"baseURLs"`
	Timeout          time.Duration `yaml:"timeout"`
	ConnectTimeout   time.Duration `yaml:"connectTimeout"`
	RetryCount       int           `yaml:"retryCount"`
	RetryWaitTime    time.Duration `yaml:"retryWaitTime"`
	RetryMaxWaitTime time.Duration `yaml:"retryMaxWaitTime"`
	Proxy            string        `yaml:"proxy"`
	TraceEnabled     bool          `yaml:"traceEnabled"`
	AllowHTTP        bool          `yaml:"allowHTTP"`

	Transport           http.RoundTripper          `yaml:"-"`
	TransportMiddleware []TransportMiddleware      `yaml:"-"`
	RetryConditions     []resty.RetryConditionFunc `yaml:"-"`
}

func DefaultConfig() Config {
	return Config{
		Timeout:          5 * time.Second,
		ConnectTimeout:   5 * time.Second,
		RetryCount:       0,
		RetryWaitTime:    500 * time.Millisecond,
		RetryMaxWaitTime: 2 * time.Second,
	}
}

// New returns the concrete Resty client. The caller owns it and must call
// Close when the client is no longer needed.
func New(config Config) (*resty.Client, error) {
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}

	client := resty.NewWithTransportSettings(&resty.TransportSettings{
		DialerTimeout:         normalized.ConnectTimeout,
		DialerKeepAlive:       30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	})
	defaultTransport, err := client.HTTPTransport()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("httpclient: get default transport: %w", err)
	}
	defaultTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if normalized.Proxy != "" {
		client.SetProxy(normalized.Proxy)
	}
	baseTransport := client.Transport()
	if normalized.Transport != nil {
		baseTransport = normalized.Transport
	}
	transport := baseTransport
	for index := len(normalized.TransportMiddleware) - 1; index >= 0; index-- {
		transport = normalized.TransportMiddleware[index](transport)
		if transport == nil {
			_ = client.Close()
			return nil, fmt.Errorf("httpclient: transport middleware %d returned nil", index)
		}
	}
	client.SetTransport(transport)
	closeTransport := transport
	if _, ok := closeTransport.(interface{ CloseIdleConnections() }); !ok {
		closeTransport = baseTransport
	}
	if closer, ok := closeTransport.(interface{ CloseIdleConnections() }); ok {
		client.OnClose(closer.CloseIdleConnections)
	}

	if normalized.BaseURL != "" {
		client.SetBaseURL(normalized.BaseURL)
	}
	if len(normalized.BaseURLs) > 0 {
		loadBalancer, loadBalancerErr := resty.NewRoundRobin(normalized.BaseURLs...)
		if loadBalancerErr != nil {
			_ = client.Close()
			return nil, fmt.Errorf("httpclient: create load balancer: %w", loadBalancerErr)
		}
		client.SetLoadBalancer(loadBalancer)
	}
	client.
		SetTimeout(normalized.Timeout).
		SetRetryCount(normalized.RetryCount).
		SetRetryWaitTime(normalized.RetryWaitTime).
		SetRetryMaxWaitTime(normalized.RetryMaxWaitTime).
		SetRetryAllowNonIdempotent(false).
		SetTrace(normalized.TraceEnabled).
		SetLogger(zlog.NewLoggerWithSkip(2).Sugar())
	if !normalized.AllowHTTP {
		client.SetRedirectPolicy(
			resty.RedirectFlexiblePolicy(10),
			resty.RedirectPolicyFunc(func(request *http.Request, _ []*http.Request) error {
				if request.URL != nil && request.URL.Scheme == "http" {
					return ErrInsecureTransport
				}
				return nil
			}),
		)
	}
	if len(normalized.RetryConditions) > 0 {
		client.AddRetryConditions(normalized.RetryConditions...)
	}
	client.AddRequestMiddleware(propagateRequestMetadata(normalized.AllowHTTP))
	client.AddResponseMiddleware(logResponse(normalized.Service))

	return client, nil
}

func normalizeConfig(config Config) (Config, error) {
	if config.RetryCount < 0 {
		return Config{}, errors.New("httpclient: retry count cannot be negative")
	}
	if config.Timeout < 0 || config.ConnectTimeout < 0 {
		return Config{}, errors.New("httpclient: timeout cannot be negative")
	}
	if config.RetryWaitTime < 0 || config.RetryMaxWaitTime < 0 {
		return Config{}, errors.New("httpclient: retry wait cannot be negative")
	}
	if strings.TrimSpace(config.BaseURL) != "" && len(config.BaseURLs) > 0 {
		return Config{}, errors.New("httpclient: base URL and base URLs are mutually exclusive")
	}
	if config.Transport != nil && strings.TrimSpace(config.Proxy) != "" {
		return Config{}, errors.New("httpclient: proxy cannot be combined with a custom transport")
	}
	for index, middleware := range config.TransportMiddleware {
		if middleware == nil {
			return Config{}, fmt.Errorf("httpclient: transport middleware %d is nil", index)
		}
	}

	config.BaseURL = strings.TrimSpace(config.BaseURL)
	if config.BaseURL != "" {
		if err := validateBaseURL(config.BaseURL, config.AllowHTTP); err != nil {
			return Config{}, fmt.Errorf("httpclient: invalid base URL: %w", err)
		}
	}
	config.BaseURLs = slices.Clone(config.BaseURLs)
	for index, baseURL := range config.BaseURLs {
		config.BaseURLs[index] = strings.TrimSpace(baseURL)
		if err := validateBaseURL(config.BaseURLs[index], config.AllowHTTP); err != nil {
			return Config{}, fmt.Errorf("httpclient: invalid base URL at index %d: %w", index, err)
		}
	}
	if config.Proxy != "" {
		if err := validateProxyURL(config.Proxy); err != nil {
			return Config{}, err
		}
	}

	defaults := DefaultConfig()
	if config.Timeout == 0 {
		config.Timeout = defaults.Timeout
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = defaults.ConnectTimeout
	}
	if config.RetryWaitTime == 0 {
		config.RetryWaitTime = defaults.RetryWaitTime
	}
	if config.RetryMaxWaitTime == 0 {
		config.RetryMaxWaitTime = defaults.RetryMaxWaitTime
	}
	if config.RetryMaxWaitTime < config.RetryWaitTime {
		return Config{}, errors.New("httpclient: retry max wait cannot be shorter than retry wait")
	}
	return config, nil
}

func validateBaseURL(value string, allowHTTP bool) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return errors.New("URL cannot be parsed")
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http") {
		return errors.New("URL must use HTTPS unless AllowHTTP is enabled")
	}
	if parsed.Host == "" {
		return errors.New("URL host is required")
	}
	if parsed.User != nil {
		return errors.New("URL must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("URL must not contain a query or fragment")
	}
	return nil
}

func validateProxyURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return errors.New("httpclient: invalid proxy URL")
	}
	switch parsed.Scheme {
	case "http", "https", "socks5":
		return nil
	default:
		return errors.New("httpclient: proxy URL must use http, https, or socks5")
	}
}

func propagateRequestMetadata(allowHTTP bool) resty.RequestMiddleware {
	return func(client *resty.Client, request *resty.Request) error {
		if !allowHTTP && requestUsesHTTP(client, request) {
			return ErrInsecureTransport
		}
		if request.IsRetryAllowNonIdempotent &&
			request.RetryCount > 0 &&
			!isIdempotent(request.Method) &&
			strings.TrimSpace(request.Header.Get(HeaderIdempotencyKey)) == "" {
			return ErrIdempotencyKeyRequired
		}
		ctx, requestID := zlog.EnsureRequestID(
			request.Context(),
			request.Header.Get(zlog.HeaderRequestID),
		)
		request.SetContext(ctx)
		request.SetHeader(zlog.HeaderRequestID, requestID)
		return nil
	}
}

func requestUsesHTTP(client *resty.Client, request *resty.Request) bool {
	if request.RawRequest != nil &&
		request.RawRequest.URL != nil &&
		request.RawRequest.URL.Scheme == "http" {
		return true
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		return false
	}
	if parsed.IsAbs() {
		return parsed.Scheme == "http"
	}
	baseURL, err := url.Parse(client.BaseURL())
	if err != nil {
		return false
	}
	return baseURL.ResolveReference(parsed).Scheme == "http"
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func logResponse(service string) resty.ResponseMiddleware {
	return func(_ *resty.Client, response *resty.Response) error {
		if response == nil || response.Request == nil {
			return nil
		}
		request := response.Request
		if response.CascadeError != nil {
			if !zlog.ErrorEnabled(request.Context()) {
				return nil
			}
		} else if !zlog.InfoEnabled(request.Context()) {
			return nil
		}
		fields := []zap.Field{
			zlog.String("service", service),
			zlog.String("method", request.Method),
			zlog.String("requestUrl", sanitizedRequestURL(request)),
			zlog.Int("attempts", request.Attempt),
			zlog.Int("status", response.StatusCode()),
			zlog.Duration("duration", time.Since(request.StartTime)),
		}
		logger := zlog.LoggerWithContext(zlog.NewLoggerWithSkip(3), request.Context())
		if response.CascadeError != nil {
			logger.Error(
				"http request completed",
				append(fields, zlog.String("errorType", fmt.Sprintf("%T", response.CascadeError)))...,
			)
			return nil
		}
		logger.Info("http request completed", fields...)
		return nil
	}
}

func sanitizedRequestURL(request *resty.Request) string {
	if request.RawRequest != nil && request.RawRequest.URL != nil {
		sanitized := *request.RawRequest.URL
		sanitized.User = nil
		sanitized.RawQuery = ""
		sanitized.Fragment = ""
		return sanitized.String()
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
