package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/xiangtao94/golib/pkg/zlog"
)

type MetricsConfig struct {
	Namespace  string
	AppName    string
	Path       string
	Collectors []prometheus.Collector
}

type Metrics struct {
	registry     *prometheus.Registry
	path         string
	appName      string
	reqCount     *prometheus.CounterVec
	reqDuration  *prometheus.HistogramVec
	reqSizeBytes *prometheus.HistogramVec
	respSize     *prometheus.HistogramVec
}

func NewMetrics(config MetricsConfig) (*Metrics, error) {
	if config.Namespace == "" {
		config.Namespace = "monitor"
	}
	if config.Path == "" {
		config.Path = "/metrics"
	}
	labels := []string{"app_name", "status_class", "route", "method"}
	metrics := &Metrics{
		registry: prometheus.NewRegistry(),
		path:     config.Path,
		appName:  config.AppName,
		reqCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		}, labels),
		reqDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
		}, labels),
		reqSizeBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request size in bytes.",
		}, labels),
		respSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes.",
		}, labels),
	}

	allCollectors := []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		metrics.reqCount,
		metrics.reqDuration,
		metrics.reqSizeBytes,
		metrics.respSize,
	}
	allCollectors = append(allCollectors, config.Collectors...)
	for _, collector := range allCollectors {
		if collector == nil {
			continue
		}
		if err := metrics.registry.Register(collector); err != nil {
			return nil, fmt.Errorf("register prometheus collector: %w", err)
		}
	}
	return metrics, nil
}

func (metrics *Metrics) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.URL.Path == metrics.path {
			ctx.Next()
			return
		}
		start := time.Now()
		ctx.Next()

		route := ctx.FullPath()
		if route == "" {
			route = "unmatched"
		}
		statusClass := strconv.Itoa(ctx.Writer.Status()/100) + "xx"
		labels := []string{metrics.appName, statusClass, route, ctx.Request.Method}
		responseSize := max(ctx.Writer.Size(), 0)
		metrics.reqCount.WithLabelValues(labels...).Inc()
		metrics.reqDuration.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
		metrics.reqSizeBytes.WithLabelValues(labels...).Observe(getRequestSize(ctx.Request))
		metrics.respSize.WithLabelValues(labels...).Observe(float64(responseSize))
	}
}

func RegisterMetrics(engine *gin.Engine, metrics *Metrics) {
	engine.Use(metrics.Middleware())
	handler := promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
	engine.GET(metrics.path, func(ctx *gin.Context) {
		ctx.Request = ctx.Request.WithContext(zlog.WithNoLog(ctx.Request.Context()))
		handler.ServeHTTP(ctx.Writer, ctx.Request)
	})
}

func getRequestSize(request *http.Request) float64 {
	size := 0
	if request.URL != nil {
		size = len(request.URL.String())
	}
	size += len(request.Method)
	size += len(request.Proto)
	for name, values := range request.Header {
		size += len(name)
		for _, value := range values {
			size += len(value)
		}
	}
	size += len(request.Host)
	if request.ContentLength != -1 {
		size += int(request.ContentLength)
	}
	return float64(size)
}
