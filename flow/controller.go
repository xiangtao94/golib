package flow

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
	"github.com/xiangtao94/golib/pkg/zlog"
	"reflect"
)

type IController[T any] interface {
	ILayer
	Action(req *T) (any, error)
	ShouldRender() bool
	RequestBind() binding.Binding
	SetTrace(traceId string)
	RenderJsonFail(err error)
	RenderJsonSuccess(data any)
}

type Controller struct {
	Layer
}

// 默认实现，建议具体业务Controller重写
func (c *Controller) Action(req *any) (any, error) {
	panic("implement me")
}

// 手动设置traceId
func (c *Controller) SetTrace(traceId string) {
	if traceId == "" {
		zlog.Warnf(c.ctx, "[controller] set trace failed, traceId is empty")
		return
	}
	c.GetCtx().Set(zlog.ContextKeyRequestID, traceId)
}

// 默认使用 Form 绑定
func (c *Controller) RequestBind() binding.Binding {
	return binding.Form
}

func (c *Controller) ShouldRender() bool {
	return true
}

func (c *Controller) RenderJsonFail(err error) {
	render.RenderJsonFail(c.GetCtx(), err)
}

func (c *Controller) RenderJsonSuccess(data any) {
	render.RenderJsonSucc(c.GetCtx(), data)
}

// clone Controller 实例（浅复制）
// 这里改为用 reflect 创建新实例，避免指针类型判断复杂性
func cloneController[T any](ctl IController[T]) IController[T] {
	typ := reflect.TypeOf(ctl)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	v := reflect.New(typ).Interface()
	newCtl, ok := v.(IController[T])
	if !ok {
		panic("cloneController: type does not implement IController[T]")
	}
	return newCtl
}

// Gin Handler
func Use[T any](ctl IController[T]) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		newCtl := cloneController(ctl)
		newCtl.SetCtx(ctx)
		newCtl.SetEntity(newCtl)

		var req T
		contentType := ctx.GetHeader("Content-Type")

		var err error
		if contentType == "" {
			// 无 Content-Type，使用 Controller 自定义的绑定器
			err = ctx.ShouldBindWith(&req, newCtl.RequestBind())
		} else {
			err = ctx.ShouldBind(&req)
		}

		if err != nil {
			zlog.Errorf(newCtl.GetCtx(), "Controller %T param bind error: %v", newCtl, err)
			newCtl.RenderJsonFail(errors.ErrorParamInvalid)
			return
		}

		data, err := newCtl.Action(&req)
		if err != nil {
			zlog.Errorf(newCtl.GetCtx(), "Controller %T call action error: %v", newCtl, err)
			newCtl.RenderJsonFail(err)
			return
		}

		if newCtl.ShouldRender() {
			newCtl.RenderJsonSuccess(data)
		}
	}
}
