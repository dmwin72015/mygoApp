package server

import (
	"mygo-app/internal/domain"

	"github.com/gin-gonic/gin"
)

// setupRoutes mounts all public and authenticated route groups.
func (s *Server) setupRoutes(r *gin.Engine) {
	// Global Middlewares: Panic recovery, Request logging, Tracing, CORS, i18n, Max payload size
	r.Use(
		gin.Recovery(),
		gin.Logger(),
		RequestIDMiddleware(),
		MaxBodyBytesMiddleware(10<<20), // 10 MB payload limit
		CORSMiddleware(),
		I18nMiddleware(),
	)

	// 1. Health & Ping (Public, probe-friendly)
	r.GET("/ping", s.handlePing)
	r.GET("/health", s.handleHealth)

	// 2. Authentication & Registration (Public)
	auth := r.Group("/auth")
	{
		auth.GET("/captcha", s.handleAuthCaptcha)
		auth.POST("/login", s.handleAuthLogin)
		auth.POST("/reset-password", s.handleAuthResetPassword)
		auth.POST("/register", s.handleAuthRegister)
	}

	// 3. Protected Business API (Bearer token verified)
	api := r.Group("/", AuthMiddleware(s.tokens))
	{
		// 3.1 Overview Stats
		api.GET("/stats", s.handleStatsGet)

		// 3.2 Collection Stores (Generic CRUD)
		s.registerCollections(api)

		// 3.3 Domain Submodules
		s.registerUserRoutes(api)
		s.registerLedgerRoutes(api)
		s.registerSettingsRoutes(api)
	}
}

// registerCollections registers REST routes for all core business collections.
func (s *Server) registerCollections(api *gin.RouterGroup) {
	registerCollectionRoutes(api, "/students", domain.PermStudents, s.repo.Students())
	registerCollectionRoutes(api, "/teachers", domain.PermTeachers, s.repo.Teachers())
	registerCollectionRoutes(api, "/classrooms", domain.PermClassrooms, s.repo.Classrooms())
	registerCollectionRoutes(api, "/courses", domain.PermCourses, s.repo.Courses())
	registerCollectionRoutes(api, "/lessons", domain.PermLessons, s.repo.Lessons())
}
