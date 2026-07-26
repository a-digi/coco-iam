package scanwatch

import (
	"sync"
	"testing"
	"time"
)

// fakePersister is a scanPersister test double recording every call —
// lets Watcher's aggregation logic be tested without a real
// *dbhandle.Handle/sqlite file.
type fakePersister struct {
	mu sync.Mutex

	created []createCall
	updated []updateCall
	closed  []closeCall

	createErr error
}

type createCall struct {
	id, ip    string
	startedAt time.Time
}
type updateCall struct {
	id                      string
	distinctPorts, hitCount int
	samplePorts             string
	lastSeenAt              time.Time
}
type closeCall struct {
	id      string
	endedAt time.Time
}

func (f *fakePersister) CreateScan(id, ip string, startedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, createCall{id, ip, startedAt})
	return f.createErr
}

func (f *fakePersister) UpdateScan(id string, distinctPorts, hitCount int, samplePorts string, lastSeenAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, updateCall{id, distinctPorts, hitCount, samplePorts, lastSeenAt})
	return nil
}

func (f *fakePersister) CloseScan(id string, endedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = append(f.closed, closeCall{id, endedAt})
	return nil
}

func (f *fakePersister) CloseAllOpen() (int64, error) {
	return 0, nil
}

func (f *fakePersister) createCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.created)
}

func mustWatcher(t *testing.T, persist scanPersister, threshold int, window time.Duration) *Watcher {
	t.Helper()
	w, err := NewWatcher(persist, threshold, window, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	return w
}

func TestRecordHit_BelowThresholdDoesNotCreateEpisode(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 5, time.Minute)

	now := time.Now()
	for _, port := range []int{22, 80, 443, 3306} { // 4 ports, threshold is 5
		w.RecordHit("203.0.113.7", port, now)
	}

	if got := persist.createCount(); got != 0 {
		t.Fatalf("CreateScan calls = %d, want 0 (below threshold)", got)
	}
}

func TestRecordHit_CrossingThresholdCreatesEpisodeWithFirstHitTimestamp(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 5, time.Minute)

	base := time.Now()
	ports := []int{22, 80, 443, 3306, 8080} // exactly the threshold
	for i, port := range ports {
		w.RecordHit("203.0.113.7", port, base.Add(time.Duration(i)*time.Second))
	}

	if got := persist.createCount(); got != 1 {
		t.Fatalf("CreateScan calls = %d, want exactly 1", got)
	}
	call := persist.created[0]
	if call.ip != "203.0.113.7" {
		t.Fatalf("CreateScan ip = %q, want %q", call.ip, "203.0.113.7")
	}
	if !call.startedAt.Equal(base) {
		t.Fatalf("CreateScan startedAt = %v, want %v (the first hit, not the one crossing threshold)", call.startedAt, base)
	}
}

func TestRecordHit_AdditionalHitsAfterThresholdDoNotCreateAgain(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 3, time.Minute)

	now := time.Now()
	for _, port := range []int{22, 80, 443, 8080, 9090} { // 3 crosses threshold, 2 more after
		w.RecordHit("203.0.113.7", port, now)
	}

	if got := persist.createCount(); got != 1 {
		t.Fatalf("CreateScan calls = %d, want exactly 1 (one episode, not one per port)", got)
	}
}

func TestRecordHit_StaleWindowResetsPreEpisodeState(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 3, time.Minute)

	base := time.Now()
	w.RecordHit("203.0.113.7", 22, base)
	w.RecordHit("203.0.113.7", 80, base.Add(30*time.Second)) // 2 distinct ports, still below threshold

	// A long gap passes — the pre-episode window should reset rather
	// than let these two old ports count toward a threshold crossed
	// much later.
	late := base.Add(time.Hour)
	w.RecordHit("203.0.113.7", 443, late)

	if got := persist.createCount(); got != 0 {
		t.Fatalf("CreateScan calls = %d, want 0 (only 1 distinct port in the fresh window)", got)
	}
}

func TestRecordHit_DifferentIPsTrackedIndependently(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 2, time.Minute)

	now := time.Now()
	w.RecordHit("203.0.113.7", 22, now)
	w.RecordHit("198.51.100.9", 80, now)

	if got := persist.createCount(); got != 0 {
		t.Fatalf("CreateScan calls = %d, want 0 (each IP has only 1 distinct port so far)", got)
	}

	w.RecordHit("203.0.113.7", 443, now)
	if got := persist.createCount(); got != 1 {
		t.Fatalf("CreateScan calls = %d, want 1 (only 203.0.113.7 crossed threshold)", got)
	}
}

func TestFlush_WritesCurrentTotals(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 3, time.Minute)

	now := time.Now()
	for _, port := range []int{22, 80, 443, 8080} {
		w.RecordHit("203.0.113.7", port, now)
	}
	w.Flush(now)

	if len(persist.updated) != 1 {
		t.Fatalf("UpdateScan calls = %d, want 1", len(persist.updated))
	}
	u := persist.updated[0]
	if u.distinctPorts != 4 {
		t.Fatalf("distinctPorts = %d, want 4", u.distinctPorts)
	}
	if u.hitCount != 4 {
		t.Fatalf("hitCount = %d, want 4", u.hitCount)
	}
	if u.samplePorts != "22,80,443,8080" {
		t.Fatalf("samplePorts = %q, want %q (sorted)", u.samplePorts, "22,80,443,8080")
	}
}

func TestFlush_ClosesEpisodeAfterGracePeriod(t *testing.T) {
	persist := &fakePersister{}
	window := 10 * time.Millisecond
	w := mustWatcher(t, persist, 2, window)

	base := time.Now()
	w.RecordHit("203.0.113.7", 22, base)
	w.RecordHit("203.0.113.7", 80, base)

	past2xWindow := base.Add(3 * window) // > 2x window since last hit
	w.Flush(past2xWindow)

	if len(persist.closed) != 1 {
		t.Fatalf("CloseScan calls = %d, want 1", len(persist.closed))
	}

	// The episode was evicted from memory — a later hit from the same
	// IP must start a brand new episode, not reopen the closed one.
	w.RecordHit("203.0.113.7", 22, past2xWindow)
	w.RecordHit("203.0.113.7", 443, past2xWindow)
	if got := persist.createCount(); got != 2 {
		t.Fatalf("CreateScan calls = %d, want 2 (a new episode, not a reopened one)", got)
	}
}

func TestFlush_DoesNotCloseWithinGracePeriod(t *testing.T) {
	persist := &fakePersister{}
	window := time.Hour
	w := mustWatcher(t, persist, 2, window)

	now := time.Now()
	w.RecordHit("203.0.113.7", 22, now)
	w.RecordHit("203.0.113.7", 80, now)
	w.Flush(now.Add(time.Minute))

	if len(persist.closed) != 0 {
		t.Fatalf("CloseScan calls = %d, want 0 (well within the grace period)", len(persist.closed))
	}
	if len(persist.updated) != 1 {
		t.Fatalf("UpdateScan calls = %d, want 1", len(persist.updated))
	}
}

func TestFlush_EvictsStalePreEpisodeStateWithoutPersisting(t *testing.T) {
	persist := &fakePersister{}
	window := 10 * time.Millisecond
	w := mustWatcher(t, persist, 5, window)

	now := time.Now()
	w.RecordHit("203.0.113.7", 22, now) // only 1 distinct port — never crosses threshold=5

	w.Flush(now.Add(time.Hour))

	if got := persist.createCount(); got != 0 {
		t.Fatalf("CreateScan calls = %d, want 0", got)
	}
	if len(persist.updated) != 0 {
		t.Fatalf("UpdateScan calls = %d, want 0 (never became a real episode)", len(persist.updated))
	}
	if len(persist.closed) != 0 {
		t.Fatalf("CloseScan calls = %d, want 0 (never became a real episode)", len(persist.closed))
	}
}

func TestConsume_ParsesLinesAndCreatesEpisode(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 3, time.Minute)

	lines := make(chan string, 10)
	lines <- "coco-portscan: SRC=203.0.113.7 PROTO=TCP DPT=22"
	lines <- "unrelated kernel line that must be ignored"
	lines <- "coco-portscan: SRC=203.0.113.7 PROTO=TCP DPT=80"
	lines <- "coco-portscan: SRC=203.0.113.7 PROTO=TCP DPT=443"
	close(lines)

	w.Consume(lines, "coco-portscan: ")

	if got := persist.createCount(); got != 1 {
		t.Fatalf("CreateScan calls = %d, want 1", got)
	}
	if persist.created[0].ip != "203.0.113.7" {
		t.Fatalf("created ip = %q, want %q", persist.created[0].ip, "203.0.113.7")
	}
}

// TestRecordHit_ConcurrentHitsForSameIPAreCountedExactly exercises the
// actual hot path under real parallel load (run with -race) — a real
// scan generates many packets in quick succession, arriving on
// whatever goroutine Consume happens to be processing them on.
func TestRecordHit_ConcurrentHitsForSameIPAreCountedExactly(t *testing.T) {
	persist := &fakePersister{}
	w := mustWatcher(t, persist, 3, time.Minute)

	now := time.Now()
	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(port int) {
			defer wg.Done()
			w.RecordHit("203.0.113.7", port, now)
		}(i)
	}
	wg.Wait()

	if got := persist.createCount(); got != 1 {
		t.Fatalf("CreateScan calls = %d, want exactly 1 (never double-created under concurrency)", got)
	}
}
