package flow

import (
	"encoding/json"
	jsoniter "github.com/json-iterator/go"
	"github.com/tiant-go/golib/pkg/errors"
	"github.com/tiant-go/golib/pkg/http"
)

type Res struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    jsoniter.RawMessage `json:"data"`
}

type ApiRes struct {
	Code    int
	Message string
	Data    []byte
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
		entity.LogErrorf("ApiGetWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	res, e := entity.Client.HttpGet(entity.GetCtx(), path, reqOpts)
	if e != nil {
		return nil, e
	}
	return entity.handel(path, res)
}

func (entity *Api) ApiPostWithOpts(path string, reqOpts http.HttpRequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		entity.LogErrorf("ApiPostWithOpts failed, api client is needed, path:%s", path)
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
	httpRes := Res{}
	if len(res.Response) > 0 {
		e := jsoniter.Unmarshal(res.Response, &httpRes)
		if e != nil {
			// 限制一下错误日志打印的长度，2k
			data := res.Response
			if len(data) > 2000 {
				data = data[0:2000]
			}
			// 返回数据json unmarshal失败，打印错误日志
			entity.LogErrorf("http response json unmarshal failed, path:%s, response:%s, err:%s", path, string(data), e)
			return nil, e
		}
	}
	apiRes := &ApiRes{
		Code:    httpRes.Code,
		Message: httpRes.Message,
		Data:    httpRes.Data,
	}
	return apiRes, nil
}

func (entity *Api) DecodeApiResponse(outPut interface{}, data *ApiRes, err error) error {
	if err != nil {
		return err
	}

	if data.Code != 200 {
		return errors.Error{
			Code:    data.Code,
			Message: data.Message,
		}
	}
	if len(data.Data) > 0 {
		// 解析数据
		if err = json.Unmarshal(data.Data, outPut); err != nil {
			entity.LogErrorf("api error, api response unmarshal, data:%s, err:%+v", data.Data, err.Error())
			return errors.ErrorSystemError
		}

	}
	return nil
}
