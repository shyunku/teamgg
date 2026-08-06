package platform

import (
	"github.com/gin-gonic/gin"
	"team.gg-server/controllers/middlewares"
)

func UsePlatformRouter(r *gin.RouterGroup) {
	g := r.Group("/platform")
	g.Use(middlewares.UnsafeAuthMiddleware)
	UseCustomGameRouter(g)
	UseStatisticsRouter(g)

	g.GET("/tokenTest", TestToken)
}

func TestToken(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "still alive",
	})
}
