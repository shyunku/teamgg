package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
	"net/http"
	"os"
	"sync"
	"team.gg-server/controllers/middlewares"
	"team.gg-server/controllers/socket"
	"team.gg-server/controllers/test"
	v1 "team.gg-server/controllers/v1"
	"team.gg-server/core"
)

var GlobalLogger = log.GetLogger()

func SetupRouter() *gin.Engine {
	gin.DefaultWriter = GlobalLogger
	gin.DefaultErrorWriter = GlobalLogger

	// setting cors
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{
		"http://localhost:8080",
		"http://team-gg.net-temp.s3-website.ap-northeast-2.amazonaws.com",
		"https://team-gg.net.s3-website.ap-northeast-2.amazonaws.com",
		"https://team-gg.net",
		"https://www.team-gg.net",
		"https://dwe4cvxze1hsa.cloudfront.net",
	}
	config.AllowCredentials = true
	//config.AllowHeaders = []string{
	//	"Origin",
	//	"Content-Length",
	//	"Content-Type",
	//	"Authorization",
	//}

	r := gin.Default()
	r.Use(cors.New(config))
	r.Use(middlewares.DefaultMiddleware)
	r.GET("/ping", ping)

	// platform routes
	v1.UseV1Router(r)
	if core.DebugMode {
		test.UseTestRouter(r)
	}
	socket.UseSocket(r)

	// 404
	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"code": 404, "message": "Page not found"})
	})

	return r
}

func ping(c *gin.Context) {
	c.String(200, "pong")
}

func RunGin(ctx context.Context, waitGroup *sync.WaitGroup) {
	log.Infof("Starting server on port on %s...", core.AppServerPort)
	r := SetupRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", core.AppServerPort),
		Handler: r,
	}

	go func() {
		<-ctx.Done()
		log.Info("server is shutting down...")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Fatalf("server shutdown failed:%+v", err)
		}
		log.Info("server shutdown complete.")
		waitGroup.Done()
	}()

	// 서버 시작
	if core.DebugMode || true {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
			os.Exit(-3)
		}
	} else {
		if err := srv.ListenAndServeTLS("certificates/cert.pem", "certificates/key.pem"); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
			os.Exit(-3)
		}
	}
}
