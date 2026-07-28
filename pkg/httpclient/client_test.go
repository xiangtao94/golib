package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"resty.dev/v3"

	"github.com/xiangtao94/golib/pkg/zlog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func response(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func TestDefaultConfigUsesBoundedTimeoutsAndNoRetries(t *testing.T) {
	config := DefaultConfig()

	require.Positive(t, config.Timeout)
	require.Positive(t, config.ConnectTimeout)
	require.Zero(t, config.RetryCount)
	require.False(t, config.AllowHTTP)
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "base URL and base URLs",
			config: Config{
				BaseURL:  "https://api.example.com",
				BaseURLs: []string{"https://api-2.example.com"},
			},
		},
		{
			name:   "plaintext URL by default",
			config: Config{BaseURL: "http://api.example.com"},
		},
		{
			name:   "URL credentials",
			config: Config{BaseURL: "https://user:secret@api.example.com"},
		},
		{
			name:   "URL query",
			config: Config{BaseURL: "https://api.example.com?token=secret"},
		},
		{
			name:   "negative retries",
			config: Config{RetryCount: -1},
		},
		{
			name: "negative retry wait",
			config: Config{
				RetryWaitTime: -time.Second,
			},
		},
		{
			name: "nil transport middleware",
			config: Config{
				TransportMiddleware: []TransportMiddleware{nil},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := New(test.config)

			require.Error(t, err)
			require.Nil(t, client)
		})
	}
}

func TestNewBuildsConcreteRestyClient(t *testing.T) {
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "middleware", request.Header.Get("X-Test-Middleware"))
		return response(request, http.StatusNoContent, ""), nil
	})
	middleware := func(next http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(request *http.Request) (*http.Response, error) {
			request.Header.Set("X-Test-Middleware", "middleware")
			return next.RoundTrip(request)
		})
	}

	client, err := New(Config{
		Service:             "payments",
		BaseURL:             "http://payment.internal",
		AllowHTTP:           true,
		Timeout:             3 * time.Second,
		ConnectTimeout:      time.Second,
		RetryCount:          2,
		RetryWaitTime:       time.Millisecond,
		RetryMaxWaitTime:    2 * time.Millisecond,
		TraceEnabled:        true,
		Transport:           base,
		TransportMiddleware: []TransportMiddleware{middleware},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	require.IsType(t, &resty.Client{}, client)
	require.Equal(t, "http://payment.internal", client.BaseURL())
	require.Equal(t, 3*time.Second, client.Timeout())
	require.Equal(t, 2, client.RetryCount())
	require.False(t, client.IsRetryAllowNonIdempotent())
	require.True(t, client.IsTrace())

	result, err := client.R().Get("/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, result.StatusCode())
}

func TestNewDefaultTransportRequiresTLS12(t *testing.T) {
	client, err := New(Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	transport, err := client.HTTPTransport()
	require.NoError(t, err)
	require.NotNil(t, transport.TLSClientConfig)
	require.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
}

func TestNewDoesNotMutateBaseURLs(t *testing.T) {
	baseURLs := []string{"  https://api-1.example.com  ", "https://api-2.example.com"}

	client, err := New(Config{BaseURLs: baseURLs})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	require.Equal(t, "  https://api-1.example.com  ", baseURLs[0])
}

func TestRequestMiddlewarePropagatesRequestID(t *testing.T) {
	var requestID string
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestID = request.Header.Get(zlog.HeaderRequestID)
		return response(request, http.StatusOK, "ok"), nil
	})
	client, err := New(Config{
		BaseURL:   "http://service.internal",
		AllowHTTP: true,
		Transport: transport,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx := zlog.WithRequestID(context.Background(), "request-123")
	_, err = client.R().SetContext(ctx).Get("/")

	require.NoError(t, err)
	require.Equal(t, "request-123", requestID)
}

func TestRequestMiddlewareRejectsFullHTTPURLByDefault(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return response(request, http.StatusOK, "ok"), nil
	})
	client, err := New(Config{Transport: transport})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	result, err := client.R().Get("http://service.internal/health")

	require.ErrorIs(t, err, ErrInsecureTransport)
	require.Nil(t, result)
	require.Zero(t, attempts.Load())
}

func TestRequestMiddlewareRejectsHTTPBaseURLSetAfterConstruction(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return response(request, http.StatusOK, "ok"), nil
	})
	client, err := New(Config{Transport: transport})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	client.SetBaseURL("http://service.internal")

	result, err := client.R().Get("/health")

	require.ErrorIs(t, err, ErrInsecureTransport)
	require.Nil(t, result)
	require.Zero(t, attempts.Load())
}

func TestRedirectPolicyRejectsHTTPSDowngrade(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts.Add(1)
		result := response(request, http.StatusFound, "")
		result.Header.Set("Location", "http://service.internal/plaintext")
		return result, nil
	})
	client, err := New(Config{Transport: transport})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	result, err := client.R().Get("https://service.internal/secure")

	require.ErrorIs(t, err, ErrInsecureTransport)
	require.NotNil(t, result)
	require.EqualValues(t, 1, attempts.Load())
}

func TestNonIdempotentRetryRequiresIdempotencyKey(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return response(request, http.StatusServiceUnavailable, "retry"), nil
	})
	client, err := New(Config{
		BaseURL:   "http://service.internal",
		AllowHTTP: true,
		Transport: transport,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	result, err := client.R().
		SetRetryCount(1).
		SetRetryAllowNonIdempotent(true).
		Post("/")

	require.ErrorIs(t, err, ErrIdempotencyKeyRequired)
	require.Nil(t, result)
	require.Zero(t, attempts.Load())
}

func TestNonIdempotentRetryWithIdempotencyKeyIsAllowed(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempt := attempts.Add(1)
		if attempt == 1 {
			return response(request, http.StatusServiceUnavailable, "retry"), nil
		}
		return response(request, http.StatusOK, "ok"), nil
	})
	client, err := New(Config{
		BaseURL:          "http://service.internal",
		AllowHTTP:        true,
		Transport:        transport,
		RetryWaitTime:    time.Nanosecond,
		RetryMaxWaitTime: time.Nanosecond,
		RetryConditions: []resty.RetryConditionFunc{
			func(result *resty.Response, _ error) bool {
				return result != nil && result.StatusCode() == http.StatusServiceUnavailable
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	result, err := client.R().
		SetHeader(HeaderIdempotencyKey, "operation-123").
		SetRetryCount(1).
		SetRetryAllowNonIdempotent(true).
		Post("/")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.StatusCode())
	require.EqualValues(t, 2, attempts.Load())
}

func TestNewRejectsMiddlewareReturningNil(t *testing.T) {
	client, err := New(Config{
		TransportMiddleware: []TransportMiddleware{
			func(http.RoundTripper) http.RoundTripper { return nil },
		},
	})

	require.Error(t, err)
	require.Nil(t, client)
}

func TestCloseClosesBaseTransportIdleConnectionsOnce(t *testing.T) {
	transport := &closingTransport{}
	client, err := New(Config{Transport: transport})
	require.NoError(t, err)

	require.NoError(t, client.Close())
	require.NoError(t, client.Close())
	require.EqualValues(t, 1, transport.closed.Load())
}

type closingTransport struct {
	closed atomic.Int32
}

func (*closingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not implemented")
}

func (transport *closingTransport) CloseIdleConnections() {
	transport.closed.Add(1)
}
