// Package algo -----------------------------
// @file      : handler.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/6/9 11:18
// Description:
// -------------------------------------------
package gimg

import (
	"github.com/disintegration/gift"
	"github.com/pierrre/imageserver/image"
	"github.com/pierrre/imageserver/image/crop"
	imageserver_image_gift "github.com/pierrre/imageserver/image/gift"
)

func NewImageHandler() *image.Handler {
	return &image.Handler{
		Processor: image.ListProcessor([]image.Processor{
			&crop.Processor{},
			&imageserver_image_gift.ResizeProcessor{
				DefaultResampling: gift.NearestNeighborResampling,
				MaxWidth:          1024,
				MaxHeight:         1024,
			},
			&imageserver_image_gift.RotateProcessor{
				DefaultInterpolation: gift.NearestNeighborInterpolation,
			},
		}),
	}
}
