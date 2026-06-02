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
	allowed, _ := l.AllowWithRetry(key, now)
	return allowed
}

func (l *RateLimiter) AllowWithRetry(key string, now time.Time) (bool, time.Duration) {
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
		return true, 0
	}
	if entry.Count >= l.limit {
		return false, retryAfterDuration(entry.ExpiresAt, now)
	}
	entry.Count++
	return true, 0
}
