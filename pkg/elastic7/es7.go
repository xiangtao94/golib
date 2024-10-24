package elastic7

import (
	"fmt"
	"github.com/tiant-go/golib/pkg/env"
	"github.com/tiant-go/golib/pkg/zlog"
	"net/http"
	"strings"

	"github.com/olivere/elastic/v7"
)

type ElasticClientConfig struct {
	Addr     string `yaml:"addr"`
	Service  string `yaml:"service"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`

	Sniff       bool `yaml:"sniff"`
	HealthCheck bool `yaml:"healthCheck"`
	Gzip        bool `yaml:"gzip"`

	Decoder       elastic.Decoder
	RetryStrategy elastic.Retrier
	HttpClient    *http.Client
	Other         []elastic.ClientOptionFunc
}

func NewESClient(cfg ElasticClientConfig) (*elastic.Client, error) {
	addrs := strings.Split(cfg.Addr, ",")
	options := []elastic.ClientOptionFunc{
		elastic.SetURL(addrs...),
		elastic.SetSniff(cfg.Sniff),
		elastic.SetHealthcheck(cfg.HealthCheck),
		elastic.SetGzip(cfg.Gzip),
	}

	l := esLogger{
		addr:    cfg.Addr,
		service: cfg.Service,
	}
	options = append(options,
		elastic.SetTraceLog(&elasticDebugLogger{l}),
		elastic.SetInfoLog(&elasticInfoLogger{l}),
		elastic.SetErrorLog(&elasticErrorLogger{l}))

	if cfg.Username != "" || cfg.Password != "" {
		options = append(options, elastic.SetBasicAuth(cfg.Username, cfg.Password))
	}

	if cfg.HttpClient != nil {
		options = append(options, elastic.SetHttpClient(cfg.HttpClient))
	}
	if cfg.Decoder != nil {
		options = append(options, elastic.SetDecoder(cfg.Decoder))
	}

	if cfg.RetryStrategy != nil {
		options = append(options, elastic.SetRetrier(cfg.RetryStrategy))
	}

	// override
	if len(cfg.Other) > 0 {
		options = append(options, cfg.Other...)
	}
	return elastic.NewClient(options...)
}

type esLogger struct {
	addr    string
	service string
}

func (l esLogger) commonFields(ralCode int) []zlog.Field {
	return []zlog.Field{
		zlog.String("prot", "es"),
		zlog.String("service", l.service),
		zlog.String("addr", l.addr),
		zlog.String("localIp", env.LocalIP),
		zlog.String("module", env.GetAppName()),
		zlog.Int("ralCode", ralCode),
		zlog.Int("cost", -1),
		zlog.String("requestStartTime", ""),
		zlog.String("requestEndTime", ""),
		zlog.String("requestId", ""),
	}
}

type elasticDebugLogger struct {
	esLogger
}
type elasticInfoLogger struct {
	esLogger
}
type elasticErrorLogger struct {
	esLogger
}

func (l elasticDebugLogger) Printf(format string, v ...interface{}) {
	zlog.DebugLogger(nil, fmt.Sprintf(format, v...), l.commonFields(0)...)
}

func (l elasticInfoLogger) Printf(format string, v ...interface{}) {
	zlog.InfoLogger(nil, fmt.Sprintf(format, v...), l.commonFields(0)...)
}

func (l elasticErrorLogger) Printf(format string, v ...interface{}) {
	zlog.ErrorLogger(nil, fmt.Sprintf(format, v...), l.commonFields(-1)...)
}
