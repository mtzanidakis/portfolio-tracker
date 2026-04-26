package auth

import (
	"sync"
	"time"
)

// LastUsedThrottler rate-limits "touch this token" writes per token id.
// A busy CLI hitting the API every second would otherwise generate one
// UPDATE per request — the throttler collapses bursts into one write per
// minute per token. The granularity ('last_used_at' is a minute-bucket
// approximation, not a precise audit trail) is intentional.
type LastUsedThrottler struct {
	window time.Duration
	mu     sync.Mutex
	last   map[int64]time.Time
}

// NewLastUsedThrottler returns a throttler that fires at most once per
// window per id. Use 1 * time.Minute in production.
func NewLastUsedThrottler(window time.Duration) *LastUsedThrottler {
	return &LastUsedThrottler{
		window: window,
		last:   make(map[int64]time.Time),
	}
}

// Allow records "now" and returns true if the caller should perform the
// underlying write. Returns false when another writer touched this id
// within the throttle window.
func (t *LastUsedThrottler) Allow(id int64, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if prev, ok := t.last[id]; ok && now.Sub(prev) < t.window {
		return false
	}
	t.last[id] = now
	return true
}

// Forget drops the throttle entry for id, e.g. after a token is
// revoked or deleted. Callers can also rely on the natural eviction
// pressure of the map staying small.
func (t *LastUsedThrottler) Forget(id int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, id)
}
