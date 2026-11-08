package server

import (
	"net/http"

	"mygo-app/internal/i18n"

	"github.com/gin-gonic/gin"
)

// handleStatsGet handles GET /stats for the dashboard summary.
func (s *Server) handleStatsGet(c *gin.Context) {
	stats, err := s.repo.Stats(c.Request.Context())
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, stats)
}

// registerSettingsRoutes registers routes for /settings key-value configuration.
func (s *Server) registerSettingsRoutes(rg *gin.RouterGroup) {
	rg.GET("/settings/:key", func(c *gin.Context) {
		key := c.Param("key")
		val, err := s.repo.Settings().Get(c.Request.Context(), key)
		if err != nil || val == "" {
			writeJSONError(c, http.StatusNotFound, "not found")
			return
		}
		c.JSON(http.StatusOK, gin.H{"value": val})
	})

	rg.PUT("/settings/:key", func(c *gin.Context) {
		lang := LangFromContext(c)
		key := c.Param("key")
		var req struct {
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if err := s.repo.Settings().Set(c.Request.Context(), key, req.Value); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"value": req.Value})
	})
}
