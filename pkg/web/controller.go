package web

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
	"github.com/xiangtao94/golib/pkg/zlog"
)

// Controller is deliberately independent of Gin. The HTTP adapter passes the
// request context and owns binding and rendering.
type Controller[T any] interface {
	Action(context.Context, *T) (any, error)
}

type controllerConfig struct {
	binding  binding.Binding
	renderer *render.JSONRenderer
}

type ControllerOption func(*controllerConfig)

func WithBinding(requestBinding binding.Binding) ControllerOption {
	return func(config *controllerConfig) {
		config.binding = requestBinding
	}
}

func WithRenderer(renderer *render.JSONRenderer) ControllerOption {
	return func(config *controllerConfig) {
		config.renderer = renderer
	}
}

type renderPolicy interface {
	ShouldRender() bool
}

// Handle adapts a constructed business controller to Gin. Controllers should
// hold only constructor-injected dependencies; request state arrives through
// Action parameters.
func Handle[T any](controller Controller[T], options ...ControllerOption) gin.HandlerFunc {
	if controller == nil {
		panic("web: nil controller")
	}
	config := controllerConfig{
		binding:  binding.Form,
		renderer: render.NewJSONRenderer(nil),
	}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}

	return func(ginCtx *gin.Context) {
		requestContext, _ := zlog.EnsureRequestID(
			ginCtx.Request.Context(),
			ginCtx.GetHeader(zlog.HeaderRequestID),
		)
		ginCtx.Request = ginCtx.Request.WithContext(requestContext)

		var request T
		var err error
		if ginCtx.GetHeader("Content-Type") == "" {
			err = ginCtx.ShouldBindWith(&request, config.binding)
		} else {
			err = ginCtx.ShouldBind(&request)
		}
		if err != nil {
			zlog.Errorf(ginCtx, "controller %T parameter binding failed: %v", controller, err)
			config.renderer.Failure(ginCtx, errors.ErrorParamInvalid)
			return
		}

		data, err := controller.Action(requestContext, &request)
		if err != nil {
			zlog.Errorf(ginCtx, "controller %T action failed: %v", controller, err)
			config.renderer.Failure(ginCtx, err)
			return
		}
		if policy, ok := controller.(renderPolicy); !ok || policy.ShouldRender() {
			config.renderer.Success(ginCtx, data)
		}
	}
}
