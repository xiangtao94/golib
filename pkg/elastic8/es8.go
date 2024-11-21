package elastic8

import (
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/go-elasticsearch/v8"
	"net/http"
	"os"
)

type ElasticClientConfig struct {
	Addr     string `yaml:"addr"`
	Service  string `yaml:"service"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`

	Sniff       bool `yaml:"sniff"`
	HealthCheck bool `yaml:"healthCheck"`
	Gzip        bool `yaml:"gzip"`
	HttpClient  *http.Client
}

func GetClient(config ElasticClientConfig) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{config.Addr},
		Username:  config.Username,
		Password:  config.Password,
		Logger: &elastictransport.JSONLogger{
			Output:             os.Stdout,
			EnableRequestBody:  true,
			EnableResponseBody: true,
		},
	}
	return elasticsearch.NewClient(cfg)
}
