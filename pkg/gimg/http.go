// Package algo -----------------------------
// @file      : http.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/6/9 11:25
// Description:
// -------------------------------------------
package gimg

import (
	"log"
	"net/http"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/pierrre/imageserver"
	imageserver_http "github.com/pierrre/imageserver/http"
	imageserver_http_crop "github.com/pierrre/imageserver/http/crop"
	imageserver_http_gamma "github.com/pierrre/imageserver/http/gamma"
	imageserver_http_gift "github.com/pierrre/imageserver/http/gift"
	imageserver_http_image "github.com/pierrre/imageserver/http/image"
)

func RegisterImageHTTPHandler(engine *gin.Engine, path string, server imageserver.Server) {
	if server == nil {
		log.Println(syscall.Getpid(), "[ImageServer] nil, skip HTTP handler registration")
		return
	}
	handler := func(ctx *gin.Context) {
		h := imageserver_http.Handler{
			Parser: imageserver_http.ListParser([]imageserver_http.Parser{
				&imageserver_http.SourcePathParser{},
				&imageserver_http_crop.Parser{},
				&imageserver_http_image.FormatParser{},
				&imageserver_http_image.QualityParser{},
				&imageserver_http_gift.RotateParser{},
				&imageserver_http_gift.ResizeParser{},
				&imageserver_http_gamma.CorrectionParser{},
			}),
			Server: server,
			ErrorFunc: func(err error, req *http.Request) {
				log.Printf("[ImageHTTP Error] %v", err)
			},
		}
		h.ServeHTTP(ctx.Writer, ctx.Request)
	}

	engine.GET(path, handler)
	engine.HEAD(path, handler)
}
