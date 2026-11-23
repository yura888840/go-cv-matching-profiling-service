package httpapi

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/service"
	"net/http"
	"time"
)

type HealthChecker interface{ Ping(context.Context) error }

func Router(s *service.MatchingService, db HealthChecker, allowlist *Allowlist, apiKey string) *gin.Engine {
	r := gin.Default()
	r.Use(allowlist.Middleware())
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "database": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "database": "ok"})
	})
	r.POST("/api/v1/matches", APIKeyMiddleware(apiKey), func(c *gin.Context) {
		var req service.Request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		start := time.Now()
		result, err := s.Match(c.Request.Context(), req)
		if err != nil {
			status := 400
			if errors.Is(err, service.ErrNotFound) {
				status = 404
			} else if err.Error() == "context canceled" {
				status = 408
			} else if !isValidation(err) {
				status = 500
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": result, "duration_ms": float64(time.Since(start).Microseconds()) / 1000})
	})
	return r
}
func isValidation(err error) bool {
	switch err.Error() {
	case "invalid vacancy id", "top must be 1..1000", "workers must be 1..64", "unsupported mode":
		return true
	}
	return len(err.Error()) >= 17 && err.Error()[:17] == "cv_limit must be "
}
