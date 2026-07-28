package flow

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	httpclient "github.com/xiangtao94/golib/pkg/http"
)

func TestAPIHandleAcceptsEverySuccessfulHTTPStatus(t *testing.T) {
	api := &Api{}

	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusNoContent} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			response, err := api.handle("/resource", &httpclient.Result{
				HttpCode: status,
				Response: []byte(`{"code":200,"message":"ok"}`),
			})

			require.NoError(t, err)
			require.NotNil(t, response)
		})
	}
}

func TestAPIHandleRejectsNilResponse(t *testing.T) {
	api := &Api{}

	response, err := api.handle("/resource", nil)

	require.Nil(t, response)
	require.Error(t, err)
}

func TestDecodeAPIResponseReturnsTransportErrorFirst(t *testing.T) {
	api := &Api{}
	transportErr := errors.New("transport failed")

	require.ErrorIs(t, api.DecodeApiResponse(nil, nil, transportErr), transportErr)
}
