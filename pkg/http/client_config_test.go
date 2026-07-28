package http

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPClientOmitsBodiesByDefault(t *testing.T) {
	client := &ClientConf{}

	require.NoError(t, client.initClient())
	require.Equal(t, -1, client.MaxReqBodyLen)
	require.Equal(t, -1, client.MaxRespBodyLen)
}

func TestHTTPClientInitializationErrorIsSticky(t *testing.T) {
	client := &ClientConf{Domains: []string{"://invalid"}}

	first := client.initClient()
	second := client.initClient()

	require.Error(t, first)
	require.Error(t, second)
	require.Nil(t, client.HTTPClient)
}
