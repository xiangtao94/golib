package http

import (
	"encoding/json"
	"github.com/pkg/errors"
	"net/url"
	"time"
	"unsafe"
)

const (
	EncodeJson    = "_json"
	EncodeForm    = "_form"
	EncodeRaw     = "_raw"
	EncodeRawByte = "_raw_byte"
)

type HttpRequestOptions struct {
	// 通用请求体，可通过Encode来对body做编码
	RequestBody interface{}
	// 请求头指定
	Headers map[string]string
	// cookie 设定
	Cookies map[string]string
	/*
		httpGet / httPost 默认 application/x-www-form-urlencoded
		httpPostJson 默认 application/json
	*/
	ContentType string
	// 针对 RequestBody 的编码
	Encode string
	// 接口请求级timeout。不管retry是多少，那么每次执行的总时间都是timeout。
	// 这个timeout与client.Timeout 没有直接关系，总执行超时时间取二者最小值。
	Timeout time.Duration

	// 重试策略，可不指定，默认使用`defaultRetryPolicy`(只有在`api.yaml`中指定retry>0 时生效)
	RetryPolicy RetryPolicy `json:"-"`
}

func (o *HttpRequestOptions) getData() ([]byte, error) {

	if o.RequestBody == nil {
		return nil, nil
	}

	switch o.Encode {
	case EncodeJson:
		reqBody, err := json.Marshal(o.RequestBody)
		return reqBody, err
	case EncodeRaw:
		var err error
		encodeData, ok := o.RequestBody.(string)
		if !ok {
			err = errors.New("EncodeRaw need string type")
		}
		return *(*[]byte)(unsafe.Pointer(&encodeData)), err
	case EncodeRawByte:
		var err error
		encodeData, ok := o.RequestBody.([]byte)
		if !ok {
			err = errors.New("EncodeRawByte need []byte type")
		}
		return encodeData, err
	case EncodeForm:
		fallthrough
	default:
		encodeData, err := o.getFormRequestData()
		return *(*[]byte)(unsafe.Pointer(&encodeData)), err
	}
}
func (o *HttpRequestOptions) getFormRequestData() (string, error) {
	v := url.Values{}

	if data, ok := o.RequestBody.(map[string]string); ok {
		for key, value := range data {
			v.Add(key, value)
		}
		return v.Encode(), nil
	}

	if data, ok := o.RequestBody.(map[string]interface{}); ok {
		for key, value := range data {
			var vStr string
			switch value.(type) {
			case string:
				vStr = value.(string)
			default:
				if tmp, err := json.Marshal(value); err != nil {
					return "", err
				} else {
					vStr = string(tmp)
				}
			}

			v.Add(key, vStr)
		}
		return v.Encode(), nil
	}

	return "", errors.New("unSupport RequestBody type")
}
func (o *HttpRequestOptions) GetContentType() (cType string) {
	if cType = o.ContentType; cType != "" {
		return cType
	}
	// 根据 encode 获得一个默认的类型
	switch o.Encode {
	case EncodeJson:
		cType = "application/json"

	case EncodeForm:
		fallthrough
	default:
		cType = "application/x-www-form-urlencoded"
	}
	return cType
}
