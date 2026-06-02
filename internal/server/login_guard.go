package server

import (
	"sync"
	"time"
)

type LoginGuard struct {
	mu               sync.Mutex
	failureThreshold int
	window           time.Duration
	initialLock      time.Duration
	maxLock          time.Duration
	entries          map[string]*loginGuardEntry
}

type loginGuardEntry struct {
	Failures        int
	WindowExpiresAt time.Time
	LockedUntil     time.Time
	LockLevel       int
}

type loginGuardResult struct {
	Locked    bool
	RetryFor  time.Duration
	Failures  int
	LockLevel int
}

func NewLoginGuard(failureThreshold int, window, initialLock, maxLock time.Duration) *LoginGuard {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if window <= 0 {
		window = 30 * time.Minute
	}
	if initialLock <= 0 {
		initialLock = 5 * time.Minute
	}
	if maxLock <= 0 || maxLock < initialLock {
		maxLock = time.Hour
	}
	return &LoginGuard{
		failureThreshold: failureThreshold,
		window:           window,
		initialLock:      initialLock,
		maxLock:          maxLock,
		entries:          map[string]*loginGuardEntry{},
	}
}

func (g *LoginGuard) Check(key string, now time.Time) loginGuardResult {
	key = normalizeGuardKey(key)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cleanupLocked(now)

	entry := g.entries[key]
	if entry == nil || !now.Before(entry.LockedUntil) {
		return loginGuardResult{}
	}
	return loginGuardResult{
		Locked:    true,
		RetryFor:  retryAfterDuration(entry.LockedUntil, now),
		Failures:  entry.Failures,
		LockLevel: entry.LockLevel,
	}
}

func (g *LoginGuard) RecordFailure(key string, now time.Time) loginGuardResult {
	key = normalizeGuardKey(key)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cleanupLocked(now)

	entry := g.entries[key]
	if entry == nil {
		entry = &loginGuardEntry{WindowExpiresAt: now.Add(g.window)}
		g.entries[key] = entry
	}
	if now.Before(entry.LockedUntil) {
		return loginGuardResult{
			Locked:    true,
			RetryFor:  retryAfterDuration(entry.LockedUntil, now),
			Failures:  entry.Failures,
			LockLevel: entry.LockLevel,
		}
	}
	if now.After(entry.WindowExpiresAt) {
		entry.Failures = 0
		entry.WindowExpiresAt = now.Add(g.window)
	}

	entry.Failures++
	entry.WindowExpiresAt = now.Add(g.window)
	if entry.Failures < g.failureThreshold {
		return loginGuardResult{Failures: entry.Failures, LockLevel: entry.LockLevel}
	}

	entry.Failures = 0
	entry.LockLevel++
	lockFor := g.lockDuration(entry.LockLevel)
	entry.LockedUntil = now.Add(lockFor)
	entry.WindowExpiresAt = entry.LockedUntil.Add(g.window)
	return loginGuardResult{
		Locked:    true,
		RetryFor:  lockFor,
		Failures:  entry.Failures,
		LockLevel: entry.LockLevel,
	}
}

func (g *LoginGuard) RecordSuccess(key string) {
	key = normalizeGuardKey(key)
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}

func (g *LoginGuard) lockDuration(level int) time.Duration {
	if level <= 1 {
		return g.initialLock
	}
	lockFor := g.initialLock
	for i := 1; i < level; i++ {
		if lockFor >= g.maxLock/2 {
			return g.maxLock
		}
		lockFor *= 2
	}
	if lockFor > g.maxLock {
		return g.maxLock
	}
	return lockFor
}

func (g *LoginGuard) cleanupLocked(now time.Time) {
	for key, entry := range g.entries {
		if now.Before(entry.LockedUntil) {
			continue
		}
		if now.After(entry.WindowExpiresAt) {
			delete(g.entries, key)
		}
	}
}

func normalizeGuardKey(key string) string {
	if key == "" {
		return "unknown"
	}
	return key
}

func retryAfterDuration(until, now time.Time) time.Duration {
	if !until.After(now) {
		return 0
	}
	return until.Sub(now)
}

func retryAfterSeconds(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}
