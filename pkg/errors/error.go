package errors

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/env"
)

// Error 结构体支持多语言
type Error struct {
	Code    int
	Message map[string]string // 存储不同语言的消息
}

// NewError 创建新的错误对象，并支持双语
func NewError(code int, messages map[string]string) Error {
	// 如果messages为空，则自动从ErrMsg获取
	if messages == nil {
		messages = make(map[string]string)
	}

	// 获取zh和en默认消息
	if msg, ok := ErrMsg["zh"][code]; ok {
		messages["zh"] = msg
	}
	if msg, ok := ErrMsg["en"][code]; ok {
		messages["en"] = msg
	}

	return Error{
		Code:    code,
		Message: messages,
	}
}

func (err Error) Sprintf(v ...interface{}) Error {
	for s, s2 := range err.Message {
		err.Message[s] = fmt.Sprintf(s2, v...)
	}
	return err
}

// GetMessage 获取指定语言的错误信息
func (err Error) GetMessage(ctx *gin.Context) string {
	lang := ctx.GetString(env.I18N_CONTEXT)
	// 如果语言不存在，返回默认语言信息
	if msg, exists := err.Message[lang]; exists {
		return msg
	} else {
		return err.Message[env.GetLanguage()]
	}
}

// Error 方法默认返回当前设定语言的信息
func (err Error) Error() string {
	if msg, ok := err.Message[env.GetLanguage()]; ok {
		return msg
	}
	return "Unknown error"
}

// 定义错误码
const (
	SYSTEM_ERROR    = 1
	PARAM_ERROR     = 2
	USER_NOT_LOGIN  = 3
	INVALID_REQUEST = 4
	DEFAULT_ERROR   = 100
	CUSTOM_ERROR    = 101
)

// 多语言错误消息
var ErrMsg = map[string]map[int]string{
	"zh": {
		PARAM_ERROR:     "请求参数错误",
		SYSTEM_ERROR:    "服务异常，请稍后重试",
		USER_NOT_LOGIN:  "用户Session已失效，请重新登录",
		INVALID_REQUEST: "请求无效，请稍后再试",
		DEFAULT_ERROR:   "服务开小差了，请稍后再试",
	},
	"en": {
		PARAM_ERROR:     "Request parameter error",
		SYSTEM_ERROR:    "Service exception, please try again later",
		USER_NOT_LOGIN:  "User session expired, please log in again",
		INVALID_REQUEST: "Invalid request, please try again later",
		DEFAULT_ERROR:   "The service is down, please try again later",
	},
}

// 定义标准错误
var (
	ErrorParamInvalid   = NewError(PARAM_ERROR, nil)
	ErrorSystemError    = NewError(SYSTEM_ERROR, nil)
	ErrorUserNotLogin   = NewError(USER_NOT_LOGIN, nil)
	ErrorInvalidRequest = NewError(INVALID_REQUEST, nil)
	ErrorDefault        = NewError(DEFAULT_ERROR, nil)
	ErrorCustomError    = NewError(CUSTOM_ERROR, map[string]string{"zh": "%s", "en": "%s"})
)
