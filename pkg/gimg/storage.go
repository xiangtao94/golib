// Package algo -----------------------------
// @file      : storage.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/6/9 11:18
// Description:
// -------------------------------------------
package gimg

import (
	"bytes"
	"context"
	"image"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/pierrre/imageserver"
)

type MinioStoreServer struct {
	Client *minio.Client
	Bucket string
}

func NewMinioStoreServer(bucket string, client *minio.Client) *MinioStoreServer {
	return &MinioStoreServer{
		Client: client,
		Bucket: bucket,
	}
}

func (s *MinioStoreServer) Get(params imageserver.Params) (*imageserver.Image, error) {
	source, err := params.GetString("source")
	if err != nil {
		return nil, err
	}
	im, err := s.loadImage(source)
	if err != nil {
		return nil, &imageserver.ParamError{Param: "source", Message: err.Error()}
	}
	params["format"] = im.Format
	return im, nil
}

func (s *MinioStoreServer) loadImage(imgKey string) (*imageserver.Image, error) {
	obj, err := s.Client.GetObject(context.Background(), s.Bucket, imgKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, obj)
	if err != nil {
		return nil, err
	}

	data := buf.Bytes()
	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, &imageserver.ImageError{Message: err.Error()}
	}
	return &imageserver.Image{Data: data, Format: format}, nil
}
