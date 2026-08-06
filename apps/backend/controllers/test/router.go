package test

import (
	"github.com/gin-gonic/gin"
	"team.gg-server/third_party/riot"
)

func UseTestRouter(r *gin.Engine) {
	g := r.Group("/test")
	g.GET("/riotApiCalls", GetRiotApiCalls)
}

func GetRiotApiCalls(c *gin.Context) {
	c.JSON(200, riot.ApiCalls)
}
