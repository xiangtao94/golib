package components

import (
	"github.com/tiant-go/golib/pkg/errors"
)

var OutErrMsg = map[int]string{}

var OutErrMap = map[int]int{}

// 用户未登录
var ErrorTokenExpire = errors.Error{
	Code:    401,
	Message: "用户session失效，请重新登录",
}
