package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPutObjectMapsGenericOptionsAndResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", request.Method)
		}
		if request.URL.EscapedPath() != "/test-bucket/reports/july.txt" {
			t.Fatalf("path = %q", request.URL.EscapedPath())
		}
		if request.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Fatalf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		if request.Header.Get("X-Amz-Meta-Owner") != "finance" {
			t.Fatalf("X-Amz-Meta-Owner = %q", request.Header.Get("X-Amz-Meta-Owner"))
		}
		if request.Header.Get("X-Amz-Server-Side-Encryption") != "AES256" {
			t.Fatalf(
				"X-Amz-Server-Side-Encryption = %q",
				request.Header.Get("X-Amz-Server-Side-Encryption"),
			)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != "hello s3" {
			t.Fatalf("body = %q", body)
		}

		w.Header().Set("ETag", `"etag-1"`)
		w.Header().Set("X-Amz-Version-Id", "version-7")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.PutObject(
		context.Background(),
		"test-bucket",
		"reports/july.txt",
		strings.NewReader("hello s3"),
		int64(len("hello s3")),
		&UploadOptions{
			Metadata:             map[string]string{"owner": "finance"},
			ServerSideEncryption: EncryptionAES256,
		},
	)
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if result.Key != "reports/july.txt" ||
		result.Size != int64(len("hello s3")) ||
		result.ETag != "etag-1" ||
		result.VersionID != "version-7" {
		t.Fatalf("PutObject() result = %+v", result)
	}
}

func TestPutObjectStreamsKnownModerateObjectAsSingleRequest(t *testing.T) {
	const objectSize = int64(20 << 20)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", request.Method)
		}
		query := request.URL.Query()
		if query.Has("uploads") || query.Has("uploadId") || query.Has("partNumber") {
			t.Errorf("query = %q, want no multipart parameters", request.URL.RawQuery)
		}
		written, err := io.Copy(io.Discard, request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if written != objectSize {
			t.Errorf("request body size = %d, want %d", written, objectSize)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(context.Background(), Config{
		Endpoint:                      server.URL,
		Region:                        "us-east-1",
		AccessKeyID:                   "access-key",
		SecretAccessKey:               "secret-key",
		UsePathStyle:                  true,
		AllowHTTP:                     true,
		SinglePutThreshold:            32 << 20,
		UploadPartSize:                5 << 20,
		UploadPartConcurrency:         1,
		MaxConcurrentMultipartUploads: 1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)

	_, err = client.PutObject(
		context.Background(),
		"test-bucket",
		"large.bin",
		io.LimitReader(zeroReader{}, objectSize),
		objectSize,
		nil,
	)
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("request calls = %d, want 1", got)
	}
}

func TestPutObjectHonorsContextWhileWaitingForMultipartSlot(t *testing.T) {
	client := &Client{
		config:         Config{SinglePutThreshold: 1},
		multipartSlots: make(chan struct{}, 1),
	}
	client.multipartSlots <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.PutObject(
		ctx,
		"test-bucket",
		"large.bin",
		strings.NewReader("large"),
		5,
		nil,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PutObject() error = %v, want context.Canceled", err)
	}
}

func TestGetObjectReturnsBodyAndMetadataFromOneRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", request.Method)
		}
		w.Header().Set("Content-Length", "7")
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("ETag", `"etag-2"`)
		w.Header().Set("X-Amz-Meta-Owner", "legal")
		w.Header().Set("X-Amz-Version-Id", "version-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "content")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	object, err := client.GetObject(
		context.Background(),
		"test-bucket",
		"contracts/a.txt",
	)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	defer object.Body.Close()

	body, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("read object body: %v", err)
	}
	if string(body) != "content" {
		t.Fatalf("body = %q, want content", body)
	}
	if calls.Load() != 1 {
		t.Fatalf("request calls = %d, want 1", calls.Load())
	}
	if object.Info.Key != "contracts/a.txt" ||
		object.Info.Size != 7 ||
		object.Info.ETag != "etag-2" ||
		object.Info.Metadata["owner"] != "legal" ||
		object.Info.VersionID != "version-8" {
		t.Fatalf("GetObject() info = %+v", object.Info)
	}
}

func TestObjectExistsMapsNotFoundWithoutReturningAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	exists, err := client.ObjectExists(
		context.Background(),
		"test-bucket",
		"missing.txt",
	)
	if err != nil {
		t.Fatalf("ObjectExists() error = %v", err)
	}
	if exists {
		t.Fatal("ObjectExists() = true, want false")
	}
}

func TestStatObjectExposesProviderNeutralNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.StatObject(
		context.Background(),
		"test-bucket",
		"missing.txt",
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("StatObject() error = %v, want ErrNotFound", err)
	}
}

func TestStatObjectExposesProviderNeutralAccessDeniedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.StatObject(
		context.Background(),
		"test-bucket",
		"private.txt",
	)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("StatObject() error = %v, want ErrAccessDenied", err)
	}
}

func TestListObjectsFollowsContinuationTokensAndPreservesPrefixes(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		call := calls.Add(1)
		if request.URL.Query().Get("list-type") != "2" {
			t.Fatalf("list-type = %q", request.URL.Query().Get("list-type"))
		}
		if request.URL.Query().Get("prefix") != "reports/" {
			t.Fatalf("prefix = %q", request.URL.Query().Get("prefix"))
		}
		if request.URL.Query().Get("delimiter") != "/" {
			t.Fatalf("delimiter = %q", request.URL.Query().Get("delimiter"))
		}
		w.Header().Set("Content-Type", "application/xml")

		if call == 1 {
			_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>reports/</Prefix>
  <KeyCount>2</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>next-page</NextContinuationToken>
  <Contents>
    <Key>reports/a.txt</Key>
    <LastModified>2026-07-28T01:02:03.000Z</LastModified>
    <ETag>&quot;etag-a&quot;</ETag>
    <Size>3</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
  <CommonPrefixes><Prefix>reports/archive/</Prefix></CommonPrefixes>
</ListBucketResult>`)
			return
		}

		if request.URL.Query().Get("continuation-token") != "next-page" {
			t.Fatalf(
				"continuation-token = %q",
				request.URL.Query().Get("continuation-token"),
			)
		}
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>reports/</Prefix>
  <KeyCount>1</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>reports/z.txt</Key>
    <LastModified>2026-07-28T02:03:04.000Z</LastModified>
    <ETag>&quot;etag-z&quot;</ETag>
    <Size>9</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListBucketResult>`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	var objects []ObjectInfo
	for object, err := range client.ListObjects(
		context.Background(),
		"test-bucket",
		ListOptions{Prefix: "reports/"},
	) {
		if err != nil {
			t.Fatalf("ListObjects() error = %v", err)
		}
		objects = append(objects, object)
	}
	if calls.Load() != 2 {
		t.Fatalf("request calls = %d, want 2", calls.Load())
	}
	if len(objects) != 3 {
		t.Fatalf("len(objects) = %d, want 3: %+v", len(objects), objects)
	}
	if objects[0].Key != "reports/a.txt" || objects[0].ETag != "etag-a" {
		t.Fatalf("objects[0] = %+v", objects[0])
	}
	if objects[1].Key != "reports/archive/" || !objects[1].IsPrefix {
		t.Fatalf("objects[1] = %+v", objects[1])
	}
	if objects[2].Key != "reports/z.txt" || objects[2].Size != 9 {
		t.Fatalf("objects[2] = %+v", objects[2])
	}
}

func TestListObjectsStopsFetchingWhenCallerBreaks(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/xml")
		if call > 1 {
			t.Fatal("iterator fetched a page after caller stopped")
		}
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <KeyCount>1</KeyCount>
  <MaxKeys>1</MaxKeys>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>next-page</NextContinuationToken>
  <Contents><Key>first.txt</Key><Size>1</Size></Contents>
</ListBucketResult>`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	for object, err := range client.ListObjects(
		context.Background(),
		"test-bucket",
		ListOptions{PageSize: 1},
	) {
		if err != nil {
			t.Fatalf("ListObjects() error = %v", err)
		}
		if object.Key != "first.txt" {
			t.Fatalf("object key = %q", object.Key)
		}
		break
	}
	if calls.Load() != 1 {
		t.Fatalf("request calls = %d, want 1", calls.Load())
	}
}

func TestListObjectsYieldsValidationErrorWithoutRequest(t *testing.T) {
	client := &Client{}
	var errorsSeen []error

	for _, err := range client.ListObjects(
		context.Background(),
		"",
		ListOptions{},
	) {
		errorsSeen = append(errorsSeen, err)
	}

	if len(errorsSeen) != 1 || errorsSeen[0] == nil {
		t.Fatalf("validation errors = %v", errorsSeen)
	}
}

func TestCopyObjectEscapesTheSourceAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", request.Method)
		}
		if request.URL.EscapedPath() != "/destination/copied/report.txt" {
			t.Fatalf("path = %q", request.URL.EscapedPath())
		}
		copySource := request.Header.Get("X-Amz-Copy-Source")
		if copySource != "source%2Freports%2F2026%20july.txt" {
			t.Fatalf("X-Amz-Copy-Source = %q", copySource)
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <ETag>&quot;copy-etag&quot;</ETag>
  <LastModified>2026-07-28T02:03:04.000Z</LastModified>
</CopyObjectResult>`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.CopyObject(
		context.Background(),
		"source",
		"reports/2026 july.txt",
		"destination",
		"copied/report.txt",
	)
	if err != nil {
		t.Fatalf("CopyObject() error = %v", err)
	}
	if result.ETag != "copy-etag" {
		t.Fatalf("CopyObject() result = %+v", result)
	}
}

func TestCreateBucketChecksForExistenceBeforeCreating(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		call := calls.Add(1)
		switch call {
		case 1:
			if request.Method != http.MethodHead {
				t.Fatalf("first method = %s, want HEAD", request.Method)
			}
			w.WriteHeader(http.StatusNotFound)
		case 2:
			if request.Method != http.MethodPut {
				t.Fatalf("second method = %s, want PUT", request.Method)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read create bucket body: %v", err)
			}
			if len(bytes.TrimSpace(body)) != 0 {
				t.Fatalf("custom endpoint create bucket body = %q, want empty", body)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request %d: %s", call, request.Method)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.CreateBucket(context.Background(), "new-bucket"); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("request calls = %d, want 2", calls.Load())
	}
}

func TestGetFileDoesNotLeavePartialDestinationOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "12")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "short")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	destination := filepath.Join(t.TempDir(), "download.txt")
	err := client.GetFile(
		context.Background(),
		"test-bucket",
		"download.txt",
		destination,
	)
	if err == nil {
		t.Fatal("GetFile() error = nil, want truncated response error")
	}
	if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want os.ErrNotExist", statErr)
	}
}

func TestGetFileAtomicallyReplacesTheDestinationAfterSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "new data")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	destination := filepath.Join(t.TempDir(), "download.txt")
	if err := os.WriteFile(destination, []byte("old data"), 0o600); err != nil {
		t.Fatalf("write existing destination: %v", err)
	}

	err := client.GetFile(
		context.Background(),
		"test-bucket",
		"download.txt",
		destination,
	)
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(content) != "new data" {
		t.Fatalf("destination content = %q, want new data", content)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".*.part"))
	if err != nil {
		t.Fatalf("glob temporary files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestDeleteObjectUsesAnIdempotentDeleteRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", request.Method)
		}
		if request.URL.EscapedPath() != "/test-bucket/old/report.txt" {
			t.Fatalf("path = %q", request.URL.EscapedPath())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.DeleteObject(
		context.Background(),
		"test-bucket",
		"old/report.txt",
	); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}
}

func newTestClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	client, err := NewClient(context.Background(), Config{
		Endpoint:        endpoint,
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
	return client
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}
