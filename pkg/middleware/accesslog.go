package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/xiangtao94/golib/pkg/zlog"
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
	PrintCookie  bool     `yaml:"printCookie"`
	// request body 最大长度展示，0表示采用默认的10240，-1表示不打印
	MaxReqBodyLen int `yaml:"maxReqBodyLen"`
	// response body 最大长度展示，0表示采用默认的10240，-1表示不打印。指定长度的时候需注意，返回的json可能被截断
	MaxRespBodyLen int `yaml:"maxRespBodyLen"`
	// 自定义Skip功能
	Skip func(ctx *gin.Context) bool
}

// DefaultAccessLoggerConfig 返回默认的Access日志配置
func DefaultAccessLoggerConfig() AccessLoggerConfig {
	return AccessLoggerConfig{
		SkipPaths:      []string{},
		PrintHeaders:   []string{},
		PrintCookie:    false,
		MaxReqBodyLen:  _defaultPrintRequestLen,
		MaxRespBodyLen: _defaultPrintResponseLen,
		Skip:           nil,
	}
}

// mergeWithDefaultAccessLog 将用户配置与默认配置合并
func mergeWithDefaultAccessLog(userConf AccessLoggerConfig) AccessLoggerConfig {
	defaultConf := DefaultAccessLoggerConfig()

	// 如果用户没有设置，使用默认值
	if userConf.SkipPaths == nil {
		userConf.SkipPaths = defaultConf.SkipPaths
	}
	if userConf.PrintHeaders == nil {
		userConf.PrintHeaders = defaultConf.PrintHeaders
	}
	if userConf.MaxReqBodyLen == 0 {
		userConf.MaxReqBodyLen = defaultConf.MaxReqBodyLen
	}
	if userConf.MaxRespBodyLen == 0 {
		userConf.MaxRespBodyLen = defaultConf.MaxRespBodyLen
	}
	if userConf.Skip == nil {
		userConf.Skip = defaultConf.Skip
	}

	return userConf
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

		// 固定notice
		commonFields := []zlog.Field{
			zlog.String("method", c.Request.Method),
			zlog.String("uri", path),
			zlog.Int("status", c.Writer.Status()),
			zlog.String("clientIp", c.ClientIP()),
			zlog.String("requestParam", reqParam),
		}
		if len(conf.PrintHeaders) > 0 {
			commonFields = append(commonFields, zlog.String("requestHeader", getHeader(c, conf.PrintHeaders)))
		}
		if conf.PrintCookie {
			commonFields = append(commonFields, zlog.String("cookie", getCookie(c)))
		}
		contentType := c.Writer.Header().Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			mediaType = ""
		}
		var response any
		if blw.body != nil && maxRespBodyLen != -1 {
			if strings.Contains(mediaType, "application/json") {
				response = json.RawMessage{}
				_ = json.Unmarshal(blw.body.Bytes(), &response)
			} else if strings.Contains(mediaType, "text/event-stream") {
				response = blw.body.String()
			}
		}
		commonFields = append(commonFields, zlog.Any("responseBody", response), zlog.Int("bodySize", c.Writer.Size()))
		commonFields = append(commonFields, AppendCostTime(start, time.Now())...)
		// 新的notice添加方式
		customerFields := zlog.GetCustomerFields(c)
		commonFields = append(commonFields, customerFields...)
		zlog.AccessInfo(c, commonFields...)
	}
}

// 请求参数
func getReqBody(c *gin.Context, maxReqBodyLen int) (reqBody string) {
	// 不打印参数
	if maxReqBodyLen == -1 {
		return reqBody
	}
	if c.Request.Method == "GET" || c.Request.Method == "DELETE" {
		allParams := c.Request.URL.Query()
		reqBody = allParams.Encode()
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

func RegistryAccessLog(engine *gin.Engine, conf ...AccessLoggerConfig) {
	var logConf AccessLoggerConfig
	if len(conf) > 0 {
		// 使用传入的配置，并与默认配置合并
		logConf = mergeWithDefaultAccessLog(conf[0])
	} else {
		// 使用默认配置
		logConf = DefaultAccessLoggerConfig()
	}
	engine.Use(AccessLog(logConf))
}

func AppendCostTime(begin, end time.Time) []zlog.Field {
	return []zlog.Field{
		zlog.String("startTime", zlog.GetFormatRequestTime(begin)),
		zlog.String("endTime", zlog.GetFormatRequestTime(end)),
		zlog.String("cost", fmt.Sprintf("%v%s", zlog.GetRequestCost(begin, end), "ms")),
	}
}
