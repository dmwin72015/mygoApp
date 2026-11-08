package server

import (
	"net/http"
	"strconv"

	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/gin-gonic/gin"
)

// registerLedgerRoutes registers routes for /ledger financial accounting.
func (s *Server) registerLedgerRoutes(rg *gin.RouterGroup) {
	rg.GET("/ledger", RequirePermission(domain.PermBilling, false), func(c *gin.Context) {
		studentID := c.Query("studentId")
		limit := 0
		if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
			limit = l
		}
		entries, err := s.repo.Ledger().List(c.Request.Context(), studentID, limit)
		if err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if entries == nil {
			entries = []domain.LedgerEntry{}
		}
		c.JSON(http.StatusOK, gin.H{"items": entries})
	})

	type moveReq struct {
		StudentID string `json:"studentId"`
		Amount    int64  `json:"amount"`
		Note      string `json:"note"`
	}

	rg.POST("/ledger/topup", RequirePermission(domain.PermBilling, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		var req moveReq
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if req.Amount <= 0 {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.amount_positive"))
			return
		}
		entry, err := s.repo.Ledger().TopUp(c.Request.Context(), req.StudentID, req.Amount, req.Note)
		if err != nil {
			writeJSONError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, entry)
	})

	rg.POST("/ledger/consume", RequirePermission(domain.PermBilling, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		var req moveReq
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if req.Amount <= 0 {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.amount_positive"))
			return
		}
		entry, err := s.repo.Ledger().Consume(c.Request.Context(), req.StudentID, req.Amount, req.Note)
		if err != nil {
			writeJSONError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, entry)
	})
}
