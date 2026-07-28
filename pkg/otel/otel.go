// Package otel provides optional OpenTelemetry instrumentation adapters.
// Applications own SDK providers, exporters, sampling, and shutdown.
package otel

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/xiangtao94/golib/pkg/httpclient"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type Config struct {
	ServiceName    string
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	Propagators    propagation.TextMapPropagator
}

// Gin returns the OTel server instrumentation followed by a middleware that
// adds trace_id and span_id to zlog context fields.
func Gin(config Config) ([]gin.HandlerFunc, error) {
	if strings.TrimSpace(config.ServiceName) == "" {
		return nil, errors.New("otel: service name is required")
	}
	var options []otelgin.Option
	if config.TracerProvider != nil {
		options = append(options, otelgin.WithTracerProvider(config.TracerProvider))
	}
	if config.MeterProvider != nil {
		options = append(options, otelgin.WithMeterProvider(config.MeterProvider))
	}
	if config.Propagators != nil {
		options = append(options, otelgin.WithPropagators(config.Propagators))
	}
	return []gin.HandlerFunc{
		otelgin.Middleware(config.ServiceName, options...),
		traceFieldsMiddleware,
	}, nil
}

// HTTPTransport adapts otelhttp to golib's standard RoundTripper middleware.
func HTTPTransport(config Config) httpclient.TransportMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		var options []otelhttp.Option
		if config.TracerProvider != nil {
			options = append(options, otelhttp.WithTracerProvider(config.TracerProvider))
		}
		if config.MeterProvider != nil {
			options = append(options, otelhttp.WithMeterProvider(config.MeterProvider))
		}
		if config.Propagators != nil {
			options = append(options, otelhttp.WithPropagators(config.Propagators))
		}
		return otelhttp.NewTransport(next, options...)
	}
}

func WithTraceFields(ctx context.Context) context.Context {
	if ctx == nil {
		panic("otel: nil context")
	}
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ctx
	}
	return zlog.WithFields(
		ctx,
		zlog.String("trace_id", spanContext.TraceID().String()),
		zlog.String("span_id", spanContext.SpanID().String()),
	)
}

func traceFieldsMiddleware(ctx *gin.Context) {
	ctx.Request = ctx.Request.WithContext(WithTraceFields(ctx.Request.Context()))
	ctx.Next()
}
