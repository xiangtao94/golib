package http

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type trackingRoundTripper struct {
	attempts atomic.Int32
	closed   atomic.Int32
}

func (transport *trackingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	attempt := transport.attempts.Add(1)
	status := http.StatusServiceUnavailable
	if attempt > 1 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("response")),
		Request:    request,
	}, nil
}

func (transport *trackingRoundTripper) CloseIdleConnections() {
	transport.closed.Add(1)
}

func TestHTTPClientOmitsBodiesByDefault(t *testing.T) {
	config := DefaultClientConfig()

	require.Zero(t, config.MaxReqBodyLen)
	require.Zero(t, config.MaxRespBodyLen)
	require.Zero(t, config.RetryCount)
}

func TestNewHTTPClientReturnsConfigurationErrorImmediately(t *testing.T) {
	client, err := NewClient(ClientConfig{Domains: []string{"://invalid"}})

	require.Error(t, err)
	require.Nil(t, client)
}

func TestHTTPClientAppliesTraceAndRetryPolicy(t *testing.T) {
	transport := &trackingRoundTripper{}
	client, err := NewClient(ClientConfig{
		Domain:        "http://example.test",
		TraceEnabled:  true,
		RetryCount:    1,
		RetryWaitTime: time.Nanosecond,
		Transport:     transport,
		RetryCondition: func(response *http.Response, err error) bool {
			return err == nil && response.StatusCode == http.StatusServiceUnavailable
		},
	})
	require.NoError(t, err)

	response, err := client.Get(context.Background(), RequestOptions{})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.HttpCode)
	require.EqualValues(t, 2, transport.attempts.Load())
	require.True(t, client.client.IsTrace())
}

func TestHTTPClientZeroRetryCountMeansNoRetries(t *testing.T) {
	client, err := NewClient(ClientConfig{})

	require.NoError(t, err)
	require.Zero(t, client.client.RetryCount())
}

func TestHTTPClientCloseClosesCustomTransport(t *testing.T) {
	transport := &trackingRoundTripper{}
	client, err := NewClient(ClientConfig{Transport: transport})
	require.NoError(t, err)

	require.NoError(t, client.Close())
	require.NoError(t, client.Close())
	require.EqualValues(t, 1, transport.closed.Load())
}

func TestHTTPClientRejectsNilContext(t *testing.T) {
	client, err := NewClient(ClientConfig{Domain: "http://example.test"})
	require.NoError(t, err)

	response, err := client.Get(nil, RequestOptions{})

	require.ErrorIs(t, err, ErrNilContext)
	require.Nil(t, response)
}

func TestRequestNoRetryOverridesClientPolicy(t *testing.T) {
	transport := &trackingRoundTripper{}
	client, err := NewClient(ClientConfig{
		Domain:        "http://example.test",
		RetryCount:    1,
		RetryWaitTime: time.Nanosecond,
		Transport:     transport,
		RetryCondition: func(response *http.Response, err error) bool {
			return err == nil && response.StatusCode == http.StatusServiceUnavailable
		},
	})
	require.NoError(t, err)

	response, err := client.Get(context.Background(), RequestOptions{
		Retry: &RetryPolicy{Mode: RetryNone},
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, response.HttpCode)
	require.EqualValues(t, 1, transport.attempts.Load())
}

func TestRequestSafeMethodRetryOverridesDisabledClient(t *testing.T) {
	transport := &trackingRoundTripper{}
	client, err := NewClient(ClientConfig{
		Domain:    "http://example.test",
		Transport: transport,
		RetryCondition: func(response *http.Response, err error) bool {
			return err == nil && response.StatusCode == http.StatusServiceUnavailable
		},
	})
	require.NoError(t, err)

	response, err := client.Get(context.Background(), RequestOptions{
		Retry: &RetryPolicy{
			Mode:        RetrySafeMethods,
			MaxRetries:  1,
			MinWaitTime: time.Nanosecond,
		},
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.HttpCode)
	require.EqualValues(t, 2, transport.attempts.Load())
}

func TestRequestSafeMethodPolicyDoesNotRetryPost(t *testing.T) {
	transport := &trackingRoundTripper{}
	client, err := NewClient(ClientConfig{
		Domain:    "http://example.test",
		Transport: transport,
		RetryCondition: func(response *http.Response, err error) bool {
			return err == nil && response.StatusCode == http.StatusServiceUnavailable
		},
	})
	require.NoError(t, err)

	response, err := client.Post(context.Background(), RequestOptions{
		Retry: &RetryPolicy{
			Mode:        RetrySafeMethods,
			MaxRetries:  1,
			MinWaitTime: time.Nanosecond,
		},
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, response.HttpCode)
	require.EqualValues(t, 1, transport.attempts.Load())
}

func TestRequestNonIdempotentRetryRequiresIdempotencyKey(t *testing.T) {
	transport := &trackingRoundTripper{}
	client, err := NewClient(ClientConfig{
		Domain:    "http://example.test",
		Transport: transport,
	})
	require.NoError(t, err)

	response, err := client.Post(context.Background(), RequestOptions{
		Retry: &RetryPolicy{
			Mode:       RetryWithIdempotencyKey,
			MaxRetries: 1,
		},
	})

	require.ErrorIs(t, err, ErrIdempotencyKeyRequired)
	require.Nil(t, response)
	require.Zero(t, transport.attempts.Load())
}

func TestRequestNonIdempotentRetryWithIdempotencyKey(t *testing.T) {
	transport := &trackingRoundTripper{}
	client, err := NewClient(ClientConfig{
		Domain:    "http://example.test",
		Transport: transport,
		RetryCondition: func(response *http.Response, err error) bool {
			return err == nil && response.StatusCode == http.StatusServiceUnavailable
		},
	})
	require.NoError(t, err)

	response, err := client.Post(context.Background(), RequestOptions{
		Headers: map[string]string{HeaderIdempotencyKey: "payment-123"},
		Retry: &RetryPolicy{
			Mode:        RetryWithIdempotencyKey,
			MaxRetries:  1,
			MinWaitTime: time.Nanosecond,
		},
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.HttpCode)
	require.EqualValues(t, 2, transport.attempts.Load())
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestHTTPClientAppliesTransportMiddleware(t *testing.T) {
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "instrumented", request.Header.Get("X-Test-Middleware"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    request,
		}, nil
	})
	client, err := NewClient(ClientConfig{
		Domain:    "http://example.test",
		Transport: base,
		TransportMiddleware: []TransportMiddleware{
			func(next http.RoundTripper) http.RoundTripper {
				return roundTripFunc(func(request *http.Request) (*http.Response, error) {
					request.Header.Set("X-Test-Middleware", "instrumented")
					return next.RoundTrip(request)
				})
			},
		},
	})
	require.NoError(t, err)

	response, err := client.Get(context.Background(), RequestOptions{})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.HttpCode)
}

func TestRequestBudgetCancelsTheWholeCall(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	client, err := NewClient(ClientConfig{
		Domain:    "http://example.test",
		Transport: transport,
	})
	require.NoError(t, err)

	response, err := client.Get(context.Background(), RequestOptions{Budget: 10 * time.Millisecond})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, response)
}
