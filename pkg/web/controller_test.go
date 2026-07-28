package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/zlog"
)

type controllerRequest struct {
	Name string `form:"name" binding:"required"`
}

type testController struct {
	dependency string
	requestID  *string
}

func (controller *testController) Action(ctx context.Context, request *controllerRequest) (any, error) {
	*controller.requestID = zlog.GetRequestID(ctx)
	return gin.H{"value": controller.dependency + ":" + request.Name}, nil
}

func TestHandleUsesInjectedController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var actionRequestID string
	controller := &testController{dependency: "injected", requestID: &actionRequestID}
	engine.GET("/", Handle[controllerRequest](controller))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/?name=alice", nil)
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "injected:alice")
	require.NotEmpty(t, actionRequestID)
	require.Equal(t, response.Header().Get(zlog.HeaderRequestID), actionRequestID)
}

func TestHandleRejectsNilController(t *testing.T) {
	require.Panics(t, func() {
		Handle[controllerRequest](nil)
	})
}
