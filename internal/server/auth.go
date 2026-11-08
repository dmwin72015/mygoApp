package server

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"mygo-app/internal/domain"
)

// Session represents an authenticated user's session.
type Session struct {
	User      domain.User
	CreatedAt time.Time
	ExpiresAt time.Time
}

// TokenStore manages authentication tokens and active user sessions.
type TokenStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
}

// NewTokenStore creates a new in-memory token session manager.
func NewTokenStore(ttl time.Duration) *TokenStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour // default 7 days
	}
	return &TokenStore{
		sessions: make(map[string]Session),
		ttl:      ttl,
	}
}

// Create generates a secure 32-byte token for the user and stores the session.
func (s *TokenStore) Create(user domain.User) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = Session{
		User:      user,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}
	return token, nil
}

// Get looks up a session by token. Returns false if not found or expired.
func (s *TokenStore) Get(token string) (domain.User, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[token]
	s.mu.RUnlock()

	if !ok {
		return domain.User{}, false
	}
	if time.Now().After(sess.ExpiresAt) {
		s.Revoke(token)
		return domain.User{}, false
	}
	return sess.User, true
}

// Revoke removes a token from the store.
func (s *TokenStore) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// UpdateUser updates the cached user info across all sessions for that user ID.
func (s *TokenStore) UpdateUser(u domain.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for t, sess := range s.sessions {
		if sess.User.ID == u.ID {
			sess.User = u
			s.sessions[t] = sess
		}
	}
}
