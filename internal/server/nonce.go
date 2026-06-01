package server

import (
	"sync"
	"time"
)

type NonceStore struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	window time.Duration
}

func NewNonceStore(window time.Duration) *NonceStore {
	return &NonceStore{
		seen:   map[string]time.Time{},
		window: window,
	}
}

func (s *NonceStore) Accept(deviceID, nonce string, now time.Time) bool {
	key := deviceID + ":" + nonce
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, expires := range s.seen {
		if now.After(expires) {
			delete(s.seen, k)
		}
	}
	if _, exists := s.seen[key]; exists {
		return false
	}
	s.seen[key] = now.Add(s.window)
	return true
}
