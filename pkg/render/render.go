package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"

	errors2 "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type Render interface {
	SetReturnCode(int)
	SetReturnMsg(string)
	SetReturnData(interface{})
	SetReturnRequestId(string)
	GetReturnCode() int
	GetReturnMsg() string
}

type Factory func() Render

// JSONRenderer owns its factory. Custom response shapes are injected per
// handler/application instead of mutating package-global state.
type JSONRenderer struct {
	factory Factory
}

func NewJSONRenderer(factory Factory) *JSONRenderer {
	if factory == nil {
		factory = func() Render { return &DefaultRender{} }
	}
	return &JSONRenderer{factory: factory}
}

type DefaultRender struct {
	Code      int         `json:"code" example:"200"`
	Message   string      `json:"message" example:"Success"`
	RequestId string      `json:"request_id,omitempty"`
	Data      interface{} `json:"data"`
}

func (r *DefaultRender) SetReturnRequestId(requestID string) { r.RequestId = requestID }
func (r *DefaultRender) GetReturnCode() int                  { return r.Code }
func (r *DefaultRender) SetReturnCode(code int)              { r.Code = code }
func (r *DefaultRender) GetReturnMsg() string                { return r.Message }
func (r *DefaultRender) SetReturnMsg(message string)         { r.Message = message }
func (r *DefaultRender) GetReturnData() interface{}          { return r.Data }
func (r *DefaultRender) SetReturnData(data interface{})      { r.Data = data }

func setCommonHeader(ctx *gin.Context, requestID string) {
	ctx.Header(zlog.HeaderRequestID, requestID)
}

func StackLogger(ctx *gin.Context, err error) {
	if !strings.Contains(fmt.Sprintf("%+v", err), "\n") {
		return
	}

	info := map[string]interface{}{
		"time":   time.Now().Format("2006-01-02 15:04:05"),
		"level":  "error",
		"module": "errorstack",
	}
	if ctx != nil {
		info["requestId"] = zlog.GetRequestID(ctx)
	}
	encoded, _ := json.Marshal(info)
	fmt.Printf("%s\n-------------------stack-start-------------------\n%+v\n-------------------stack-end-------------------\n", encoded, err)
}

func (renderer *JSONRenderer) JSON(ctx *gin.Context, httpStatus, code int, message string, data interface{}) {
	if httpStatus < 100 || httpStatus > 599 {
		httpStatus = http.StatusInternalServerError
	}
	requestID := zlog.GetRequestID(ctx)
	response := renderer.factory()
	response.SetReturnCode(code)
	response.SetReturnMsg(message)
	response.SetReturnData(data)
	response.SetReturnRequestId(requestID)
	setCommonHeader(ctx, requestID)
	ctx.JSON(httpStatus, response)
}

func (renderer *JSONRenderer) Success(ctx *gin.Context, data interface{}) {
	renderer.JSON(ctx, http.StatusOK, http.StatusOK, "success", data)
}

func (renderer *JSONRenderer) Failure(ctx *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := errors2.ErrorSystemError.Code
	message := errors2.ErrorSystemError.GetMessage(ctx)

	var typedError errors2.Error
	if errors.As(err, &typedError) {
		status = typedError.HTTPStatus
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		code = typedError.Code
		message = typedError.GetMessage(ctx)
	}
	renderer.JSON(ctx, status, code, message, gin.H{})
	StackLogger(ctx, err)
}

func RenderJson(ctx *gin.Context, httpStatus, code int, message string, data interface{}) {
	NewJSONRenderer(nil).JSON(ctx, httpStatus, code, message, data)
}

func RenderJsonSucc(ctx *gin.Context, data interface{}) {
	NewJSONRenderer(nil).Success(ctx, data)
}

func RenderJsonFail(ctx *gin.Context, err error) {
	NewJSONRenderer(nil).Failure(ctx, err)
}

func RenderStream(ctx *gin.Context, id, event string, data interface{}) error {
	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		return errors.New("response writer does not support streaming")
	}
	if err := sse.Encode(ctx.Writer, sse.Event{Id: id, Event: event, Data: data}); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func RenderStreamFail(ctx *gin.Context, err error) error {
	response := DefaultRender{
		Code:      errors2.ErrorSystemError.Code,
		Message:   errors2.ErrorSystemError.GetMessage(ctx),
		RequestId: zlog.GetRequestID(ctx),
	}
	var typedError errors2.Error
	if errors.As(err, &typedError) {
		response.Code = typedError.Code
		response.Message = typedError.GetMessage(ctx)
	}
	encoded, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return marshalErr
	}
	return RenderStream(ctx, "", "error", string(encoded))
}
