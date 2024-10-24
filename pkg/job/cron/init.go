package cron

import (
	"github.com/gin-gonic/gin"
)

func InitCrontab(g *gin.Engine) (c *Cron) {
	c = New(g)
	c.Start()
	return c
}
