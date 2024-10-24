package cycle

import (
	"github.com/gin-gonic/gin"
)

func InitCycle(g *gin.Engine) (c *Cycle) {
	c = New(g)
	return c
}
