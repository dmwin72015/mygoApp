package server

import (
	"context"
	"net/http"
	"time"

	"mygo-app/internal/domain"

	"github.com/gin-gonic/gin"
)

// Server is the HTTP REST API server powered by Gin.
type Server struct {
	repo   domain.Repo
	tokens *TokenStore
	engine *gin.Engine
}

// Option configures Server options.
type Option func(*Server)

// WithTokenTTL configures session expiration duration.
func WithTokenTTL(ttl time.Duration) Option {
	return func(s *Server) {
		s.tokens = NewTokenStore(ttl)
	}
}

// New creates and configures the Gin-powered HTTP Server.
func New(repo domain.Repo, opts ...Option) *Server {
	s := &Server{
		repo:   repo,
		tokens: NewTokenStore(7 * 24 * time.Hour),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Initialize Gin engine
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), CORSMiddleware(), I18nMiddleware())

	// 1. Public endpoints
	r.POST("/auth/login", s.handleAuthLogin)

	// 2. Protected endpoints (Bearer Token verified)
	api := r.Group("/", AuthMiddleware(s.tokens))

	// Overview Stats (all authenticated users)
	api.GET("/stats", s.handleStatsGet)

	// Collection Stores
	registerCollectionRoutes(api, "/students", domain.PermStudents, s.repo.Students(),
		func(v *domain.Student, id string) { v.ID = id })

	registerCollectionRoutes(api, "/teachers", domain.PermTeachers, s.repo.Teachers(),
		func(v *domain.Teacher, id string) { v.ID = id })

	registerCollectionRoutes(api, "/classrooms", domain.PermClassrooms, s.repo.Classrooms(),
		func(v *domain.Classroom, id string) { v.ID = id })

	registerCollectionRoutes(api, "/courses", domain.PermCourses, s.repo.Courses(),
		func(v *domain.Course, id string) { v.ID = id })

	registerCollectionRoutes(api, "/lessons", domain.PermLessons, s.repo.Lessons(),
		func(v *domain.Lesson, id string) { v.ID = id })

	// Account management, Ledger and Settings
	s.registerUserRoutes(api)
	s.registerLedgerRoutes(api)
	s.registerSettingsRoutes(api)

	s.engine = r
	return s
}

// ServeHTTP implements http.Handler, delegating to the underlying Gin engine.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}

// Engine returns the underlying Gin engine.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Tokens returns the server's token store, useful in tests.
func (s *Server) Tokens() *TokenStore {
	return s.tokens
}

// Shutdown gracefully closes the data repository.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.repo != nil {
		return s.repo.Close()
	}
	return nil
}
