package server

import (
	"net/http"
	"sync"
	"time"

	"gpufleet/internal/auth"
)

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: map[string]time.Time{},
		ttl:      ttl,
	}
}

func (s *SessionStore) Create(w http.ResponseWriter, secure bool) error {
	token, err := auth.RandomToken(32)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(s.ttl)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     "gpufleet_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(s.ttl),
	})
	return nil
}

func (s *SessionStore) Clear(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("gpufleet_session"); err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "gpufleet_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *SessionStore) Valid(r *http.Request) bool {
	cookie, err := r.Cookie("gpufleet_session")
	if err != nil || cookie.Value == "" {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.sessions[cookie.Value]
	if !ok {
		return false
	}
	if now.After(expires) {
		delete(s.sessions, cookie.Value)
		return false
	}
	s.sessions[cookie.Value] = now.Add(s.ttl)
	return true
}
