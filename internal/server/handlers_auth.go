package server

import (
	"errors"
	"net/http"
	"strings"

	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/gin-gonic/gin"
)

// handleAuthCaptcha handles GET /auth/captcha.
func (s *Server) handleAuthCaptcha(c *gin.Context) {
	if s.captcha == nil {
		writeJSONError(c, http.StatusNotImplemented, "captcha service not configured")
		return
	}
	item, err := s.captcha.Generate()
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, item)
}

// handleAuthLogin handles POST /auth/login.
func (s *Server) handleAuthLogin(c *gin.Context) {
	lang := LangFromContext(c)
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		CaptchaID   string `json:"captcha_id"`
		CaptchaCode string `json:"captcha_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
		return
	}

	// Verify math arithmetic graphic captcha if enabled
	if !s.skipCaptcha && s.captcha != nil {
		if strings.TrimSpace(req.CaptchaCode) == "" {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.captcha_req"))
			return
		}
		if !s.captcha.Verify(req.CaptchaID, req.CaptchaCode) {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.bad_captcha"))
			return
		}
	}

	user, err := s.repo.Auth().Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrBadCredentials) {
			writeJSONError(c, http.StatusUnauthorized, i18n.Tr(lang, "err.bad_credentials"))
			return
		}
		writeJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}

	token, err := s.tokens.Create(user)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

// handleAuthResetPassword handles POST /auth/reset-password.
func (s *Server) handleAuthResetPassword(c *gin.Context) {
	lang := LangFromContext(c)
	var req struct {
		Username    string `json:"username"`
		RecoveryKey string `json:"recovery_key"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
		return
	}
	err := s.repo.Auth().ResetPassword(c.Request.Context(), req.Username, req.RecoveryKey, req.NewPassword)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRecoveryKey) {
			writeJSONError(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrUserNotFound) {
			writeJSONError(c, http.StatusNotFound, err.Error())
			return
		}
		writeJSONError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": i18n.Tr(lang, "msg.pwd_reset_success"),
	})
}

// handleAuthRegister handles POST /auth/register.
func (s *Server) handleAuthRegister(c *gin.Context) {
	lang := LangFromContext(c)
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.username_req"))
		return
	}
	if len(req.Password) < 6 {
		writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.password_min"))
		return
	}
	user, err := s.repo.Auth().Register(c.Request.Context(), req.Username, req.Password, req.DisplayName)
	if err != nil {
		if strings.Contains(err.Error(), "exists") || strings.Contains(err.Error(), "存在") || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			writeJSONError(c, http.StatusConflict, i18n.Tr(lang, "err.user_exists"))
			return
		}
		writeJSONError(c, http.StatusBadRequest, err.Error())
		return
	}

	token, err := s.tokens.Create(user)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":   token,
		"user":    user,
		"message": i18n.Tr(lang, "msg.register_success"),
	})
}
