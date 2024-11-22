package flow

import (
	"github.com/xiangtao94/golib/pkg/zlog"

	"github.com/gin-gonic/gin"
	"reflect"
	"sync"
	"time"
)

type ILayer interface {
	GetCtx() *gin.Context
	SetCtx(*gin.Context)
	Create(newFlow ILayer) interface{}
	OnCreate()
	Assign(targets ...any)
	CopyWithCtx(ctx *gin.Context) interface{}
	SetEntity(entity ILayer)
	StartTimer(timerKey string)
	StopTimer(timerKey string) int

	LogDebugf(format string, args ...interface{})
	LogInfof(format string, args ...interface{})
	LogWarnf(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
}

type Layer struct {
	ctx    *gin.Context
	entity ILayer
	m      sync.Mutex
	timer  map[string]int64
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

func (entity *Layer) Create(newLayer ILayer) interface{} {
	newLayer.SetCtx(entity.ctx)
	newLayer.SetEntity(newLayer)
	newLayer.OnCreate()
	return newLayer
}

func (entity *Layer) Assign(targets ...any) {
	// 遍历，根据target的类型new出对象，并赋值到target指针
	for _, dst := range targets {
		pDst := reflect.ValueOf(dst)
		if pDst.Kind() == reflect.Ptr {
			pDst = pDst.Elem()
		}
		if pDst.Kind() == reflect.Ptr {
			t := pDst.Type().Elem()
			v := reflect.New(t).Elem().Addr().Interface().(ILayer)
			flow := entity.Create(v)
			pDst.Set(reflect.ValueOf(flow))
		}
	}
}

func (entity *Layer) CopyWithCtx(ctx *gin.Context) interface{} {
	v := NewObject(entity.entity).(ILayer)
	e := entity.Create(v).(ILayer)
	if ctx != nil {
		e.SetCtx(ctx)
	}
	return e
}

func (entity *Layer) StartTimer(timerKey string) {
	entity.m.Lock()
	defer entity.m.Unlock()
	if entity.timer == nil {
		entity.timer = make(map[string]int64)
	}
	entity.timer[timerKey] = time.Now().UnixNano()
}

func (entity *Layer) StopTimer(timerKey string) int {
	entity.m.Lock()
	defer entity.m.Unlock()
	if entity.timer == nil {
		return 0
	}
	if v, ok := entity.timer[timerKey]; ok {
		now := time.Now().UnixNano()
		pass := int((now - v) / int64(time.Millisecond)) //ms
		zlog.AddField(entity.GetCtx(), zlog.Int(timerKey, pass))
		delete(entity.timer, timerKey)
		return pass
	}
	return 0
}

func (entity *Layer) LogDebugf(format string, args ...interface{}) {
	zlog.Debugf(entity.ctx, format, args...)
}

func (entity *Layer) LogInfof(format string, args ...interface{}) {
	zlog.Infof(entity.ctx, format, args...)
}

func (entity *Layer) LogWarnf(format string, args ...interface{}) {
	zlog.Warnf(entity.ctx, format, args...)
}

func (entity *Layer) LogErrorf(format string, args ...interface{}) {
	zlog.Errorf(entity.ctx, format, args...)
}

func NewObject(src interface{}) interface{} {
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

func Create[T ILayer](ctx *gin.Context, newLayer T) T {
	newLayer.SetCtx(ctx)
	newLayer.SetEntity(newLayer)
	newLayer.OnCreate()
	return newLayer
}
