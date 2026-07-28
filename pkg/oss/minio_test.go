package oss

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinIORequiresHTTPSByDefault(t *testing.T) {
	_, err := newMinioClient(MinioConf{Endpoint: "http://localhost:9000"})
	require.Error(t, err)
}

func TestMinIOAcceptsHTTPSAndDoesNotMutateTLSConfig(t *testing.T) {
	source := &tls.Config{}
	client, err := newMinioClient(MinioConf{
		Endpoint:  "https://storage.example.com",
		TLSConfig: source,
	})

	require.NoError(t, err)
	require.NotNil(t, client)
	require.Zero(t, source.MinVersion)
}
