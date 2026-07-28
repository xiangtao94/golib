package mongodb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigUsesBoundedTimeoutsAndSecureTransport(t *testing.T) {
	config := DefaultConfig()

	if config.ConnectTimeout <= 0 {
		t.Fatal("ConnectTimeout must be positive")
	}
	if config.ServerSelectionTimeout <= 0 {
		t.Fatal("ServerSelectionTimeout must be positive")
	}
	if config.PingTimeout <= 0 {
		t.Fatal("PingTimeout must be positive")
	}
	if config.MaxPoolSize == 0 {
		t.Fatal("MaxPoolSize must be bounded")
	}
	if config.AllowInsecureTransport {
		t.Fatal("insecure MongoDB transport must be opt-in")
	}
}

func TestNewRejectsNilContext(t *testing.T) {
	//lint:ignore SA1012 This test verifies that nil contexts are rejected.
	client, err := New(nil, Config{})

	if !errors.Is(err, ErrNilContext) {
		t.Fatalf("New() error = %v, want ErrNilContext", err)
	}
	if client != nil {
		t.Fatal("New() returned a client")
	}
}

func TestNormalizeConfigRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "missing URI",
			config: Config{Database: "users"},
		},
		{
			name:   "missing database",
			config: Config{URI: "mongodb+srv://cluster.example.com"},
		},
		{
			name: "unsupported scheme",
			config: Config{
				URI:      "https://cluster.example.com",
				Database: "users",
			},
		},
		{
			name: "missing host",
			config: Config{
				URI:      "mongodb://",
				Database: "users",
			},
		},
		{
			name: "plaintext standard URI",
			config: Config{
				URI:      "mongodb://database.internal",
				Database: "users",
			},
		},
		{
			name: "SRV explicitly disables TLS",
			config: Config{
				URI:      "mongodb+srv://cluster.example.com/?tls=false",
				Database: "users",
			},
		},
		{
			name: "invalid certificates",
			config: Config{
				URI: "mongodb://database.internal/?" +
					"tls=true&tlsAllowInvalidCertificates=true",
				Database: "users",
			},
		},
		{
			name: "invalid hostnames",
			config: Config{
				URI: "mongodb://database.internal/?" +
					"tls=true&tlsAllowInvalidHostnames=true",
				Database: "users",
			},
		},
		{
			name: "negative timeout",
			config: Config{
				URI:            "mongodb+srv://cluster.example.com",
				Database:       "users",
				ConnectTimeout: -time.Second,
			},
		},
		{
			name: "minimum pool exceeds maximum",
			config: Config{
				URI:         "mongodb+srv://cluster.example.com",
				Database:    "users",
				MinPoolSize: 11,
				MaxPoolSize: 10,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := normalizeConfig(test.config)
			if err == nil {
				t.Fatal("normalizeConfig() succeeded")
			}
		})
	}
}

func TestNormalizeConfigAcceptsSecureAndExplicitDevelopmentURIs(t *testing.T) {
	tests := []Config{
		{
			URI:      "mongodb+srv://cluster.example.com",
			Database: "users",
		},
		{
			URI:      "mongodb://database.internal/?tls=true",
			Database: "users",
		},
		{
			URI:                    "mongodb://127.0.0.1:27017",
			Database:               "users",
			AllowInsecureTransport: true,
		},
		{
			URI: "mongodb://127.0.0.1:27017/?" +
				"tls=true&tlsAllowInvalidCertificates=true",
			Database:               "users",
			AllowInsecureTransport: true,
		},
	}

	for _, config := range tests {
		normalized, err := normalizeConfig(config)
		if err != nil {
			t.Fatalf("normalizeConfig(%q) error = %v", config.URI, err)
		}
		if normalized.ConnectTimeout <= 0 ||
			normalized.ServerSelectionTimeout <= 0 ||
			normalized.PingTimeout <= 0 {
			t.Fatalf("normalized timeouts = %+v", normalized)
		}
	}
}

func TestValidationErrorDoesNotExposeMongoCredentials(t *testing.T) {
	const secret = "top-secret"
	_, err := normalizeConfig(Config{
		URI:      "mongodb://admin:" + secret + "@database.internal",
		Database: "users",
	})

	if err == nil {
		t.Fatal("normalizeConfig() succeeded")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("validation error exposed credentials: %v", err)
	}
}

func TestNilClientAccessorsAndCloseAreSafe(t *testing.T) {
	var client *Client

	if client.Driver() != nil {
		t.Fatal("Driver() returned a value")
	}
	if client.Database() != nil {
		t.Fatal("Database() returned a value")
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
