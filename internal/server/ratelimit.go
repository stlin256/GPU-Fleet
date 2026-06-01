package server

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]*rateEntry
}

type rateEntry struct {
	Count     int
	ExpiresAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		clients: map[string]*rateEntry{},
	}
}

func (l *RateLimiter) Allow(key string, now time.Time) bool {
	if key == "" {
		key = "unknown"
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	for client, entry := range l.clients {
		if now.After(entry.ExpiresAt) {
			delete(l.clients, client)
		}
	}
	entry := l.clients[key]
	if entry == nil || now.After(entry.ExpiresAt) {
		l.clients[key] = &rateEntry{Count: 1, ExpiresAt: now.Add(l.window)}
		return true
	}
	if entry.Count >= l.limit {
		return false
	}
	entry.Count++
	return true
}
