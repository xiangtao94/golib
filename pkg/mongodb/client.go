// Package mongodb owns MongoDB client construction, readiness verification,
// and shutdown. Collection and repository semantics remain in business code.
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var (
	ErrNilContext = errors.New("mongodb: nil context")
	ErrNilClient  = errors.New("mongodb: nil client")
)

type Config struct {
	URI                    string        `yaml:"uri"`
	Database               string        `yaml:"database"`
	AppName                string        `yaml:"appName"`
	ConnectTimeout         time.Duration `yaml:"connectTimeout"`
	ServerSelectionTimeout time.Duration `yaml:"serverSelectionTimeout"`
	PingTimeout            time.Duration `yaml:"pingTimeout"`
	MinPoolSize            uint64        `yaml:"minPoolSize"`
	MaxPoolSize            uint64        `yaml:"maxPoolSize"`
	MaxConnIdleTime        time.Duration `yaml:"maxConnIdleTime"`
	AllowInsecureTransport bool          `yaml:"allowInsecureTransport"`
}

func DefaultConfig() Config {
	return Config{
		ConnectTimeout:         5 * time.Second,
		ServerSelectionTimeout: 5 * time.Second,
		PingTimeout:            5 * time.Second,
		MaxPoolSize:            100,
	}
}

// Client owns both the MongoDB driver client and the selected database handle.
// The creating infrastructure layer must call Close.
type Client struct {
	driver      *mongo.Client
	database    *mongo.Database
	pingTimeout time.Duration
}

// New constructs the official MongoDB driver client and verifies connectivity
// with a primary read preference before returning it.
func New(ctx context.Context, config Config) (*Client, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}

	clientOptions := options.Client().
		ApplyURI(normalized.URI).
		SetConnectTimeout(normalized.ConnectTimeout).
		SetServerSelectionTimeout(normalized.ServerSelectionTimeout).
		SetMinPoolSize(normalized.MinPoolSize).
		SetMaxPoolSize(normalized.MaxPoolSize).
		SetMaxConnIdleTime(normalized.MaxConnIdleTime)
	if normalized.AppName != "" {
		clientOptions.SetAppName(normalized.AppName)
	}

	driver, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf(
			"mongodb: create driver client: %w",
			redactDriverError(err, normalized.URI),
		)
	}

	pingContext, cancel := context.WithTimeout(ctx, normalized.PingTimeout)
	defer cancel()
	if err := driver.Ping(pingContext, readpref.Primary()); err != nil {
		cleanupContext, cleanupCancel := context.WithTimeout(
			context.Background(),
			normalized.ConnectTimeout,
		)
		defer cleanupCancel()
		cleanupErr := driver.Disconnect(cleanupContext)
		return nil, errors.Join(
			fmt.Errorf("mongodb: ping primary: %w", redactDriverError(err, normalized.URI)),
			wrapCleanupError(cleanupErr),
		)
	}

	return &Client{
		driver:      driver,
		database:    driver.Database(normalized.Database),
		pingTimeout: normalized.PingTimeout,
	}, nil
}

func normalizeConfig(config Config) (Config, error) {
	if strings.TrimSpace(config.URI) == "" {
		return Config{}, errors.New("mongodb: URI is required")
	}
	config.Database = strings.TrimSpace(config.Database)
	if config.Database == "" {
		return Config{}, errors.New("mongodb: database is required")
	}
	if strings.ContainsRune(config.Database, '\x00') {
		return Config{}, errors.New("mongodb: database contains an invalid character")
	}
	if config.ConnectTimeout < 0 ||
		config.ServerSelectionTimeout < 0 ||
		config.PingTimeout < 0 ||
		config.MaxConnIdleTime < 0 {
		return Config{}, errors.New("mongodb: timeout cannot be negative")
	}
	if config.MaxPoolSize > 0 && config.MinPoolSize > config.MaxPoolSize {
		return Config{}, errors.New("mongodb: minimum pool size cannot exceed maximum pool size")
	}
	if err := validateURITransport(config.URI, config.AllowInsecureTransport); err != nil {
		return Config{}, err
	}

	defaults := DefaultConfig()
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = defaults.ConnectTimeout
	}
	if config.ServerSelectionTimeout == 0 {
		config.ServerSelectionTimeout = defaults.ServerSelectionTimeout
	}
	if config.PingTimeout == 0 {
		config.PingTimeout = defaults.PingTimeout
	}
	if config.MaxPoolSize == 0 {
		config.MaxPoolSize = defaults.MaxPoolSize
	}
	if config.MinPoolSize > config.MaxPoolSize {
		return Config{}, errors.New("mongodb: minimum pool size cannot exceed maximum pool size")
	}
	return config, nil
}

func validateURITransport(rawURI string, allowInsecure bool) error {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return errors.New("mongodb: URI cannot be parsed")
	}
	if parsed.Scheme != "mongodb" && parsed.Scheme != "mongodb+srv" {
		return errors.New("mongodb: URI must use mongodb or mongodb+srv")
	}
	if parsed.Host == "" {
		return errors.New("mongodb: URI host is required")
	}

	tlsConfigured := false
	tlsEnabled := parsed.Scheme == "mongodb+srv"
	for key, values := range parsed.Query() {
		if isInsecureTLSOption(key) {
			for _, value := range values {
				enabled, parseErr := strconv.ParseBool(value)
				if parseErr != nil {
					return errors.New("mongodb: URI has an invalid TLS option")
				}
				if enabled && !allowInsecure {
					return errors.New(
						"mongodb: TLS certificate and hostname verification must remain enabled",
					)
				}
			}
			continue
		}
		if !strings.EqualFold(key, "tls") && !strings.EqualFold(key, "ssl") {
			continue
		}
		tlsConfigured = true
		for _, value := range values {
			enabled, parseErr := strconv.ParseBool(value)
			if parseErr != nil {
				return errors.New("mongodb: URI has an invalid TLS option")
			}
			if !enabled {
				tlsEnabled = false
				break
			}
			tlsEnabled = true
		}
		if !tlsEnabled {
			break
		}
	}
	if parsed.Scheme == "mongodb" && !tlsConfigured {
		tlsEnabled = false
	}
	if !tlsEnabled && !allowInsecure {
		return errors.New(
			"mongodb: TLS is required; explicitly allow insecure transport for local development",
		)
	}
	return nil
}

func isInsecureTLSOption(key string) bool {
	return strings.EqualFold(key, "tlsInsecure") ||
		strings.EqualFold(key, "tlsAllowInvalidCertificates") ||
		strings.EqualFold(key, "tlsAllowInvalidHostnames")
}

func redactDriverError(err error, rawURI string) error {
	if err == nil {
		return nil
	}
	message := strings.ReplaceAll(err.Error(), rawURI, "[redacted-uri]")
	parsed, parseErr := url.Parse(rawURI)
	if parseErr == nil && parsed.User != nil {
		message = strings.ReplaceAll(message, parsed.User.String(), "[redacted-credentials]")
		if password, ok := parsed.User.Password(); ok && password != "" {
			message = strings.ReplaceAll(message, password, "[redacted-password]")
		}
	}
	return errors.New(message)
}

func wrapCleanupError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("mongodb: cleanup failed: %w", err)
}

func (client *Client) Driver() *mongo.Client {
	if client == nil {
		return nil
	}
	return client.driver
}

func (client *Client) Database() *mongo.Database {
	if client == nil {
		return nil
	}
	return client.database
}

func (client *Client) Ping(ctx context.Context) error {
	if client == nil || client.driver == nil {
		return ErrNilClient
	}
	if ctx == nil {
		return ErrNilContext
	}
	pingContext, cancel := context.WithTimeout(ctx, client.pingTimeout)
	defer cancel()
	if err := client.driver.Ping(pingContext, readpref.Primary()); err != nil {
		return fmt.Errorf("mongodb: ping primary: %w", err)
	}
	return nil
}

func (client *Client) Close(ctx context.Context) error {
	if client == nil || client.driver == nil {
		return nil
	}
	if ctx == nil {
		return ErrNilContext
	}
	if err := client.driver.Disconnect(ctx); err != nil {
		return fmt.Errorf("mongodb: disconnect: %w", err)
	}
	return nil
}
