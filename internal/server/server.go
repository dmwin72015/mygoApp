package server

import (
	"context"
	"net/http"
	"time"

	"mygo-app/internal/captcha"
	"mygo-app/internal/domain"

	"github.com/gin-gonic/gin"
)

// Server is the HTTP REST API server powered by Gin.
type Server struct {
	repo        domain.Repo
	tokens      *TokenStore
	engine      *gin.Engine
	captcha     captcha.Service
	skipCaptcha bool
}

// Option configures Server options.
type Option func(*Server)

// WithTokenTTL configures session expiration duration.
func WithTokenTTL(ttl time.Duration) Option {
	return func(s *Server) {
		s.tokens = NewTokenStore(ttl)
	}
}

// WithCaptchaService configures a custom captcha service.
func WithCaptchaService(svc captcha.Service) Option {
	return func(s *Server) {
		s.captcha = svc
	}
}

// WithSkipCaptcha configures whether login captcha verification is bypassed (e.g. for testing).
func WithSkipCaptcha(skip bool) Option {
	return func(s *Server) {
		s.skipCaptcha = skip
	}
}

// New creates and configures the Gin-powered HTTP Server.
func New(repo domain.Repo, opts ...Option) *Server {
	s := &Server{
		repo:    repo,
		tokens:  NewTokenStore(7 * 24 * time.Hour),
		captcha: captcha.NewMathCaptchaService(),
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

// Captcha returns the server's captcha service.
func (s *Server) Captcha() captcha.Service {
	return s.captcha
}

// Shutdown gracefully closes the data repository.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.repo != nil {
		return s.repo.Close()
	}
	return nil
}
