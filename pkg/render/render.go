// Package render -----------------------------
// @file      : render.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/4/2 11:10
// -------------------------------------------
package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	errors2 "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
)

// 定义render通用类型
type Render interface {
	SetReturnCode(int)
	SetReturnMsg(string)
	SetReturnData(interface{})
	SetReturnRequestId(string)
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

// default render

var defaultNew = func() Render {
	return &DefaultRender{}
}

type DefaultRender struct {
	Code      int         `json:"code" example:"200"`
	Message   string      `json:"message" example:"Success"`
	RequestId string      `json:"request_id,omitempty"`
	Data      interface{} `json:"data"`
}

func (r *DefaultRender) SetReturnRequestId(requestId string) {
	r.RequestId = requestId
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

func RenderJson(ctx *gin.Context, code int, msg string, data interface{}) {
	r := newJsonRender()
	r.SetReturnCode(code)
	r.SetReturnMsg(msg)
	r.SetReturnData(data)
	r.SetReturnRequestId(zlog.GetRequestID(ctx))
	setCommonHeader(ctx, code, msg)
	ctx.JSON(http.StatusOK, r)
	return
}

func RenderJsonSucc(ctx *gin.Context, data interface{}) {
	r := newJsonRender()
	r.SetReturnCode(200)
	r.SetReturnMsg("success")
	r.SetReturnData(data)
	r.SetReturnRequestId(zlog.GetRequestID(ctx))
	setCommonHeader(ctx, 200, "success")
	ctx.JSON(http.StatusOK, r)
	return
}

func RenderJsonFail(ctx *gin.Context, err error) {
	r := newJsonRender()

	code := 500
	msg := err.Error()

	var e2 errors2.Error
	if errors.As(err, &e2) {
		code = e2.Code
		msg = e2.GetMessage(ctx)
	} else {
		code = errors2.ErrorSystemError.Code
		msg = errors2.ErrorSystemError.GetMessage(ctx)
	}

	r.SetReturnCode(code)
	r.SetReturnMsg(msg)
	r.SetReturnData(gin.H{})

	setCommonHeader(ctx, code, msg)
	ctx.JSON(http.StatusOK, r)

	// 打印错误栈（标准库没有自动栈，需要你在生成错误时自己加）
	StackLogger(ctx, err)
	return
}
func RenderStream(ctx *gin.Context, id, event string, data interface{}) {
	flusher, _ := ctx.Writer.(http.Flusher)
	sse.Encode(ctx.Writer, sse.Event{
		Id:    id,
		Event: event,
		Data:  data,
	})
	flusher.Flush()
}

func RenderStreamFail(ctx *gin.Context, err error) {
	rander := DefaultRender{}
	if e, ok := err.(errors2.Error); ok {
		rander.Code = e.Code
		rander.Message = e.GetMessage(ctx)
	} else {
		rander.Code = errors2.ErrorSystemError.Code
		rander.Message = errors2.ErrorSystemError.GetMessage(ctx)
	}
	rander.RequestId = zlog.GetRequestID(ctx)
	flusher, _ := ctx.Writer.(http.Flusher)
	str, _ := json.Marshal(rander)
	sse.Encode(ctx.Writer, sse.Event{
		Event: "error",
		Data:  string(str),
	})
	flusher.Flush()
}
