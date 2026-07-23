package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
	"team.gg-server/controllers/middlewares"
	"team.gg-server/controllers/socket"
	"team.gg-server/controllers/test"
	v1 "team.gg-server/controllers/v1"
	"team.gg-server/core"
)

var GlobalLogger = log.GetLogger()

func SetupRouter() *gin.Engine {
	configureGinMode()
	gin.DefaultWriter = GlobalLogger
	gin.DefaultErrorWriter = GlobalLogger

	// setting cors
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{
		"http://localhost:8080",
		"https://teamgg.kr",
		"https://www.teamgg.kr",
		"https://d1ud9260py0fv1.cloudfront.net",
	}
	config.AllowCredentials = false
	config.AllowHeaders = []string{
		"Origin",
		"Content-Length",
		"Content-Type",
		"Authorization",
		"X-Requested-With",
		"Accept",
	}

	r := gin.Default()
	r.Use(cors.New(config))
	r.Use(middlewares.DefaultMiddleware)
	r.GET("/", serverVersion)
	r.GET("/ping", ping)

	// platform routes
	v1.UseV1Router(r)
	if !core.IsProduction {
		test.UseTestRouter(r)
	}
	socket.UseSocket(r)

	// 404
	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"code": 404, "message": "Page not found"})
	})

	return r
}

func configureGinMode() {
	// Preserve the explicit test mode selected by controller tests.
	if gin.Mode() == gin.TestMode {
		return
	}
	if core.IsProduction {
		gin.SetMode(gin.ReleaseMode)
		return
	}
	gin.SetMode(gin.DebugMode)
}

func serverVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":      core.Version,
		"isProduction": core.IsProduction,
	})
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

	// TLS is terminated by the deployment proxy. Environment selection must
	// not silently change the server transport.
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
		os.Exit(-3)
	}
}
