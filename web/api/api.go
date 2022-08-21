package api

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/logging"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

var logger = logging.GetLogger()

func setupRouter() *gin.Engine {
	// Disable Console Color
	// gin.DisableConsoleColor()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Output: logging.Output,
	}), gin.Recovery())

	router.GET("/process", viewProcesses)
	router.GET("/overview", overview)
	router.GET("/connections", viewConnections)
	router.GET("/api/export/simple", simpleView) // mainly for Ambot
	router.GET("/metrics", export2prometheus)

	return router
}

func StartWebServer(port uint16, errChan chan<- error) {
	r := setupRouter()
	err := r.Run(fmt.Sprintf(":%d", port))
	if err != nil {
		errChan <- errors.Wrap(err, "error when start api server")
	}
}
