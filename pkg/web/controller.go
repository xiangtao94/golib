package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type handlerConfig struct {
	binding       binding.Binding
	renderer      *render.JSONRenderer
	successStatus int
}

type Option func(*handlerConfig)

// WithBinding overrides automatic binding, regardless of Content-Type.
func WithBinding(requestBinding binding.Binding) Option {
	return func(config *handlerConfig) {
		config.binding = requestBinding
	}
}

func WithRenderer(renderer *render.JSONRenderer) Option {
	return func(config *handlerConfig) {
		config.renderer = renderer
	}
}

// WithSuccessStatus sets the HTTP status for successful controller responses.
func WithSuccessStatus(status int) Option {
	return func(config *handlerConfig) {
		config.successStatus = status
	}
}

// Handle binds a request, invokes action, and renders its JSON result.
// Actions may read HTTP metadata; services receive ginCtx.Request.Context().
// HTML and streaming endpoints use Gin handlers and render directly.
func Handle[T any](action func(*gin.Context, *T) (any, error), options ...Option) gin.HandlerFunc {
	if action == nil {
		panic("web: nil action")
	}
	config := handlerConfig{
		renderer:      render.NewJSONRenderer(nil),
		successStatus: http.StatusOK,
	}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.successStatus < 200 || config.successStatus >= 300 || config.successStatus == http.StatusNoContent || config.successStatus == http.StatusResetContent {
		panic("web: success status must allow a response body")
	}

	return func(ginCtx *gin.Context) {
		requestContext, _ := zlog.EnsureRequestID(
			ginCtx.Request.Context(),
			ginCtx.GetHeader(zlog.HeaderRequestID),
		)
		ginCtx.Request = ginCtx.Request.WithContext(requestContext)

		var request T
		var err error
		if config.binding != nil {
			err = ginCtx.ShouldBindWith(&request, config.binding)
		} else {
			err = ginCtx.ShouldBind(&request)
		}
		if err != nil {
			zlog.ErrorLogger(requestContext, "handler parameter binding failed",
				zlog.String("path", ginCtx.FullPath()), zlog.String("error", errors.Diagnostic(err)))
			config.renderer.Failure(ginCtx, errors.ErrInvalidArgument)
			return
		}

		data, err := action(ginCtx, &request)
		if err != nil {
			zlog.ErrorLogger(requestContext, "handler action failed",
				zlog.String("path", ginCtx.FullPath()), zlog.String("error", errors.Diagnostic(err)))
			config.renderer.Failure(ginCtx, err)
			return
		}
		config.renderer.SuccessWithStatus(ginCtx, config.successStatus, data)
	}
}
