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
	"resty.dev/v3"
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
		RetryCondition: func(response *resty.Response, err error) bool {
			return err == nil && response.StatusCode() == http.StatusServiceUnavailable
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
