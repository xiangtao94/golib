package render

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	errors2 "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func TestFailureUsesHTTPStatusAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewJSONRenderer(nil).Failure(ctx, errors2.ErrInvalidArgument)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.NotEmpty(t, response.Header().Get(zlog.HeaderRequestID))
	require.Contains(t, response.Body.String(), `"code":"INVALID_ARGUMENT"`)
	require.Contains(t, response.Body.String(), `"reason":"INVALID_ARGUMENT"`)
	require.Contains(t, response.Body.String(), `"request_id":`)
}

func TestUnknownFailureIsInternalServerError(t *testing.T) {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewJSONRenderer(nil).Failure(ctx, errors.New("private detail"))

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), "private detail")
	require.Contains(t, response.Body.String(), `"code":"INTERNAL"`)
}

func TestRendererFactoryIsInstanceScoped(t *testing.T) {
	first := NewJSONRenderer(func(response Response) any {
		return map[string]any{"custom": response.Message}
	})
	second := NewJSONRenderer(nil)
	input := Response{Message: "first"}

	require.Equal(t, map[string]any{"custom": "first"}, first.factory(input))
	require.Equal(t, input, second.factory(input))
}

func TestFailureRendersStableDomainContract(t *testing.T) {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	domainErr := errors2.New("USER_NOT_FOUND", "user not found", http.StatusNotFound).
		WithReason("NOT_FOUND").
		WithDetails(map[string]any{"field": "user_id"})

	NewJSONRenderer(nil).Failure(ctx, domainErr)

	require.Equal(t, http.StatusNotFound, response.Code)
	require.JSONEq(t, `{
		"code": "USER_NOT_FOUND",
		"reason": "NOT_FOUND",
		"message": "user not found",
		"request_id": "`+response.Header().Get(zlog.HeaderRequestID)+`",
		"retryable": false,
		"details": {"field": "user_id"}
	}`, response.Body.String())
}
