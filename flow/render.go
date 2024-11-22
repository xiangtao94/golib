package flow

import (
	"encoding/json"
	"fmt"
	errors2 "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// 定义render通用类型
type Render interface {
	SetReturnCode(int)
	SetReturnMsg(string)
	SetReturnData(interface{})
	GetReturnCode() int
	GetReturnMsg() string
}

var newRender func() Render

func RegisterRender(s func() Render) {
	newRender = s
}

func newJsonRender() Render {
	if newRender == nil {
		newRender = defaultNew
	}
	return newRender()
}

func RenderJson(ctx *gin.Context, code int, msg string, data interface{}) {
	r := newJsonRender()
	r.SetReturnCode(code)
	r.SetReturnMsg(msg)
	r.SetReturnData(data)

	setCommonHeader(ctx, code, msg)
	ctx.JSON(http.StatusOK, r)
	return
}

func RenderJsonSucc(ctx *gin.Context, data interface{}) {
	r := newJsonRender()
	r.SetReturnCode(200)
	r.SetReturnMsg("success")
	r.SetReturnData(data)

	setCommonHeader(ctx, 200, "success")
	ctx.JSON(http.StatusOK, r)
	return
}

func RenderJsonFail(ctx *gin.Context, err error) {
	r := newJsonRender()

	code, msg := -1, errors.Cause(err).Error()
	switch errors.Cause(err).(type) {
	case errors2.Error:
		code = errors.Cause(err).(errors2.Error).Code
		msg = errors.Cause(err).(errors2.Error).Message
	default:
	}

	r.SetReturnCode(code)
	r.SetReturnMsg(msg)
	r.SetReturnData(gin.H{})

	setCommonHeader(ctx, code, msg)
	ctx.JSON(http.StatusOK, r)

	// 打印错误栈
	StackLogger(ctx, err)
	return
}

// default render

var defaultNew = func() Render {
	return &DefaultRender{}
}

type DefaultRender struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (r *DefaultRender) GetReturnCode() int {
	return r.Code
}
func (r *DefaultRender) SetReturnCode(code int) {
	r.Code = code
}
func (r *DefaultRender) GetReturnMsg() string {
	return r.Message
}
func (r *DefaultRender) SetReturnMsg(msg string) {
	r.Message = msg
}
func (r *DefaultRender) GetReturnData() interface{} {
	return r.Data
}
func (r *DefaultRender) SetReturnData(data interface{}) {
	r.Data = data
}

// 设置通用header头
func setCommonHeader(ctx *gin.Context, code int, msg string) {
	ctx.Header("code", strconv.Itoa(code))
	ctx.Header("message", msg)
	ctx.Header(zlog.ContextKeyRequestID, zlog.GetRequestID(ctx))
}

// 打印错误栈
func StackLogger(ctx *gin.Context, err error) {
	if !strings.Contains(fmt.Sprintf("%+v", err), "\n") {
		return
	}

	var info []byte
	if ctx != nil {
		info, _ = json.Marshal(map[string]interface{}{"time": time.Now().Format("2006-01-02 15:04:05"), "level": "error", "module": "errorstack", "requestId": zlog.GetRequestID(ctx)})
	} else {
		info, _ = json.Marshal(map[string]interface{}{"time": time.Now().Format("2006-01-02 15:04:05"), "level": "error", "module": "errorstack"})
	}

	fmt.Printf("%s\n-------------------stack-start-------------------\n%+v\n-------------------stack-end-------------------\n", string(info), err)
}
