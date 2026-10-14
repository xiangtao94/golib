package web_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	serviceerrors "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/web"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func returnPrivateFailure(*gin.Context, *struct{}) (any, error) {
	return nil, fmt.Errorf("delivery: %w", errors.Join(
		serviceerrors.ErrInternal.Wrap(errors.New("smtp authentication failed")),
		serviceerrors.ErrInternal.Wrap(errors.New("redis cleanup failed")),
	))
}

func TestHandleLogsPrivateCausesWithoutExposingThem(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			_, err := zlog.InitLog(zlog.LogConfig{AppName: "web-test", Format: format, LogToFile: true, LogDir: dir})
			require.NoError(t, err)
			t.Cleanup(func() { _ = zlog.CloseLogger() })
			engine := gin.New() // ContextWithFallback stays disabled, as in Gin's default.
			engine.POST("/code", web.Handle(returnPrivateFailure))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/code", nil)
			request.Header.Set(zlog.HeaderRequestID, "test-private-error")
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusInternalServerError, response.Code)
			require.NotContains(t, response.Body.String(), "smtp")
			require.NotContains(t, response.Body.String(), "redis")
			require.NotContains(t, response.Body.String(), "wrapped at")
			require.NotContains(t, response.Body.String(), "stacktrace")
			require.Contains(t, response.Body.String(), "internal server error")
			require.NoError(t, zlog.CloseLogger())
			data, err := os.ReadFile(filepath.Join(dir, "web-test.log.wf"))
			require.NoError(t, err)
			log := string(data)
			require.Contains(t, log, "smtp authentication failed")
			require.Contains(t, log, "redis cleanup failed")
			require.Contains(t, log, "test-private-error")
			require.Contains(t, log, "web_test.returnPrivateFailure")
			require.Contains(t, log, "/code")
			if format == "json" {
				var entry map[string]any
				require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(log)), &entry))
				require.Equal(t, "test-private-error", entry["requestId"])
				require.Contains(t, entry["stacktrace"], "web.Handle")
				require.Contains(t, entry["error"], "wrapped at:")
			}
		})
	}
}
