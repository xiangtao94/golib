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

	serviceerrors "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type Response struct {
	Code      string         `json:"code"`
	Reason    string         `json:"reason"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
	Data      any            `json:"data,omitempty"`
}

type Factory func(Response) any

// JSONRenderer owns its response factory. A service may adapt the standard
// contract at its outermost HTTP edge without changing business errors.
type JSONRenderer struct {
	factory Factory
}

func NewJSONRenderer(factory Factory) *JSONRenderer {
	if factory == nil {
		factory = func(response Response) any { return response }
	}
	return &JSONRenderer{factory: factory}
}

func ensureRequestID(ctx *gin.Context) string {
	requestContext, requestID := zlog.EnsureRequestID(
		ctx.Request.Context(),
		ctx.GetHeader(zlog.HeaderRequestID),
	)
	ctx.Request = ctx.Request.WithContext(requestContext)
	ctx.Header(zlog.HeaderRequestID, requestID)
	return requestID
}

func StackLogger(ctx *gin.Context, err error) {
	if !strings.Contains(fmt.Sprintf("%+v", err), "\n") {
		return
	}

	info := map[string]any{
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

func (renderer *JSONRenderer) Write(ctx *gin.Context, httpStatus int, response Response) {
	if httpStatus < 100 || httpStatus > 599 {
		httpStatus = http.StatusInternalServerError
	}
	response.RequestID = ensureRequestID(ctx)
	ctx.JSON(httpStatus, renderer.factory(response))
}

func (renderer *JSONRenderer) Success(ctx *gin.Context, data any) {
	renderer.Write(ctx, http.StatusOK, Response{
		Code:      "OK",
		Reason:    "OK",
		Message:   "success",
		Retryable: false,
		Data:      data,
	})
}

func (renderer *JSONRenderer) Failure(ctx *gin.Context, err error) {
	public := serviceerrors.From(err)
	renderer.Write(ctx, public.HTTPStatus(), Response{
		Code:      public.Code(),
		Reason:    public.Reason(),
		Message:   public.Message(),
		Retryable: public.Retryable(),
		Details:   public.Details(),
	})
	StackLogger(ctx, err)
}

func RenderStream(ctx *gin.Context, id, event string, data any) error {
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
	public := serviceerrors.From(err)
	response := Response{
		Code:      public.Code(),
		Reason:    public.Reason(),
		Message:   public.Message(),
		RequestID: ensureRequestID(ctx),
		Retryable: public.Retryable(),
		Details:   public.Details(),
	}
	encoded, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return marshalErr
	}
	return RenderStream(ctx, "", "error", string(encoded))
}
