package flow

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
	"github.com/xiangtao94/golib/pkg/zlog"
)

// IController is deliberately independent of Gin. The HTTP adapter passes the
// request context and owns binding and rendering.
type IController[T any] interface {
	Action(context.Context, *T) (any, error)
}

// Controller is the business-facing extension point consumed by Use.
// IController remains as a compatibility name for existing applications.
type Controller[T any] = IController[T]

type ControllerFactory[T any] func() IController[T]

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

// Use adapts an explicitly constructed controller to Gin. The factory is
// invoked once per request, preserving constructor-injected dependencies.
func Use[T any](factory ControllerFactory[T], options ...ControllerOption) gin.HandlerFunc {
	if factory == nil {
		panic("flow: nil controller factory")
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
		controller := factory()
		if controller == nil {
			zlog.Error(ginCtx, "controller factory returned nil")
			config.renderer.Failure(ginCtx, errors.ErrorSystemError)
			return
		}

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

		requestContext := zlog.WithRequestID(ginCtx.Request.Context(), zlog.GetRequestID(ginCtx))
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
