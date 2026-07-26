// Package ipguard implements IP-based rate limiting and abuse
// protection. See plan/ip-abuse-protection/plan.md for the full design.
//
// This file is pure logic — a fixed-window request counter keyed by
// an arbitrary string. Callers combine an IP and a policy tier
// ("<ip>:<tier>") into that key so unrelated tiers never share a
// window. No SQL, no HTTP — safe to unit test with an injected clock.
package ipguard

import (
	"sync"
	"time"
)

// Clock abstracts time.Now so tests can advance time deterministically
// instead of sleeping.
type Clock func() time.Time

// Limiter is a fixed-window counter. Each key gets its own window and
// its own mutex, so a hot key never contends with an unrelated one.
type Limiter struct {
	now     Clock
	entries sync.Map // string -> *window
}

type window struct {
	mu    sync.Mutex
	start time.Time
	count int
}

// NewLimiter builds a Limiter using the real wall clock.
func NewLimiter() *Limiter {
	return NewLimiterWithClock(time.Now)
}

// NewLimiterWithClock builds a Limiter with an injected clock, for
// deterministic tests.
func NewLimiterWithClock(clock Clock) *Limiter {
	return &Limiter{now: clock}
}

// Allow increments the counter for key and reports whether it is
// still within limit for the current window. Once the count exceeds
// limit, every subsequent call within the same window also reports
// false — the count is not decremented or capped, so callers can
// still read how far over the limit a key has gone.
func (l *Limiter) Allow(key string, limit int, windowSize time.Duration) bool {
	v, _ := l.entries.LoadOrStore(key, &window{start: l.now()})
	w := v.(*window)

	w.mu.Lock()
	defer w.mu.Unlock()

	now := l.now()
	if now.Sub(w.start) > windowSize {
		w.start = now
		w.count = 0
	}
	w.count++
	return w.count <= limit
}

// Count reports the current window's count for key without
// incrementing it. Returns 0 for a key that has never been seen.
func (l *Limiter) Count(key string) int {
	v, ok := l.entries.Load(key)
	if !ok {
		return 0
	}
	w := v.(*window)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

// Prune evicts entries whose window has been stale for longer than
// maxAge, bounding memory growth from IPs that stop sending traffic.
func (l *Limiter) Prune(maxAge time.Duration) {
	now := l.now()
	l.entries.Range(func(k, v interface{}) bool {
		w := v.(*window)
		w.mu.Lock()
		stale := now.Sub(w.start) > maxAge
		w.mu.Unlock()
		if stale {
			l.entries.Delete(k)
		}
		return true
	})
}
