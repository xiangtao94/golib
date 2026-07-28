package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func NewCORS(config CORSConfig) (gin.HandlerFunc, error) {
	allowedOrigins := make(map[string]struct{}, len(config.AllowOrigins))
	for _, origin := range config.AllowOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			return nil, errors.New("cors: wildcard origin is not allowed; configure an explicit allowlist")
		}
		allowedOrigins[origin] = struct{}{}
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	}
	if len(config.AllowHeaders) == 0 {
		config.AllowHeaders = []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"}
	}
	allowedMethods := make(map[string]struct{}, len(config.AllowMethods))
	for _, method := range config.AllowMethods {
		allowedMethods[strings.ToUpper(strings.TrimSpace(method))] = struct{}{}
	}
	allowedHeaders := make(map[string]struct{}, len(config.AllowHeaders))
	for _, header := range config.AllowHeaders {
		allowedHeaders[strings.ToLower(strings.TrimSpace(header))] = struct{}{}
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if origin == "" {
			ctx.Next()
			return
		}

		addVary(ctx.Writer.Header(), "Origin")
		addVary(ctx.Writer.Header(), "Access-Control-Request-Method")
		addVary(ctx.Writer.Header(), "Access-Control-Request-Headers")

		_, exactMatch := allowedOrigins[origin]
		if !exactMatch {
			ctx.AbortWithStatus(http.StatusForbidden)
			return
		}
		requestedMethod := ctx.Request.Method
		if requestedMethod == http.MethodOptions {
			requestedMethod = ctx.GetHeader("Access-Control-Request-Method")
		}
		if _, allowed := allowedMethods[strings.ToUpper(requestedMethod)]; requestedMethod != "" && !allowed {
			ctx.AbortWithStatus(http.StatusForbidden)
			return
		}
		if ctx.Request.Method == http.MethodOptions {
			for _, header := range strings.Split(ctx.GetHeader("Access-Control-Request-Headers"), ",") {
				header = strings.ToLower(strings.TrimSpace(header))
				if header == "" {
					continue
				}
				if _, allowed := allowedHeaders[header]; !allowed {
					ctx.AbortWithStatus(http.StatusForbidden)
					return
				}
			}
		}
		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
		ctx.Header("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
		if len(config.ExposeHeaders) > 0 {
			ctx.Header("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
		}
		if config.AllowCredentials {
			ctx.Header("Access-Control-Allow-Credentials", "true")
		}
		if config.MaxAge > 0 {
			ctx.Header("Access-Control-Max-Age", strconv.FormatInt(int64(config.MaxAge/time.Second), 10))
		}
		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}, nil
}
