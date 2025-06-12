package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/orm"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net/http"
	"time"
)

var namespace = "monitor"

var (
	labels = []string{"appName", "status", "endpoint", "method"}

	reqCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests made.",
		}, labels,
	)

	reqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latencies in seconds.",
		}, labels,
	)

	reqSizeBytes = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request sizes in bytes.",
		}, labels,
	)

	respSizeBytes = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response sizes in bytes.",
		}, labels,
	)
)

func RegistryMetrics(engine *gin.Engine, cs ...prometheus.Collector) {
	runtimeMetricsRegister := prometheus.NewRegistry()
	runtimeMetricsRegister.MustRegister(collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		reqCount,
		reqDuration,
		reqSizeBytes,
		respSizeBytes)
	if orm.MysqlPromCollector != nil {
		runtimeMetricsRegister.MustRegister(orm.MysqlPromCollector)
	}
	// 自定义监控指标
	runtimeMetricsRegister.MustRegister(cs...)
	engine.Use(PromMiddleware(env.AppName))
	engine.GET("/metrics", func(ctx *gin.Context) {
		// 避免metrics打点输出过多无用日志
		zlog.SetNoLogFlag(ctx)
		httpHandler := promhttp.InstrumentMetricHandler(
			runtimeMetricsRegister, promhttp.HandlerFor(runtimeMetricsRegister, promhttp.HandlerOpts{}),
		)
		httpHandler.ServeHTTP(ctx.Writer, ctx.Request)
	})
}

func PromMiddleware(appName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := fmt.Sprintf("%d", c.Writer.Status())
		endpoint := c.Request.URL.Path
		method := c.Request.Method
		lvs := []string{appName, status, endpoint, method}
		// no response content will return -1
		respSize := c.Writer.Size()
		if respSize < 0 {
			respSize = 0
		}
		reqCount.WithLabelValues(lvs...).Inc()
		reqDuration.WithLabelValues(lvs...).Observe(getRequestCost(start, time.Now()))
		reqSizeBytes.WithLabelValues(lvs...).Observe(getRequestSize(c.Request))
		respSizeBytes.WithLabelValues(lvs...).Observe(float64(respSize))
	}
}

func getRequestCost(start, end time.Time) float64 {
	return float64(end.Sub(start).Nanoseconds()/1e4) / 100.0
}

// getRequestSize returns the size of request object.
func getRequestSize(r *http.Request) float64 {
	size := 0
	if r.URL != nil {
		size = len(r.URL.String())
	}

	size += len(r.Method)
	size += len(r.Proto)

	for name, values := range r.Header {
		size += len(name)
		for _, value := range values {
			size += len(value)
		}
	}
	size += len(r.Host)

	// r.Form and r.MultipartForm are assumed to be included in r.URL.
	if r.ContentLength != -1 {
		size += int(r.ContentLength)
	}
	return float64(size)
}
