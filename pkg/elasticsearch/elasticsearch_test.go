package elasticsearch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/zlog"
)

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
	client := &ElasticsearchClient{}
	ctx := client.appendContext(context.Background())

	request, response := formatLogMsg(ctx, []byte("request"), []byte("response"))

	require.Nil(t, request)
	require.Nil(t, response)
}

func TestAppendContextRejectsNilContext(t *testing.T) {
	client := &ElasticsearchClient{}

	require.PanicsWithValue(t, "elasticsearch: nil context", func() {
		//lint:ignore SA1012 This test verifies that nil contexts are rejected.
		client.appendContext(nil)
	})
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
