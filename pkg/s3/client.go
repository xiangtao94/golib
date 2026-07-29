// Package s3 provides a provider-neutral client for Amazon S3 and
// S3-compatible object stores.
package s3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const maxPresignExpiry = 7 * 24 * time.Hour

var baseHTTPTransport = http.DefaultTransport.(*http.Transport).Clone()

// Config configures an S3 client. Endpoint is optional for AWS and required
// for S3-compatible services such as MinIO, RustFS, GCS XML API, and Aliyun
// OSS. When static credentials are omitted, the AWS default credential chain
// is used.
type Config struct {
	Endpoint        string      `yaml:"endpoint"`
	Region          string      `yaml:"region"`
	AccessKeyID     string      `yaml:"accessKeyID"`
	SecretAccessKey string      `yaml:"secretAccessKey"`
	SessionToken    string      `yaml:"sessionToken"`
	UsePathStyle    bool        `yaml:"usePathStyle"`
	AllowHTTP       bool        `yaml:"allowHTTP"`
	TLSConfig       *tls.Config `yaml:"-"`
}

// ObjectInfo contains provider-neutral object metadata.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
	Metadata     map[string]string
	VersionID    string
	StorageClass string
	IsPrefix     bool
}

// Client owns an AWS SDK S3 client and its HTTP connection pool.
type Client struct {
	api       *awss3.Client
	presigner *awss3.PresignClient
	transfers *transfermanager.Client
	transport *http.Transport
	config    Config
}

// NewClient creates a client for AWS S3 or an S3-compatible endpoint.
func NewClient(ctx context.Context, config Config) (*Client, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	transport := baseHTTPTransport.Clone()
	tlsConfig := config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	transport.TLSClientConfig = tlsConfig

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithHTTPClient(&http.Client{Transport: transport}),
	}
	if config.Region != "" {
		loadOptions = append(loadOptions, awsconfig.WithRegion(config.Region))
	}
	if config.AccessKeyID != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				config.AccessKeyID,
				config.SecretAccessKey,
				config.SessionToken,
			),
		))
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, fmt.Errorf("s3: load AWS configuration: %w", err)
	}
	if awsConfig.Region == "" {
		transport.CloseIdleConnections()
		return nil, errors.New(
			"s3: region is required in Config or the AWS default configuration chain",
		)
	}
	// Optional request checksums are not implemented consistently by all
	// S3-compatible providers. Required checksums remain enabled.
	awsConfig.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	awsConfig.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired

	api := awss3.NewFromConfig(awsConfig, func(options *awss3.Options) {
		options.UsePathStyle = config.UsePathStyle
		if config.Endpoint != "" {
			options.BaseEndpoint = new(strings.TrimRight(config.Endpoint, "/"))
		}
	})

	return &Client{
		api:       api,
		presigner: awss3.NewPresignClient(api),
		transfers: transfermanager.New(api),
		transport: transport,
		config:    config,
	}, nil
}

func validateConfig(config Config) error {
	hasAccessKey := config.AccessKeyID != ""
	hasSecretKey := config.SecretAccessKey != ""
	if hasAccessKey != hasSecretKey {
		return errors.New("s3: static credentials require both accessKeyID and secretAccessKey")
	}
	if config.SessionToken != "" && !hasAccessKey {
		return errors.New("s3: sessionToken requires static credentials")
	}
	if config.Endpoint == "" {
		return nil
	}

	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return fmt.Errorf("s3: invalid endpoint: %w", err)
	}
	if endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		return errors.New("s3: endpoint must be an absolute HTTP or HTTPS URL")
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("s3: endpoint must not contain credentials, query parameters, or a fragment")
	}
	if endpoint.Scheme == "http" && !config.AllowHTTP {
		return errors.New("s3: HTTPS is required; explicitly set allowHTTP for an HTTP endpoint")
	}
	return nil
}

// Close releases idle HTTP connections owned by the client.
func (client *Client) Close() {
	if client != nil && client.transport != nil {
		client.transport.CloseIdleConnections()
	}
}

// PresignGetObject creates a temporary URL for downloading an object.
func (client *Client) PresignGetObject(
	ctx context.Context,
	bucket string,
	key string,
	expiry time.Duration,
) (string, error) {
	if err := validateObjectAddress(bucket, key); err != nil {
		return "", err
	}
	if err := validatePresignExpiry(expiry); err != nil {
		return "", err
	}

	result, err := client.presigner.PresignGetObject(
		ctx,
		&awss3.GetObjectInput{
			Bucket: new(bucket),
			Key:    new(key),
		},
		awss3.WithPresignExpires(expiry),
	)
	if err != nil {
		return "", fmt.Errorf("s3: presign GET %s/%s: %w", bucket, key, err)
	}
	return result.URL, nil
}

// PresignPutObject creates a temporary URL for uploading an object.
func (client *Client) PresignPutObject(
	ctx context.Context,
	bucket string,
	key string,
	expiry time.Duration,
) (string, error) {
	if err := validateObjectAddress(bucket, key); err != nil {
		return "", err
	}
	if err := validatePresignExpiry(expiry); err != nil {
		return "", err
	}

	result, err := client.presigner.PresignPutObject(
		ctx,
		&awss3.PutObjectInput{
			Bucket: new(bucket),
			Key:    new(key),
		},
		awss3.WithPresignExpires(expiry),
	)
	if err != nil {
		return "", fmt.Errorf("s3: presign PUT %s/%s: %w", bucket, key, err)
	}
	return result.URL, nil
}

// StatObject returns metadata for an object.
func (client *Client) StatObject(
	ctx context.Context,
	bucket string,
	key string,
) (ObjectInfo, error) {
	if err := validateObjectAddress(bucket, key); err != nil {
		return ObjectInfo{}, err
	}

	result, err := client.api.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: new(bucket),
		Key:    new(key),
	})
	if err != nil {
		return ObjectInfo{}, wrapOperationError("stat", bucket, key, err)
	}

	return ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(result.ContentLength),
		LastModified: aws.ToTime(result.LastModified),
		ContentType:  aws.ToString(result.ContentType),
		ETag:         trimETag(aws.ToString(result.ETag)),
		Metadata:     cloneMetadata(result.Metadata),
		VersionID:    aws.ToString(result.VersionId),
		StorageClass: string(result.StorageClass),
	}, nil
}

func validateObjectAddress(bucket string, key string) error {
	if strings.TrimSpace(bucket) == "" {
		return errors.New("s3: bucket is required")
	}
	if key == "" {
		return errors.New("s3: object key is required")
	}
	return nil
}

func validatePresignExpiry(expiry time.Duration) error {
	if expiry <= 0 || expiry > maxPresignExpiry {
		return fmt.Errorf("s3: presign expiry must be between 1ns and %s", maxPresignExpiry)
	}
	return nil
}

func trimETag(etag string) string {
	return strings.Trim(etag, `"`)
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	return maps.Clone(metadata)
}
