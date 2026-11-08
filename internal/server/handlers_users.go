package server

import (
	"fmt"
	"net/http"
	"strconv"

	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/gin-gonic/gin"
)

// registerUserRoutes registers routes for /users account management.
func (s *Server) registerUserRoutes(rg *gin.RouterGroup) {
	// List Users
	rg.GET("/users", RequirePermission(domain.PermUsers, false), func(c *gin.Context) {
		users, err := s.repo.Users().List(c.Request.Context())
		if err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if users == nil {
			users = []domain.User{}
		}
		c.JSON(http.StatusOK, gin.H{"items": users})
	})

	// Create User
	rg.POST("/users", RequirePermission(domain.PermUsers, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		var req struct {
			domain.User
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		user := req.User
		if err := s.repo.Users().Create(c.Request.Context(), &user, req.Password); err != nil {
			writeJSONError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, user)
	})

	// Update User
	rg.PUT("/users/:id", func(c *gin.Context) {
		lang := LangFromContext(c)
		id := c.Param("id")
		curUser, _ := UserFromContext(c)

		if curUser.Role != string(domain.RoleAdmin) && strconv.FormatInt(curUser.ID, 10) != id && curUser.Code != id {
			writeJSONError(c, http.StatusForbidden, i18n.Tr(lang, "err.no_perm_other_user"))
			return
		}

		var u domain.User
		if err := c.ShouldBindJSON(&u); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		u.SetIdentity(id)
		if curUser.Role != string(domain.RoleAdmin) {
			u.Role = curUser.Role // Prevent privilege escalation
		}
		if err := s.repo.Users().Update(c.Request.Context(), u); err != nil {
			writeJSONError(c, http.StatusBadRequest, err.Error())
			return
		}
		s.tokens.UpdateUser(u)
		c.JSON(http.StatusOK, u)
	})

	// Change Password
	rg.POST("/users/:id/password", func(c *gin.Context) {
		lang := LangFromContext(c)
		id := c.Param("id")
		curUser, _ := UserFromContext(c)

		if curUser.Role != string(domain.RoleAdmin) && strconv.FormatInt(curUser.ID, 10) != id && curUser.Code != id {
			writeJSONError(c, http.StatusForbidden, i18n.Tr(lang, "err.no_perm_other_user"))
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if err := s.repo.Users().SetPassword(c.Request.Context(), id, req.Password); err != nil {
			writeJSONError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Delete User
	rg.DELETE("/users/:id", RequirePermission(domain.PermUsers, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		id := c.Param("id")
		curUser, _ := UserFromContext(c)
		if fmt.Sprint(curUser.ID) == id || curUser.Code == id {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.cannot_delete_self"))
			return
		}
		if err := s.repo.Users().Delete(c.Request.Context(), id); err != nil {
			writeJSONError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

