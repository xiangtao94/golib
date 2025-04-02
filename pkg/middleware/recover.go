package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegistryRecovery(engine *gin.Engine, handle gin.RecoveryFunc) {
	if handle == nil {
		handle = func(c *gin.Context, err interface{}) {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	engine.Use(gin.CustomRecovery(handle))
}
