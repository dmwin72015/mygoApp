package server

import (
	"errors"
	"net/http"
	"strconv"

	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/gin-gonic/gin"
)

// handleAuthLogin handles POST /auth/login.
func (s *Server) handleAuthLogin(c *gin.Context) {
	lang := LangFromContext(c)
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
		return
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

// handleStatsGet handles GET /stats.
func (s *Server) handleStatsGet(c *gin.Context) {
	stats, err := s.repo.Stats(c.Request.Context())
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, stats)
}

// registerCollectionRoutes registers REST endpoints on a RouterGroup for a collection Store[T].
func registerCollectionRoutes[T any](
	rg *gin.RouterGroup,
	path string,
	perm domain.Permission,
	store domain.Store[T],
	setID func(*T, string),
) {
	// List
	rg.GET(path, RequirePermission(perm, false), func(c *gin.Context) {
		q := domain.Query{
			Search: c.Query("search"),
		}
		if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
			q.Limit = l
		}
		rows, err := store.List(c.Request.Context(), q)
		if err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if rows == nil {
			rows = []T{}
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	})

	// Get
	rg.GET(path+"/:id", RequirePermission(perm, false), func(c *gin.Context) {
		lang := LangFromContext(c)
		id := c.Param("id")
		row, err := store.Get(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeJSONError(c, http.StatusNotFound, i18n.Tr(lang, "err.not_found"))
				return
			}
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, row)
	})

	// Create
	rg.POST(path, RequirePermission(perm, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		var item T
		if err := c.ShouldBindJSON(&item); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if err := store.Save(c.Request.Context(), &item); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, item)
	})

	// Update
	rg.PUT(path+"/:id", RequirePermission(perm, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		id := c.Param("id")
		var item T
		if err := c.ShouldBindJSON(&item); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if setID != nil {
			setID(&item, id)
		}
		if err := store.Save(c.Request.Context(), &item); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, item)
	})

	// Delete
	rg.DELETE(path+"/:id", RequirePermission(perm, true), func(c *gin.Context) {
		id := c.Param("id")
		if err := store.Delete(c.Request.Context(), id); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

// registerUserRoutes registers routes for /users.
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
		var req struct {
			domain.User
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, "无效的请求格式")
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
		id := c.Param("id")
		curUser, _ := UserFromContext(c)

		if curUser.Role != string(domain.RoleAdmin) && curUser.ID != id {
			writeJSONError(c, http.StatusForbidden, "无权修改其他账号")
			return
		}

		var u domain.User
		if err := c.ShouldBindJSON(&u); err != nil {
			writeJSONError(c, http.StatusBadRequest, "无效的请求格式")
			return
		}
		u.ID = id
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
		id := c.Param("id")
		curUser, _ := UserFromContext(c)

		if curUser.Role != string(domain.RoleAdmin) && curUser.ID != id {
			writeJSONError(c, http.StatusForbidden, "无权修改其他用户密码")
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, "无效的请求格式")
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
		id := c.Param("id")
		curUser, _ := UserFromContext(c)
		if curUser.ID == id {
			writeJSONError(c, http.StatusBadRequest, "不能删除当前登录账号")
			return
		}
		if err := s.repo.Users().Delete(c.Request.Context(), id); err != nil {
			writeJSONError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

// registerLedgerRoutes registers routes for /ledger.
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
		var req moveReq
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, "无效请求格式")
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
		var req moveReq
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, "无效请求格式")
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

// registerSettingsRoutes registers routes for /settings.
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
		key := c.Param("key")
		var req struct {
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSONError(c, http.StatusBadRequest, "无效请求格式")
			return
		}
		if err := s.repo.Settings().Set(c.Request.Context(), key, req.Value); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"value": req.Value})
	})
}
