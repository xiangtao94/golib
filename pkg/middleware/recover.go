package middleware

import (
	"errors"
	"github.com/xiangtao94/golib/pkg/zlog"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegistryRecovery(engine *gin.Engine, handle gin.RecoveryFunc) {
	if handle == nil {
		handle = func(c *gin.Context, err interface{}) {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	//engine.Use(CustomRecoveryWithZap(zlog.NewLoggerWithSkip(1), handle))
}

func CustomRecoveryWithZap(logger *zap.Logger, handle gin.RecoveryFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					var se *os.SyscallError
					if errors.As(ne, &se) {
						seStr := strings.ToLower(se.Error())
						if strings.Contains(seStr, "broken pipe") ||
							strings.Contains(seStr, "connection reset by peer") {
							brokenPipe = true
						}
					}
				}
				if brokenPipe {
					logger.Error("Broken pipe or connection reset by peer",
						zap.Any("error", err),
					)
					c.Error(err.(error)) // 记录 gin 错误
					c.Abort()
					return
				}
				logger = zlog.LoggerWithContext(logger, c)
				// 正常 panic 情况
				logger.Error("Panic Recovery",
					zap.Any("error", err),
					zap.Any("stack", string(debug.Stack())),
				)
				handle(c, err)
			}
		}()
		c.Next()
	}
}
