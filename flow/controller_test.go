package flow

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

type factoryController struct {
	dependency string
	requestID  *string
}

func (controller *factoryController) Action(ctx context.Context, request *controllerRequest) (any, error) {
	*controller.requestID = zlog.GetRequestID(ctx)
	return gin.H{"value": controller.dependency + ":" + request.Name}, nil
}

func TestUseConstructsControllerThroughFactory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var actionRequestID string
	engine.GET("/", Use(func() Controller[controllerRequest] {
		return &factoryController{dependency: "injected", requestID: &actionRequestID}
	}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/?name=alice", nil)
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "injected:alice")
	require.NotEmpty(t, actionRequestID)
	require.Equal(t, response.Header().Get(zlog.HeaderRequestID), actionRequestID)
}
