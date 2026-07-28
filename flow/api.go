package flow

import (
	"context"
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

type Api struct {
	EncodeType string
	Client     *http.Client
}

// api请求数据格式，默认json
func (entity *Api) GetEncodeType() string {
	if entity.EncodeType != "" {
		return entity.EncodeType
	}
	return http.EncodeJson
}

func (entity *Api) ApiGet(ctx context.Context, path string, requestParam map[string]string) (*ApiRes, error) {
	reqOpts := http.RequestOptions{
		QueryParams: requestParam,
	}
	return entity.ApiGetWithOpts(ctx, path, reqOpts)
}

func (entity *Api) ApiDelete(ctx context.Context, path string, requestParam interface{}) (*ApiRes, error) {
	reqOpts := http.RequestOptions{
		RequestBody: requestParam,
		Encode:      http.EncodeForm,
	}
	return entity.ApiDeleteWithOpts(ctx, path, reqOpts)
}

func (entity *Api) ApiPut(ctx context.Context, path string, requestBody interface{}) (*ApiRes, error) {
	reqOpts := http.RequestOptions{
		RequestBody: requestBody,
		Encode:      entity.GetEncodeType(),
	}
	return entity.ApiPutWithOpts(ctx, path, reqOpts)
}

func (entity *Api) ApiPost(ctx context.Context, path string, requestBody interface{}) (*ApiRes, error) {
	reqOpts := http.RequestOptions{
		RequestBody: requestBody,
		Encode:      entity.GetEncodeType(),
	}
	return entity.ApiPostWithOpts(ctx, path, reqOpts)
}

func (entity *Api) ApiGetWithOpts(ctx context.Context, path string, reqOpts http.RequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		zlog.Errorf(ctx, "ApiGetWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	reqOpts.Path = path
	res, e := entity.Client.Get(ctx, reqOpts)
	if e != nil {
		return nil, e
	}
	return entity.handleContext(ctx, path, res)
}

func (entity *Api) ApiDeleteWithOpts(ctx context.Context, path string, reqOpts http.RequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		zlog.Errorf(ctx, "ApiDeleteWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	reqOpts.Path = path
	res, e := entity.Client.Delete(ctx, reqOpts)
	if e != nil {
		return nil, e
	}
	return entity.handleContext(ctx, path, res)
}

func (entity *Api) ApiPutWithOpts(ctx context.Context, path string, reqOpts http.RequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		zlog.Errorf(ctx, "ApiPutWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	if reqOpts.Encode == "" {
		reqOpts.Encode = entity.GetEncodeType()
	}
	reqOpts.Path = path
	res, err := entity.Client.Put(ctx, reqOpts)
	if err != nil {
		return nil, err
	}
	return entity.handleContext(ctx, path, res)
}

func (entity *Api) ApiPostWithOpts(ctx context.Context, path string, reqOpts http.RequestOptions) (*ApiRes, error) {
	if entity.Client == nil {
		zlog.Errorf(ctx, "ApiPostWithOpts failed, api client is needed, path:%s", path)
		return nil, errors.ErrorSystemError
	}
	if reqOpts.Encode == "" {
		reqOpts.Encode = entity.GetEncodeType()
	}
	reqOpts.Path = path
	res, err := entity.Client.Post(ctx, reqOpts)
	if err != nil {
		return nil, err
	}
	return entity.handleContext(ctx, path, res)
}

func (entity *Api) handleContext(ctx context.Context, path string, res *http.Result) (*ApiRes, error) {
	if res == nil {
		return nil, errors.ErrorSystemError
	}
	if res.HttpCode < 200 || res.HttpCode >= 300 {
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
			zlog.Errorf(ctx, "http response json unmarshal failed, path:%s, response:%s, err:%s", path, string(data), e)
			return nil, e
		}
	}
	return apiRes, nil
}

func (entity *Api) DecodeAPIResponse(ctx context.Context, output interface{}, data *ApiRes, err error) error {
	if err != nil {
		return err
	}
	if data == nil {
		return errors.ErrorSystemError
	}
	if data.Code != 0 && data.Code != 200 {
		return errors.NewError(data.Code, map[string]string{"zh": data.Message, "en": data.Message})
	}
	if output != nil && len(data.Data) > 0 {
		// 解析数据
		if err = json.Unmarshal(data.Data, output); err != nil {
			zlog.Errorf(ctx, "api error, api response unmarshal, data:%s, err:%+v", data.Data, err.Error())
			return errors.ErrorSystemError
		}

	}
	return nil
}
