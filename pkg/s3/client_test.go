package s3

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewClientRequiresHTTPSByDefault(t *testing.T) {
	_, err := NewClient(context.Background(), Config{
		Endpoint:        "http://127.0.0.1:9000",
		Region:          "us-east-1",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("NewClient() error = %v, want HTTPS requirement", err)
	}
}

func TestNewClientRejectsNilContext(t *testing.T) {
	//lint:ignore SA1012 This test verifies that nil contexts are rejected.
	_, err := NewClient(nil, Config{})
	if !errors.Is(err, ErrNilContext) {
		t.Fatalf("NewClient() error = %v, want ErrNilContext", err)
	}
}

func TestNewClientRejectsIncompleteStaticCredentials(t *testing.T) {
	_, err := NewClient(context.Background(), Config{
		Region:      "us-east-1",
		AccessKeyID: "access-key",
	})
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("NewClient() error = %v, want incomplete credentials error", err)
	}
}

func TestNormalizeConfigBoundsUploadConcurrency(t *testing.T) {
	config := normalizeConfig(Config{})

	if config.SinglePutThreshold != defaultSinglePutThreshold ||
		config.UploadPartSize != defaultUploadPartSize ||
		config.UploadPartConcurrency != defaultUploadPartConcurrency ||
		config.MaxConcurrentMultipartUploads != defaultConcurrentMultipart {
		t.Fatalf("normalizeConfig() = %+v, want upload defaults", config)
	}
}

func TestValidateConfigRejectsUploadPartsBelowProviderMinimum(t *testing.T) {
	err := validateConfig(Config{UploadPartSize: minUploadPartSize - 1})

	if err == nil || !strings.Contains(err.Error(), "uploadPartSize") {
		t.Fatalf("validateConfig() error = %v, want uploadPartSize error", err)
	}
}

func TestNewClientRequiresAResolvedRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/missing-config")

	_, err := NewClient(context.Background(), Config{
		Endpoint:        "https://storage.example.com",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	})
	if err == nil || !strings.Contains(err.Error(), "region is required") {
		t.Fatalf("NewClient() error = %v, want missing region error", err)
	}
}

func TestNewClientDoesNotMutateTLSConfig(t *testing.T) {
	source := &tls.Config{}

	client, err := NewClient(context.Background(), Config{
		Endpoint:        "https://storage.example.com",
		Region:          "us-east-1",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		TLSConfig:       source,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)

	if source.MinVersion != 0 {
		t.Fatalf("source TLS MinVersion = %d, want 0", source.MinVersion)
	}
}

func TestPresignGetObjectUsesConfiguredEndpointAndPathStyle(t *testing.T) {
	client, err := NewClient(context.Background(), Config{
		Endpoint:        "https://storage.example.com",
		Region:          "us-east-1",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)

	rawURL, err := client.PresignGetObject(
		context.Background(),
		"test-bucket",
		"reports/2026 july.txt",
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("PresignGetObject() error = %v", err)
	}

	signedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse signed URL: %v", err)
	}
	if signedURL.Host != "storage.example.com" {
		t.Fatalf("signed URL host = %q, want storage.example.com", signedURL.Host)
	}
	if signedURL.EscapedPath() != "/test-bucket/reports/2026%20july.txt" {
		t.Fatalf("signed URL path = %q", signedURL.EscapedPath())
	}
	if signedURL.Query().Get("X-Amz-Expires") != "900" {
		t.Fatalf("X-Amz-Expires = %q, want 900", signedURL.Query().Get("X-Amz-Expires"))
	}
}

func TestStatObjectSendsSignedPathStyleRequest(t *testing.T) {
	var request *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r.Clone(r.Context())
		w.Header().Set("Content-Length", "12")
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Last-Modified", "Tue, 28 Jul 2026 01:02:03 GMT")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(context.Background(), Config{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		UsePathStyle:    true,
		AllowHTTP:       true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)

	info, err := client.StatObject(context.Background(), "test-bucket", "notes/a.txt")
	if err != nil {
		t.Fatalf("StatObject() error = %v", err)
	}

	if request == nil {
		t.Fatal("StatObject() did not send a request")
	}
	if request.Method != http.MethodHead {
		t.Fatalf("request method = %s, want HEAD", request.Method)
	}
	if request.URL.EscapedPath() != "/test-bucket/notes/a.txt" {
		t.Fatalf("request path = %q", request.URL.EscapedPath())
	}
	if !strings.Contains(request.Header.Get("Authorization"), "Credential=access-key/") {
		t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
	}
	if info.Key != "notes/a.txt" || info.Size != 12 || info.ETag != "abc123" {
		t.Fatalf("StatObject() info = %+v", info)
	}
	if info.ContentType != "text/plain" {
		t.Fatalf("ContentType = %q, want text/plain", info.ContentType)
	}
}
