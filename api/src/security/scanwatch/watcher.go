package scanwatch

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/a-digi/coco-logger/logger"
)

// DefaultThreshold and DefaultWindow are the "N distinct ports within
// a window" signature that distinguishes a scan from a single stray
// packet to one closed port — see
// plan/port-scan-detection/plan.md Phase B's episode-heuristic
// section. Both are constructor parameters, not hardcoded, so an
// operator can tune them without a code change.
const (
	DefaultThreshold = 5
	DefaultWindow    = 5 * time.Minute
)

// sweepInterval mirrors dbarchive.Archiver's own tick — cheap to poll
// often since a no-op tick is just a map scan.
const sweepInterval = 1 * time.Minute

// scanPersister is the narrow slice of ScanPersistentRepo the Watcher
// needs — kept as an interface here (unlike ipguard's direct
// concrete-type dependency on attacks_persistent) purely so this
// package's tests don't need a real *dbhandle.Handle/sqlite file for
// every test; production wiring passes the real
// scans/repository/persistent.ScanPersistentRepo.
type scanPersister interface {
	CreateScan(id, ip string, startedAt time.Time) error
	UpdateScan(id string, distinctPorts, hitCount int, samplePorts string, lastSeenAt time.Time) error
	CloseScan(id string, endedAt time.Time) error
	CloseAllOpen() (int64, error)
}

// ipState is one IP's in-progress tracking — before threshold is
// crossed this is pure in-memory bookkeeping (episodeID == ""); once
// crossed, it mirrors the corresponding scan_episodes row.
type ipState struct {
	firstHitAt time.Time
	lastHitAt  time.Time
	hitCount   int
	ports      map[int]struct{}
	episodeID  string // "" until the distinct-port threshold is crossed
}

// Watcher aggregates parsed log hits into scan episodes. Hits before
// an IP crosses the distinct-port threshold are buffered in memory
// only (see RecordHit) — a single probed port is noise, not a scan
// signature — so the eventual episode's started_at reflects the
// first real hit, not the one that crossed the threshold.
type Watcher struct {
	mu  sync.Mutex
	ips map[string]*ipState

	threshold int
	window    time.Duration

	persist scanPersister
	log     logger.Logger
}

// NewWatcher builds a Watcher and force-closes any scan_episodes row
// left open by a previous process instance — a fresh process's
// in-memory map always starts empty, and an episode is never reopened
// once closed (a later scan from the same IP is a new row — see
// RecordHit's "stale window" reset and Flush's grace-close), so any
// row still open at construction time is definitionally orphaned.
// Mirrors ipguard.hydrate's CloseAllOpen call exactly.
func NewWatcher(persist scanPersister, threshold int, window time.Duration, log logger.Logger) (*Watcher, error) {
	w := &Watcher{
		ips:       make(map[string]*ipState),
		threshold: threshold,
		window:    window,
		persist:   persist,
		log:       log,
	}
	if n, err := persist.CloseAllOpen(); err != nil {
		return nil, fmt.Errorf("scanwatch: hydrate: %w", err)
	} else if n > 0 && log != nil {
		log.Info("scanwatch: closed %d scan episode(s) left open by a previous process instance", n)
	}
	return w, nil
}

// Consume reads parsed hits from lines until the channel closes
// (meaning the underlying Source stopped, e.g. via context
// cancellation) — intended to run in its own goroutine, driven
// entirely by the Source's own lifecycle rather than its own context.
func (w *Watcher) Consume(lines <-chan string, logPrefix string) {
	for line := range lines {
		hit, ok := ParseLine(line, logPrefix)
		if !ok {
			continue
		}
		w.RecordHit(hit.IP, hit.Port, time.Now())
	}
}

// RecordHit tracks one probe against ip:port. Only once the IP has
// touched at least `threshold` distinct ports within `window` does an
// episode actually get created — every hit before that point is
// tracked but never persisted, so a single stray packet never shows up
// as an "episode" of its own.
func (w *Watcher) RecordHit(ip string, port int, now time.Time) {
	w.mu.Lock()
	state, ok := w.ips[ip]
	switch {
	case !ok:
		state = &ipState{firstHitAt: now, ports: make(map[int]struct{})}
		w.ips[ip] = state
	case state.episodeID == "" && now.Sub(state.lastHitAt) > w.window:
		// Pre-episode tracking gone stale without crossing the
		// threshold — start a fresh window instead of carrying
		// ancient ports forward indefinitely.
		state.firstHitAt = now
		state.ports = make(map[int]struct{})
		state.hitCount = 0
	}
	state.lastHitAt = now
	state.hitCount++
	state.ports[port] = struct{}{}

	needsCreate := state.episodeID == "" && len(state.ports) >= w.threshold
	if needsCreate {
		// Reserved under the lock so a concurrent hit for the same IP
		// never races into creating a second episode row — same
		// discipline as ipguard.recordAttackHit.
		state.episodeID = uuid.New().String()
	}
	episodeID := state.episodeID
	startedAt := state.firstHitAt
	w.mu.Unlock()

	if needsCreate {
		if err := w.persist.CreateScan(episodeID, ip, startedAt); err != nil && w.log != nil {
			w.log.Error("scanwatch: failed to create scan episode for %s: %v", ip, err)
		}
	}
}

// Flush writes the current in-memory totals for every open episode to
// scan_episodes, and closes any episode gone quiet past 2x the
// aggregation window (same grace convention as
// ipguard.IPGuardSecurityLayer.FlushAttacks). Pre-episode state that
// never crossed the threshold and has gone stale is simply dropped —
// nothing to persist, since it was never recorded. Intended to be
// called on a fixed interval by Run.
func (w *Watcher) Flush(now time.Time) {
	type snapshot struct {
		episodeID               string
		distinctPorts, hitCount int
		samplePorts             string
		lastSeenAt              time.Time
	}
	grace := 2 * w.window

	var snapshots []snapshot
	var toClose []string
	var toEvict []string

	w.mu.Lock()
	for ip, state := range w.ips {
		if state.episodeID == "" {
			if now.Sub(state.lastHitAt) > w.window {
				toEvict = append(toEvict, ip)
			}
			continue
		}
		snapshots = append(snapshots, snapshot{
			episodeID:     state.episodeID,
			distinctPorts: len(state.ports),
			hitCount:      state.hitCount,
			samplePorts:   formatSamplePorts(state.ports),
			lastSeenAt:    state.lastHitAt,
		})
		if now.Sub(state.lastHitAt) > grace {
			toClose = append(toClose, ip)
		}
	}
	w.mu.Unlock()

	for _, snap := range snapshots {
		if err := w.persist.UpdateScan(snap.episodeID, snap.distinctPorts, snap.hitCount, snap.samplePorts, snap.lastSeenAt); err != nil && w.log != nil {
			w.log.Error("scanwatch: failed to flush scan episode %s: %v", snap.episodeID, err)
		}
	}

	if len(toClose) > 0 {
		w.mu.Lock()
		for _, ip := range toClose {
			state, ok := w.ips[ip]
			if !ok {
				continue
			}
			if err := w.persist.CloseScan(state.episodeID, state.lastHitAt); err != nil && w.log != nil {
				w.log.Error("scanwatch: failed to close scan episode %s: %v", state.episodeID, err)
			}
			delete(w.ips, ip)
		}
		w.mu.Unlock()
	}

	if len(toEvict) > 0 {
		w.mu.Lock()
		for _, ip := range toEvict {
			delete(w.ips, ip)
		}
		w.mu.Unlock()
	}
}

// Run blocks, calling Flush every sweepInterval, until ctx is
// cancelled. Intended to be launched as a goroutine — mirrors
// dbarchive.Archiver.Run's shape. A dedicated ticker rather than
// reusing ipguard.Sweeper's, since flushing scan episodes is an
// unrelated concern from pruning bans/rate-limit counters.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Flush(time.Now())
		}
	}
}

// formatSamplePorts renders ports as a sorted, comma-separated list
// capped at 20 entries — enough to be useful on the admin detail page
// without the column growing unbounded for a scan touching thousands
// of ports.
func formatSamplePorts(ports map[int]struct{}) string {
	list := make([]int, 0, len(ports))
	for p := range ports {
		list = append(list, p)
	}
	sort.Ints(list)
	if len(list) > 20 {
		list = list[:20]
	}
	strs := make([]string, len(list))
	for i, p := range list {
		strs[i] = strconv.Itoa(p)
	}
	return strings.Join(strs, ",")
}
