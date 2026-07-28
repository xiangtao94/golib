package otel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	httpclient "github.com/xiangtao94/golib/pkg/http"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func TestGinCreatesServerSpanAndAddsTraceFieldsToContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	handlers, err := Gin(Config{
		ServiceName:    "user-center",
		TracerProvider: provider,
		Propagators:    propagation.TraceContext{},
	})
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(handlers...)

	var spanContext trace.SpanContext
	core, observed := observer.New(zap.InfoLevel)
	engine.GET("/users/:id", func(ctx *gin.Context) {
		spanContext = trace.SpanContextFromContext(ctx.Request.Context())
		zlog.LoggerWithContext(zap.New(core), ctx.Request.Context()).Info("handled")
		ctx.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/123", nil))

	require.Equal(t, http.StatusNoContent, response.Code)
	require.True(t, spanContext.IsValid())
	require.Equal(t, spanContext.TraceID().String(), observed.All()[0].ContextMap()["trace_id"])
	require.Equal(t, spanContext.SpanID().String(), observed.All()[0].ContextMap()["span_id"])
	require.Len(t, recorder.Ended(), 1)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestHTTPTransportInjectsTraceContext(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	var traceParent string
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		traceParent = request.Header.Get("traceparent")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    request,
		}, nil
	})
	middleware := HTTPTransport(Config{
		TracerProvider: provider,
		Propagators:    propagation.TraceContext{},
	})
	client, err := httpclient.NewClient(httpclient.ClientConfig{
		Domain:              "http://payment.internal",
		Transport:           base,
		TransportMiddleware: []httpclient.TransportMiddleware{middleware},
	})
	require.NoError(t, err)
	ctx, span := provider.Tracer("test").Start(context.Background(), "parent")
	defer span.End()

	_, err = client.Get(ctx, httpclient.RequestOptions{})

	require.NoError(t, err)
	require.NotEmpty(t, traceParent)
}

func TestGinRequiresServiceName(t *testing.T) {
	handlers, err := Gin(Config{})

	require.Error(t, err)
	require.Nil(t, handlers)
}
