package flow

import (
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net/http"
	"reflect"
)

type IController[T any] interface {
	ILayer
	Action(req T) (any, error)
	ShouldRender() bool
	RenderJsonFail(err error)
	RenderJsonSuccess(data any)
}

type Controller struct {
	Layer
}

func (entity *Controller) Action(any) (any, error) {
	return nil, nil
}

func (entity *Controller) ShouldRender() bool {
	return true
}

func (entity *Controller) RenderJsonFail(err error) {
	RenderJsonFail(entity.GetCtx(), err)
}

func (entity *Controller) RenderJsonSuccess(data any) {
	RenderJsonSucc(entity.GetCtx(), data)
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

func Use[T any](controller IController[*T]) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		newCTL := slave(controller).(IController[*T])
		var newReq T
		newCTL.SetCtx(ctx)
		newCTL.SetEntity(controller)
		if len(ctx.ContentType()) == 0 && ctx.Request.Method == http.MethodPost { // post默认application/json
			err := ctx.BindJSON(&newReq)
			if err != nil {
				zlog.Errorf(newCTL.GetCtx(), "Controller %T param bind error, err:%+v", newCTL, err)
				newCTL.RenderJsonFail(errors.ErrorParamInvalid)
				return
			}
		} else {
			err := ctx.ShouldBind(&newReq)
			if err != nil {
				zlog.Errorf(newCTL.GetCtx(), "Controller %T param bind error, err:%+v", newCTL, err)
				newCTL.RenderJsonFail(errors.ErrorParamInvalid)
				return
			}
		}
		// action execute
		data, err := newCTL.Action(&newReq)
		if err != nil {
			zlog.Errorf(newCTL.GetCtx(), "Controller %T call action logic error, err:%+v", newCTL, err)
			newCTL.RenderJsonFail(err)
			return
		}
		// 支持自定义渲染
		if newCTL.ShouldRender() {
			newCTL.RenderJsonSuccess(data)
		}
	}
}
