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

	// Initialize Gin engine and mount modular route groups
	r := gin.New()
	s.setupRoutes(r)
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
