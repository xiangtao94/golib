package sse

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	http2 "github.com/tiant-go/golib/flow"
	"github.com/tiant-go/golib/pkg/errors"
	"net/http"
)

// 定义 SSE 事件
type MessageEvent struct {
	Id    string
	Event string
	Data  string
}

// 实现 SSE 事件的 String() 方法
func (e MessageEvent) String() string {
	return fmt.Sprintf("id:%s\n"+
		"event:%s\n"+
		"data:%s\n\n", e.Id, e.Event, e.Data)
}

// 流式输出报错
func RenderStreamError(ctx *gin.Context, err error) {
	rander := http2.DefaultRender{}
	if e, ok := err.(errors.Error); ok {
		rander.Code = e.Code
		rander.Message = e.Message
	} else {
		rander.Code = errors.ErrorSystemError.Code
		rander.Message = errors.ErrorSystemError.Message
	}
	flusher, _ := ctx.Writer.(http.Flusher)
	str, _ := json.Marshal(rander)
	msg := MessageEvent{
		Id:    "",
		Event: "error",
		Data:  string(str),
	}
	fmt.Fprintf(ctx.Writer, "%s", msg.String())
	flusher.Flush()
}

func RenderStream(ctx *gin.Context, id, event, str string) {
	flusher, _ := ctx.Writer.(http.Flusher)
	msg := MessageEvent{
		Id:    id,
		Event: event,
		Data:  str,
	}
	fmt.Fprintf(ctx.Writer, "%s", msg.String())
	flusher.Flush()
}
