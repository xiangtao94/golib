package flow

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	httpclient "github.com/xiangtao94/golib/pkg/http"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestAPIHandleAcceptsEverySuccessfulHTTPStatus(t *testing.T) {
	api := &Api{}

	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusNoContent} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			response, err := api.handleContext(context.Background(), "/resource", &httpclient.Result{
				HttpCode: status,
				Response: []byte(`{"code":200,"message":"ok"}`),
			})

			require.NoError(t, err)
			require.NotNil(t, response)
		})
	}
}

func TestAPIHandleRejectsNilResponse(t *testing.T) {
	api := &Api{}

	response, err := api.handleContext(context.Background(), "/resource", nil)

	require.Nil(t, response)
	require.Error(t, err)
}

func TestDecodeAPIResponseReturnsTransportErrorFirst(t *testing.T) {
	api := &Api{}
	transportErr := errors.New("transport failed")

	require.ErrorIs(t, api.DecodeAPIResponse(context.Background(), nil, nil, transportErr), transportErr)
}

func TestAPIGetContextUsesTheCallContext(t *testing.T) {
	var requestID string
	client := &httpclient.ClientConf{
		Domain: "http://example.test",
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestID = request.Header.Get(zlog.HeaderRequestID)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"code":200,"message":"ok"}`)),
				Request:    request,
			}, nil
		}),
	}
	api := &Api{Client: client}
	ctx := zlog.WithRequestID(context.Background(), "req-call-context")

	response, err := api.ApiGet(ctx, "/resource", nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, "req-call-context", requestID)
}
