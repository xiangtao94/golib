package flow

import (
	"encoding/json"
	"fmt"
	"github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/http"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type ApiRes struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	RequestId string          `json:"request_id"`
	Data      json.RawMessage `json:"data,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type IApi interface {
	ILayer
	GetEncodeType() string
	ApiGet(path string, requestParam interface{}) (*ApiRes, error)
	ApiPost(path string, requestBody interface{}) (*ApiRes, error)
	ApiGetWithOpts(path string, reqOpts http.HttpRequestOptions) (*ApiRes, error)
	ApiPostWithOpts(path string, reqOpts http.HttpRequestOptions) (*ApiRes, error)
}

type Api struct {
	Layer
	EncodeType string
	Client     *http.HttpClientConf
}

// api请求数据格式，默认json
func (entity *Api) GetEncodeType() string {
	if entity.EncodeType != "" {
		return entity.EncodeType
	}
	return http.EncodeJson
}

func (entity *Api) ApiGet(path string, requestParam interface{}) (*ApiRes, error) {
	reqOpts := http.HttpRequestOptions{
		RequestBody: requestParam,
		Encode:      http.EncodeForm,
	}
	return entity.ApiGetWithOpts(path, reqOpts)
}

func (entity *Api) ApiDelete(path string, requestParam interface{}) (*ApiRes, error) {
	reqOpts := http.HttpRequestOptions{
		RequestBody: requestParam,
		Encode:      http.EncodeForm,
	}
	return entity.ApiDeleteWithOpts(path, reqOpts)
}

func (entity *Api) ApiPut(path string, requestBody interface{}) (*ApiRes, error) {
	api2 := entity.GetEntity().(IApi)
	reqOpts := http.HttpRequestOptions{
		RequestBody: requestBody,
		Encode:      api2.GetEncodeType(),
	}
	return entity.ApiPutWithOpts(path, reqOpts)
}

func (entity *Api) ApiPost(path string, requestBody interface{}) (*ApiRes, error) {
	api2 := entity.GetEntity().(IApi)
	reqOpts := http.HttpRequestOptions{
		RequestBody: requestBody,
		Encode:      api2.GetEncodeType(),
	}
	return entity.ApiPostWithOpts(path, reqOpts)
}

func (entity *Api) ApiGetWithOpts(path string, reqOpts http.HttpRequestOptions) (*ApiRes, error) {
	//GET请求写死为form
	reqOpts.Encode = http.EncodeForm
	if entity.Client == nil {
		zlog.Errorf(entity.GetCtx(), "ApiGetWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	res, e := entity.Client.HttpGet(entity.GetCtx(), path, reqOpts)
	if e != nil {
		return nil, e
	}
	return entity.handel(path, res)
}

func (entity *Api) ApiDeleteWithOpts(path string, reqOpts http.HttpRequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		zlog.Errorf(entity.GetCtx(), "ApiDeleteWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	res, e := entity.Client.HttpDelete(entity.GetCtx(), path, reqOpts)
	if e != nil {
		return nil, e
	}
	return entity.handel(path, res)
}

func (entity *Api) ApiPutWithOpts(path string, reqOpts http.HttpRequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		zlog.Errorf(entity.GetCtx(), "ApiPutWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	if reqOpts.Encode == "" {
		reqOpts.Encode = entity.GetEncodeType()
	}
	res, err := entity.Client.HttpPut(entity.GetCtx(), path, reqOpts)
	if err != nil {
		return nil, err
	}
	return entity.handel(path, res)
}

func (entity *Api) ApiPostWithOpts(path string, reqOpts http.HttpRequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		zlog.Errorf(entity.GetCtx(), "ApiPostWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	if reqOpts.Encode == "" {
		reqOpts.Encode = entity.GetEncodeType()
	}
	res, err := entity.Client.HttpPost(entity.GetCtx(), path, reqOpts)
	if err != nil {
		return nil, err
	}
	return entity.handel(path, res)
}

func (entity *Api) handel(path string, res *http.HttpResult) (*ApiRes, error) {
	if res.HttpCode > 200 {
		return nil, fmt.Errorf("api response status code: %d, message: %s", res.HttpCode, string(res.Response))
	}
	apiRes := &ApiRes{}
	if len(res.Response) > 0 {
		e := json.Unmarshal(res.Response, &apiRes)
		if e != nil {
			// 限制一下错误日志打印的长度，2k
			data := res.Response
			if len(data) > 2000 {
				data = data[0:2000]
			}
			// 返回数据json unmarshal失败，打印错误日志
			zlog.Errorf(entity.GetCtx(), "http response json unmarshal failed, path:%s, response:%s, err:%s", path, string(data), e)
			return nil, e
		}
	}
	return apiRes, nil
}

func (entity *Api) DecodeApiResponse(outPut interface{}, data *ApiRes, err error) error {
	if data.Code != 200 {
		return errors.NewError(data.Code, map[string]string{"zh": data.Message, "en": data.Message})
	}
	if len(data.Data) > 0 {
		// 解析数据
		if err = json.Unmarshal(data.Data, outPut); err != nil {
			zlog.Errorf(entity.GetCtx(), "api error, api response unmarshal, data:%s, err:%+v", data.Data, err.Error())
			return errors.ErrorSystemError
		}

	}
	return nil
}
