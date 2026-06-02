package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"gpufleet/internal/auth"
)

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
	ttl      time.Duration
	meta     *MetadataStore
}

func NewSessionStore(ttl time.Duration, meta *MetadataStore) *SessionStore {
	return &SessionStore{
		sessions: map[string]time.Time{},
		ttl:      ttl,
		meta:     meta,
	}
}

func (s *SessionStore) Create(w http.ResponseWriter, secure bool) error {
	token, err := auth.RandomToken(32)
	if err != nil {
		return err
	}
	now := time.Now()
	expires := now.Add(s.ttl)
	s.mu.Lock()
	s.sessions[token] = expires
	s.mu.Unlock()
	if s.meta != nil {
		if err := s.meta.SaveWebSession(sessionTokenHash(token), now, expires); err != nil {
			s.mu.Lock()
			delete(s.sessions, token)
			s.mu.Unlock()
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "gpufleet_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.ttl.Seconds()),
		Expires:  expires,
	})
	return nil
}

func (s *SessionStore) Clear(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("gpufleet_session"); err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
		if s.meta != nil {
			_ = s.meta.DeleteWebSession(sessionTokenHash(cookie.Value))
		}
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
	if s.validMemorySession(cookie.Value, now) {
		return true
	}
	if s.meta == nil {
		return false
	}
	session, ok := s.meta.WebSession(sessionTokenHash(cookie.Value), now)
	if !ok {
		return false
	}
	s.mu.Lock()
	s.sessions[cookie.Value] = session.ExpiresAt
	s.mu.Unlock()
	return true
}

func (s *SessionStore) validMemorySession(token string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.sessions[token]
	if !ok {
		return false
	}
	if !now.Before(expires) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func sessionTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
