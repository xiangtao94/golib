package flow

import (
	"github.com/gin-gonic/gin"
	"reflect"
)

type ILayer interface {
	SetCtx(*gin.Context)
	GetCtx() *gin.Context
	OnCreate()
	SetEntity(entity ILayer)
}

type Layer struct {
	ctx    *gin.Context
	entity ILayer
}

func (entity *Layer) SetCtx(ctx *gin.Context) {
	entity.ctx = ctx
}

func (entity *Layer) GetCtx() *gin.Context {
	return entity.ctx
}

func (entity *Layer) SetEntity(flow ILayer) {
	entity.entity = flow
}

func (entity *Layer) GetEntity() ILayer {
	return entity.entity
}

func (entity *Layer) OnCreate() {

}

// 复制对象并带上新的上下文
func CopyWithCtx[T ILayer](src T) T {
	var v T
	v = NewObject(src)     // 深拷贝 src
	v.SetCtx(src.GetCtx()) // 设置新的上下文
	v.SetEntity(v)         // 确保实体指向自己
	v.OnCreate()           // 调用创建方法
	return v
}

// 泛型版本的 NewObject，创建 src 的深拷贝
func NewObject[T ILayer](src T) T {
	typ := reflect.TypeOf(src)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem() // 获取指针指向的实际类型
	}
	newInstance := reflect.New(typ).Interface().(T) // 创建新实例
	return newInstance
}

func Create[T ILayer](ctx *gin.Context, newLayer T) T {
	newLayer.SetCtx(ctx)
	newLayer.SetEntity(newLayer)
	newLayer.OnCreate()
	return newLayer
}
