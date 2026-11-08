package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handlePing handles GET /ping for simple heartbeat checks.
func (s *Server) handlePing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// handleHealth handles GET /health for container/orchestrator liveness & readiness probes.
func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "up"
	if s.repo != nil {
		if _, err := s.repo.Stats(ctx); err != nil {
			dbStatus = "down"
		}
	} else {
		dbStatus = "down"
	}

	source := "unconnected"
	if s.repo != nil {
		source = s.repo.Source()
	}

	status := http.StatusOK
	resp := gin.H{
		"status":    "ok",
		"db":        dbStatus,
		"source":    source,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if reqID := RequestIDFromContext(c); reqID != "" {
		resp["request_id"] = reqID
	}

	if dbStatus != "up" {
		resp["status"] = "degraded"
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, resp)
}
