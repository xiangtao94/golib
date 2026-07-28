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

	NewJSONRenderer(nil).Failure(ctx, errors2.ErrorParamInvalid)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.NotEmpty(t, response.Header().Get(zlog.HeaderRequestID))
	require.Contains(t, response.Body.String(), `"code":2`)
	require.Contains(t, response.Body.String(), `"request_id":`)
}

func TestUnknownFailureIsInternalServerError(t *testing.T) {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewJSONRenderer(nil).Failure(ctx, errors.New("private detail"))

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), "private detail")
}

func TestRendererFactoryIsInstanceScoped(t *testing.T) {
	first := NewJSONRenderer(func() Render { return &DefaultRender{Message: "first"} })
	second := NewJSONRenderer(nil)

	require.NotSame(t, first.factory(), second.factory())
}
