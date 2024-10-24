package errors

import (
	"fmt"

	"github.com/pkg/errors"
)

type Error struct {
	Code    int
	Message string
}

func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

func (err Error) Error() string {
	return err.Message
}

func (err Error) Sprintf(v ...interface{}) Error {
	err.Message = fmt.Sprintf(err.Message, v...)
	return err
}

func (err Error) Equal(e error) bool {
	switch errors.Cause(e).(type) {
	case Error:
		return err.Code == errors.Cause(e).(Error).Code
	default:
		return false
	}
}

func (err Error) WrapPrint(core error, message string) error {
	if core == nil {
		return nil
	}
	err.Message = fmt.Sprint(err.Message, core)
	return errors.Wrap(err, message)
}

func (err Error) WrapPrintf(core error, format string, message ...interface{}) error {
	if core == nil {
		return nil
	}
	err.Message = fmt.Sprintf(err.Message, core)
	return errors.Wrap(err, fmt.Sprintf(format, message...))
}

func (err Error) Wrap(core error) error {
	if core == nil {
		return nil
	}
	msg := err.Message
	err.Message = core.Error()
	return errors.Wrap(err, msg)
}

// 标准准出错误码定义
const (
	// 通用错误码
	PARAM_ERROR     = 1   //参数错误
	SYSTEM_ERROR    = 2   //服务内部错误
	USER_NOT_LOGIN  = 3   //用户未登录
	INVALID_REQUEST = 6   //无效请求
	DEFAULT_ERROR   = 100 //默认错误，未准出的错误码，会修改为此错误码
	CUSTOM_ERROR    = 101 //自定义错误，无固定错误文案
)

// 标准准出错误码文案定义
var ErrMsg = map[int]string{
	// 通用错误文案
	PARAM_ERROR:     "请求参数错误",
	SYSTEM_ERROR:    "服务异常，请稍后重试",
	USER_NOT_LOGIN:  "用户Session已失效，请重新登录",
	INVALID_REQUEST: "请求无效，请稍后再试",
	DEFAULT_ERROR:   "服务开小差了，请稍后再试",
}

// *****以下是通用准出错误码的简便定义***********
// 正常
var ErrorSuccess = Error{
	Code:    0,
	Message: "success",
}

// 参数错误
var ErrorParamInvalid = Error{
	Code:    PARAM_ERROR,
	Message: ErrMsg[PARAM_ERROR],
}

// 系统异常
var ErrorSystemError = Error{
	Code:    SYSTEM_ERROR,
	Message: ErrMsg[SYSTEM_ERROR],
}

// 用户未登录
var ErrorUserNotLogin = Error{
	Code:    USER_NOT_LOGIN,
	Message: ErrMsg[USER_NOT_LOGIN],
}

// 无效请求
var ErrorInvalidRequest = Error{
	Code:    INVALID_REQUEST,
	Message: ErrMsg[INVALID_REQUEST],
}

// 默认错误
var ErrorDefault = Error{
	Code:    DEFAULT_ERROR,
	Message: ErrMsg[DEFAULT_ERROR],
}

// 自定义错误
// 使用方式：ErrorCustomError.Sprintf(v)
var ErrorCustomError = Error{
	Code:    CUSTOM_ERROR,
	Message: "%s",
}
