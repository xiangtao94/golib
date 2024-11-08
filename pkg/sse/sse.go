package sse

import (
	"encoding/json"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	http2 "github.com/tiant-go/golib/flow"
	"github.com/tiant-go/golib/pkg/errors"
	"net/http"
)

func RenderStreamData(ctx *gin.Context, data interface{}) {
	flusher, _ := ctx.Writer.(http.Flusher)
	sse.Encode(ctx.Writer, sse.Event{
		Data: data,
	})
	flusher.Flush()
}

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
	sse.Encode(ctx.Writer, sse.Event{
		Event: "error",
		Data:  string(str),
	})
	flusher.Flush()
}
