// Package gimg -----------------------------
// @file      : server.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/4/15 18:17
// -------------------------------------------
package gimg

import (
	"github.com/minio/minio-go/v7"
	"github.com/pierrre/imageserver"
	"github.com/pierrre/imageserver/cache"
	"github.com/pierrre/imageserver/cache/memory"
)

type ImageServer struct {
	server  imageserver.Server
	handler imageserver.Handler
}

func NewImageServer(cfg ImageServerConfig, client *minio.Client) *ImageServer {
	store := NewMinioStoreServer(cfg.BucketName, client)
	cachedServer := &cache.Server{
		Server:       store,
		Cache:        memory.New(cfg.CacheSize),
		KeyGenerator: NewSourceHashKeyGenerator(),
	}
	return &ImageServer{
		server:  cachedServer,
		handler: NewImageHandler(),
	}
}

func (s *ImageServer) Get(params imageserver.Params) (*imageserver.Image, error) {
	params, err := s.transformParams(params)
	if err != nil {
		return nil, err
	}
	im, err := s.server.Get(params)
	if err != nil {
		return nil, err
	}
	return s.handler.Handle(im, params)
}

func (s *ImageServer) GetByImageId(imgKey string) (*imageserver.Image, error) {
	return s.Get(imageserver.Params{"source": imgKey})
}

func (s *ImageServer) transformParams(params imageserver.Params) (imageserver.Params, error) {
	source, err := params.GetString("source")
	if err != nil {
		return nil, err
	}
	params["source"] = SanitizeImageKey(source)
	return params, nil
}
