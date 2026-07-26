package ipguard

import (
	"context"
	"time"

	"github.com/a-digi/coco-logger/logger"
)

// sweepInterval matches the existing ticker-goroutine convention in
// this codebase (e.g. api/src/oauthserver/archiver, 10 minutes) —
// frequent enough that a lifted ban or a stale counter doesn't linger
// long, infrequent enough not to matter for load. See
// plan/ip-abuse-protection/plan.md section 6.
const sweepInterval = 5 * time.Minute

// Sweeper periodically prunes expired bans (DB + in-memory) and stale
// in-memory rate-limit counters, and flushes/closes attack episodes
// (section 11), so none of them grow unbounded.
type Sweeper struct {
	guard    *IPGuardSecurityLayer
	log      logger.Logger
	interval time.Duration
}

// NewSweeper builds a Sweeper on the standard interval — the intended
// construction path from main.go.
func NewSweeper(guard *IPGuardSecurityLayer, log logger.Logger) *Sweeper {
	return NewSweeperWithInterval(guard, log, sweepInterval)
}

// NewSweeperWithInterval builds a Sweeper on a custom interval, for
// deterministic tests that can't wait out the real 5-minute default.
func NewSweeperWithInterval(guard *IPGuardSecurityLayer, log logger.Logger, interval time.Duration) *Sweeper {
	return &Sweeper{guard: guard, log: log, interval: interval}
}

// Run ticks on the configured interval until ctx is done, pruning
// expired bans and stale counters on each tick. A failed prune is
// logged and does not stop future ticks.
func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep()
		}
	}
}

func (s *Sweeper) sweep() {
	if err := s.guard.PruneExpiredBans(); err != nil && s.log != nil {
		s.log.Error("ipguard: sweep: prune expired bans: %v", err)
	}
	s.guard.PruneStaleCounters()
	s.guard.FlushAttacks()
}
