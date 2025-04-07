package flow

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	errors2 "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net/http"
	"reflect"
)

type IController[T any] interface {
	ILayer
	Action(req *T) (any, error)
	ShouldRender() bool
	SetTrace(traceId string)
	RenderJsonFail(err error)
	RenderJsonSuccess(data any)
}

type Controller struct {
	Layer
}

func (entity *Controller) Action(req *any) (any, error) {
	//TODO implement me
	panic("implement me")
}

// 手动设置requestId
func (entity *Controller) SetTrace(traceId string) {
	if traceId == "" {
		zlog.Warnf(entity.ctx, "[controller] set trace failed, traceId is empty")
		return
	}
	entity.GetCtx().Set(zlog.ContextKeyRequestID, traceId)
}

func (entity *Controller) ShouldRender() bool {
	return true
}

func (entity *Controller) RenderJsonFail(err error) {
	render.RenderJsonFail(entity.GetCtx(), err)
}

func (entity *Controller) RenderJsonSuccess(data any) {
	render.RenderJsonSucc(entity.GetCtx(), data)
}

func slave(src any) any {
	typ := reflect.TypeOf(src)
	if typ.Kind() == reflect.Ptr { //如果是指针类型
		typ = typ.Elem()               //获取源实际类型(否则为指针类型)
		dst := reflect.New(typ).Elem() //创建对象
		return dst.Addr().Interface()  //返回指针
	} else {
		dst := reflect.New(typ).Elem() //创建对象
		return dst.Interface()         //返回值
	}
}

func Use[T any](ctl IController[T]) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		newCTL := slave(ctl).(IController[T])
		newCTL.SetCtx(ctx)
		newCTL.SetEntity(newCTL)
		// 处理请求序列化
		var newReq T
		method := ctx.Request.Method
		var err error
		if len(ctx.ContentType()) == 0 { // 针对无 Content-Type 的请求特殊处理
			switch method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				// 优先尝试 JSON 绑定
				if err = ctx.ShouldBindBodyWith(&newReq, binding.JSON); err == nil {
					break
				}
				// 尝试 Form 绑定
				err = ctx.ShouldBindWith(&newReq, binding.Form)
			default:
				if len(ctx.Params) > 0 {
					err = ctx.ShouldBindUri(&newReq)
				} else {
					// GET/DELETE 等请求使用 Form 参数绑定
					err = ctx.ShouldBindWith(&newReq, binding.Form)
				}
			}
		} else {
			err = ctx.ShouldBind(&newReq)
		}
		if err != nil {
			zlog.Errorf(newCTL.GetCtx(), "Controller %T param bind error, err:%+v", newCTL, err)
			newCTL.RenderJsonFail(errors2.ErrorParamInvalid)
			return
		}
		// 实际业务逻辑执行
		data, err := newCTL.Action(&newReq)
		if err != nil {
			zlog.Errorf(newCTL.GetCtx(), "Controller %T call action logic error, err:%+v", newCTL, err)
			newCTL.RenderJsonFail(err)
			return
		}
		// 支持自定义渲染出参
		if newCTL.ShouldRender() {
			newCTL.RenderJsonSuccess(data)
		}
	}
}
