// Package algo -----------------------------
// @file      : oss.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/6/9 11:39
// Description:
// -------------------------------------------
package oss

import (
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/xiangtao94/golib/pkg/errors"
	"net/url"
)

type MinioConf struct {
	AK       string `yaml:"ak"`
	SK       string `yaml:"sk"`
	Endpoint string `yaml:"endpoint"`
}

func NewMClientByAK(endpoint string, accessKey, secretKey string) (*minio.Client, error) {
	endpointUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.ErrorSystemError
	}
	minioClient, err := minio.New(endpointUrl.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return minioClient, nil
}
