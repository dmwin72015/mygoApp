package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/gin-gonic/gin"
)

const (
	userKey      = "server_user"
	langKey      = "server_lang"
	requestIDKey = "request_id"
)

// RequestIDFromContext returns the unique request ID from Gin context.
func RequestIDFromContext(c *gin.Context) string {
	if val, exists := c.Get(requestIDKey); exists {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// RequestIDMiddleware injects or propagates X-Request-ID for distributed tracing and audit logs.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			var b [12]byte
			_, _ = rand.Read(b[:])
			reqID = "req_" + hex.EncodeToString(b[:])
		}
		c.Set(requestIDKey, reqID)
		c.Writer.Header().Set("X-Request-ID", reqID)
		c.Next()
	}
}

// MaxBodyBytesMiddleware limits request body size (e.g. 10MB) to protect against memory exhaustion.
func MaxBodyBytesMiddleware(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

// UserFromContext retrieves the authenticated user from the Gin context.
func UserFromContext(c *gin.Context) (domain.User, bool) {
	val, exists := c.Get(userKey)
	if !exists {
		return domain.User{}, false
	}
	u, ok := val.(domain.User)
	return u, ok
}

// LangFromContext retrieves the active language from the request context.
func LangFromContext(c *gin.Context) string {
	if val, exists := c.Get(langKey); exists {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return i18n.LangZH
}

// I18nMiddleware extracts and normalizes the language from Accept-Language header or ?lang=.
func I18nMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.Query("lang")
		if lang == "" {
			lang = c.GetHeader("Accept-Language")
		}
		c.Set(langKey, i18n.Normalize(lang))
		c.Next()
	}
}

// CORSMiddleware adds CORS headers to all responses and handles preflight requests.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Accept-Language")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// AuthMiddleware validates Bearer tokens and attaches the User to the Gin context.
func AuthMiddleware(tokens *TokenStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := LangFromContext(c)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSONError(c, http.StatusUnauthorized, i18n.Tr(lang, "err.unauthorized"))
			c.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		user, ok := tokens.Get(token)
		if !ok {
			writeJSONError(c, http.StatusUnauthorized, i18n.Tr(lang, "err.unauthorized"))
			c.Abort()
			return
		}
		c.Set(userKey, user)
		c.Next()
	}
}

// RequirePermission checks whether the authenticated user has sufficient permission.
func RequirePermission(perm domain.Permission, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := LangFromContext(c)
		user, ok := UserFromContext(c)
		if !ok {
			writeJSONError(c, http.StatusUnauthorized, i18n.Tr(lang, "err.unauthorized"))
			c.Abort()
			return
		}
		role := domain.ParseRole(user.Role)
		if write && !role.CanWrite(perm) {
			writeJSONError(c, http.StatusForbidden, i18n.Tr(lang, "err.forbidden"))
			c.Abort()
			return
		}
		if !write && !role.CanRead(perm) {
			writeJSONError(c, http.StatusForbidden, i18n.Tr(lang, "err.forbidden"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// writeJSONError writes a standardized error JSON response with request tracking ID.
func writeJSONError(c *gin.Context, status int, message string) {
	resp := gin.H{
		"error":   message,
		"message": message,
	}
	if reqID := RequestIDFromContext(c); reqID != "" {
		resp["request_id"] = reqID
	}
	c.JSON(status, resp)
}
