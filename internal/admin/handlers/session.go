package handlers

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]time.Time)}
}

func (s *SessionStore) Create() string {
	token := uuid.NewString()
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	return token
}

func (s *SessionStore) Valid(token string) bool {
	s.mu.Lock()
	exp, ok := s.sessions[token]
	s.mu.Unlock()
	return ok && time.Now().Before(exp)
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}
