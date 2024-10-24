package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Recovery(handle gin.RecoveryFunc) gin.HandlerFunc {
	if handle == nil {
		handle = func(c *gin.Context, err interface{}) {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	return gin.CustomRecovery(handle)
}
