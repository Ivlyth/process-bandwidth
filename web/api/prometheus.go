package api

import (
	"context"
	"fmt"
	prom2 "github.com/Ivlyth/process-bandwidth/web/prom"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

type Server struct {
	limiter     *rate.Limiter
	promhandler http.HandlerFunc
}

type prometheusLogger struct{}

func (prometheusLogger) Println(v ...interface{}) {
	// TODO caller skip ?
	logger.Debugf("Prometheus Error: %s", fmt.Sprint(v...))
}

func New() *Server {
	registry := prometheus.NewRegistry()

	puller := prom2.NewPuller()
	puller.Add(prom2.NewProcessBandwidthPuller())

	registry.Register(puller)
	promhandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:          prometheusLogger{},
		EnableOpenMetrics: false,
	}).ServeHTTP

	return &Server{
		limiter:     rate.NewLimiter(1.0/1, 1),
		promhandler: promhandler,
	}
}

var server = New()

func export2prometheus(c *gin.Context) {
	// rate limit

	ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
	defer cancel()
	if err := server.limiter.Wait(ctx); err != nil {
		c.Writer.WriteHeader(http.StatusTooManyRequests)
		c.Writer.Write([]byte("Too Many Requests. Rate Limit: 1req/1s"))
		return
	}

	server.promhandler(c.Writer, c.Request)
}
