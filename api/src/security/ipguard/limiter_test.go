package ipguard

import (
	"testing"
	"time"
)

func TestLimiter_AllowsUpToLimitThenDenies(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	l := NewLimiterWithClock(clock)

	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4:global", 3, time.Minute) {
			t.Fatalf("call %d: expected allowed within limit", i+1)
		}
	}
	if l.Allow("1.2.3.4:global", 3, time.Minute) {
		t.Fatal("4th call: expected denied, limit is 3")
	}
	if got := l.Count("1.2.3.4:global"); got != 4 {
		t.Fatalf("Count() = %d, want 4 (increments even once over limit)", got)
	}
}

func TestLimiter_WindowResetsAfterElapsed(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	l := NewLimiterWithClock(clock)

	for i := 0; i < 3; i++ {
		l.Allow("1.2.3.4:global", 3, time.Minute)
	}
	if l.Allow("1.2.3.4:global", 3, time.Minute) {
		t.Fatal("expected denied before window elapses")
	}

	now = now.Add(2 * time.Minute)
	if !l.Allow("1.2.3.4:global", 3, time.Minute) {
		t.Fatal("expected allowed again once the window has elapsed")
	}
	if got := l.Count("1.2.3.4:global"); got != 1 {
		t.Fatalf("Count() = %d, want 1 (window reset)", got)
	}
}

func TestLimiter_KeysAreIndependent(t *testing.T) {
	l := NewLimiter()

	for i := 0; i < 5; i++ {
		l.Allow("1.2.3.4:global", 3, time.Minute)
	}
	if !l.Allow("5.6.7.8:global", 3, time.Minute) {
		t.Fatal("a different key must not be affected by another key's count")
	}
	if !l.Allow("1.2.3.4:sensitive", 3, time.Minute) {
		t.Fatal("a different tier for the same IP must not share a window")
	}
}

func TestLimiter_CountUnknownKeyIsZero(t *testing.T) {
	l := NewLimiter()
	if got := l.Count("nope"); got != 0 {
		t.Fatalf("Count() for unseen key = %d, want 0", got)
	}
}

func TestLimiter_PruneEvictsStaleEntriesOnly(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	l := NewLimiterWithClock(clock)

	l.Allow("stale:global", 10, time.Minute)

	now = now.Add(30 * time.Minute)
	l.Allow("fresh:global", 10, time.Minute)

	l.Prune(time.Hour)
	if got := l.Count("stale:global"); got != 1 {
		t.Fatalf("Prune with maxAge > actual age evicted nothing unexpected, Count() = %d, want 1", got)
	}

	l.Prune(10 * time.Minute)
	if got := l.Count("stale:global"); got != 0 {
		t.Fatalf("stale key survived Prune, Count() = %d, want 0 (evicted)", got)
	}
	if got := l.Count("fresh:global"); got != 1 {
		t.Fatalf("fresh key was wrongly evicted, Count() = %d, want 1", got)
	}
}
