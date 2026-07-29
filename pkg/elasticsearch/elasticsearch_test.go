package elasticsearch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	elasticapi "github.com/elastic/go-elasticsearch/v9"
	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/zlog"
)

type trackingReadCloser struct {
	reader io.Reader
	reads  int
}

func (body *trackingReadCloser) Read(data []byte) (int, error) {
	body.reads++
	return body.reader.Read(data)
}

func (body *trackingReadCloser) Close() error { return nil }

func TestElasticLoggerLogRoundTripHandlesNilRequestAndResponse(t *testing.T) {
	logger := &elasticLogger{}
	transportErr := errors.New("transport failed")

	t.Run("nil request and response", func(t *testing.T) {
		assertLogRoundTripDoesNotPanic(t, func() error {
			return logger.LogRoundTrip(nil, nil, transportErr, time.Now(), time.Millisecond)
		})
	})

	t.Run("nil response", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "http://localhost:9200/index/_search", nil).
			WithContext(zlog.WithNoLog(context.Background()))

		assertLogRoundTripDoesNotPanic(t, func() error {
			return logger.LogRoundTrip(request, nil, transportErr, time.Now(), time.Millisecond)
		})
	})
}

func TestCaptureBodyPrefixPreservesCompleteBody(t *testing.T) {
	original := "sensitive-payload"
	prefix, restored := captureBodyPrefix(io.NopCloser(strings.NewReader(original)), 4)

	require.Equal(t, "sens", string(prefix))
	all, err := io.ReadAll(restored)
	require.NoError(t, err)
	require.Equal(t, original, string(all))
}

func TestElasticsearchBodyLoggingIsOffByDefault(t *testing.T) {
	logger := newLogger(0, 0)
	request, response := formatLogMsg(
		logLimits{request: logger.requestLimit, response: logger.responseLimit},
		[]byte("request"),
		[]byte("response"),
	)

	require.Nil(t, request)
	require.Nil(t, response)
}

func TestElasticLoggerDoesNotCaptureBodyForNoLogContext(t *testing.T) {
	logger := newLogger(-1, 4)
	request := httptest.NewRequest(http.MethodGet, "https://localhost:9200/index/_search", nil).
		WithContext(zlog.WithNoLog(context.Background()))
	body := &trackingReadCloser{reader: strings.NewReader("response")}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	require.NoError(t, logger.LogRoundTrip(
		request,
		response,
		nil,
		time.Now(),
		time.Millisecond,
	))

	require.Zero(t, body.reads)
	require.Same(t, body, response.Body)
}

func TestDocumentInsertSplitsBulkRequestsByDocumentCount(t *testing.T) {
	var mu sync.Mutex
	var batchSizes []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		mu.Lock()
		batchSizes = append(batchSizes, strings.Count(string(body), `"create"`))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/vnd.elasticsearch+json;compatible-with=9")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = io.WriteString(w, `{"errors":false,"took":1,"items":[]}`)
	}))
	defer server.Close()
	typedClient, err := elasticapi.NewTyped(elasticapi.WithAddresses(server.URL))
	require.NoError(t, err)
	client := &ElasticsearchClient{
		Client:       typedClient,
		bulkMaxDocs:  2,
		bulkMaxBytes: 1 << 20,
	}

	require.NoError(t, client.DocumentInsert(
		context.Background(),
		"documents",
		[]any{
			map[string]any{"value": 1},
			map[string]any{"value": 2},
			map[string]any{"value": 3},
			map[string]any{"value": 4},
			map[string]any{"value": 5},
		},
	))

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []int{2, 2, 1}, batchSizes)
}

func TestDocumentInsertRejectsDocumentLargerThanByteBudget(t *testing.T) {
	client := &ElasticsearchClient{
		bulkMaxDocs:  defaultBulkMaxDocs,
		bulkMaxBytes: 64,
	}

	err := client.DocumentInsert(
		context.Background(),
		"documents",
		[]any{map[string]any{"value": strings.Repeat("x", 128)}},
	)

	require.ErrorContains(t, err, "exceeds bulk byte limit")
}

func assertLogRoundTripDoesNotPanic(t *testing.T, logRoundTrip func() error) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("LogRoundTrip panicked: %v", recovered)
		}
	}()

	if err := logRoundTrip(); err != nil {
		t.Fatalf("LogRoundTrip returned an error: %v", err)
	}
}
