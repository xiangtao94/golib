package middleware

import (
	"bytes"
	"fmt"
	"github.com/xiangtao94/golib/pkg/zlog"
	"io"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	_defaultPrintRequestLen  = 10240
	_defaultPrintResponseLen = 10240
)

type customRespWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w customRespWriter) WriteString(s string) (int, error) {
	if w.body != nil {
		w.body.WriteString(s)
	}
	return w.ResponseWriter.WriteString(s)
}

func (w customRespWriter) Write(b []byte) (int, error) {
	if w.body != nil {
		w.body.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// access日志打印
type AccessLoggerConfig struct {
	SkipPaths    []string `yaml:"skipPaths"`
	PrintHeaders []string `yaml:"printHeaders"`
	SkipCookie   bool     `yaml:"skipCookie"`
	// request body 最大长度展示，0表示采用默认的10240，-1表示不打印
	MaxReqBodyLen int `yaml:"maxReqBodyLen"`
	// response body 最大长度展示，0表示采用默认的10240，-1表示不打印。指定长度的时候需注意，返回的json可能被截断
	MaxRespBodyLen int `yaml:"maxRespBodyLen"`
	// 自定义Skip功能
	Skip func(ctx *gin.Context) bool
}

func AccessLog(conf AccessLoggerConfig) gin.HandlerFunc {
	notLogged := conf.SkipPaths
	var skip map[string]struct{}
	if length := len(notLogged); length > 0 {
		skip = make(map[string]struct{}, length)
		for _, path := range notLogged {
			skip[path] = struct{}{}
		}
	}

	maxReqBodyLen := conf.MaxReqBodyLen
	if maxReqBodyLen == 0 {
		maxReqBodyLen = _defaultPrintRequestLen
	}

	maxRespBodyLen := conf.MaxRespBodyLen
	if maxRespBodyLen == 0 {
		maxRespBodyLen = _defaultPrintResponseLen
	}

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path

		// body writer
		blw := &customRespWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		// 请求参数，涉及到回写，要在处理业务逻辑之前
		reqParam := getReqBody(c, maxReqBodyLen)

		c.Set(zlog.ContextKeyUri, path)
		_ = zlog.GetRequestID(c)

		// Process request
		c.Next()

		// Log only when path is not being skipped
		if _, ok := skip[path]; ok {
			return
		}

		if conf.Skip != nil && conf.Skip(c) {
			return
		}
		response := ""
		if blw.body != nil && maxRespBodyLen != -1 {
			response = blw.body.String()
			if len(response) > maxRespBodyLen {
				response = response[:maxRespBodyLen]
			}
		}
		// 固定notice
		commonFields := []zlog.Field{
			zlog.String("method", c.Request.Method),
			zlog.String("clientIp", c.ClientIP()),
			zlog.String("requestParam", reqParam),
			zlog.Int("responseStatus", c.Writer.Status()),
			zlog.String("response", response),
			zlog.Int("bodySize", c.Writer.Size()),
		}
		if len(conf.PrintHeaders) > 0 {
			commonFields = append(commonFields, zlog.String("requestHeader", getHeader(c, conf.PrintHeaders)))
		}
		if !conf.SkipCookie {
			commonFields = append(commonFields, zlog.String("cookie", getCookie(c)))
		}
		commonFields = append(commonFields, zlog.AppendCostTime(start, time.Now())...)
		// 新的notice添加方式
		customerFields := zlog.GetCustomerFields(c)
		commonFields = append(commonFields, customerFields...)
		zlog.InfoLogger(c, "", commonFields...)
	}
}

// 请求参数
func getReqBody(c *gin.Context, maxReqBodyLen int) (reqBody string) {
	// 不打印参数
	if maxReqBodyLen == -1 {
		return reqBody
	}

	// body中的参数
	if c.Request.Body != nil && c.ContentType() == binding.MIMEMultipartPOSTForm {
		requestBody, err := c.GetRawData()
		if err != nil {
			zlog.WarnLogger(c, "get http request body error: "+err.Error())
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))

		if _, err := c.MultipartForm(); err != nil {
			zlog.WarnLogger(c, "parse http request form body error: "+err.Error())
		}
		reqBody = c.Request.PostForm.Encode()
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	} else if c.Request.Body != nil {
		requestBody, err := c.GetRawData()
		if err != nil {
			zlog.WarnLogger(c, "get http request body error: "+err.Error())
		}
		reqBody = *(*string)(unsafe.Pointer(&requestBody))
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	} else if len(c.Request.URL.Query()) > 0 {
		allParams := c.Request.URL.Query()
		reqBody = allParams.Encode()
	}
	// 截断参数
	if len(reqBody) > maxReqBodyLen {
		reqBody = reqBody[:maxReqBodyLen]
	}
	return reqBody
}

func getCookie(ctx *gin.Context) string {
	cStr := ""
	for _, c := range ctx.Request.Cookies() {
		cStr += fmt.Sprintf("%s=%s&", c.Name, c.Value)
	}
	return strings.TrimRight(cStr, "&")
}

func getHeader(ctx *gin.Context, headers []string) string {
	cStr := ""
	for k, v := range ctx.Request.Header {
		if slices.Contains(headers, k) {
			cStr += fmt.Sprintf("%s=%s&", k, v)
		}
	}
	return strings.TrimRight(cStr, "&")
}

func RegistryAccessLog(engine *gin.Engine, conf AccessLoggerConfig) {
	engine.Use(AccessLog(conf))
}
