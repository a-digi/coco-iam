package consumer

import (
	"context"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/store"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-logger/logger"
)

// OrchestratorConfig captures the dynamic-worker tuning knobs. Defaults
// match the approved plan: MIN=1, MAX=20, STEP=100, every 15 s.
type OrchestratorConfig struct {
	Min      int
	Max      int
	Step     int
	Interval time.Duration
}

// OrchestratorConfigFromEnv reads MAIL_MIN_WORKERS / MAIL_MAX_WORKERS /
// MAIL_WORKERS_STEP and falls back to approved defaults.
func OrchestratorConfigFromEnv() OrchestratorConfig {
	return OrchestratorConfig{
		Min:      envInt("MAIL_MIN_WORKERS", 1),
		Max:      envInt("MAIL_MAX_WORKERS", 20),
		Step:     envInt("MAIL_WORKERS_STEP", 100),
		Interval: 15 * time.Second,
	}
}

// Orchestrator owns the worker-pool sizing loop for the mail-outbound queue.
type Orchestrator struct {
	queue queue.Manager
	store *store.Store
	log   logger.Logger
	cfg   OrchestratorConfig

	// atomic int32 so the HTTP status handler can read the current size
	// without contending with the sizing goroutine.
	current atomic.Int32
	backlog atomic.Int32
}

// NewOrchestrator builds the goroutine orchestrator.
func NewOrchestrator(q queue.Manager, st *store.Store, cfg OrchestratorConfig, log logger.Logger) *Orchestrator {
	if cfg.Min <= 0 {
		cfg.Min = 1
	}
	if cfg.Max < cfg.Min {
		cfg.Max = cfg.Min
	}
	if cfg.Step <= 0 {
		cfg.Step = 100
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 15 * time.Second
	}
	return &Orchestrator{queue: q, store: st, log: log, cfg: cfg}
}

// Start runs the sizing loop until ctx is cancelled. The first tick applies
// immediately so the pool doesn't sit at its register-time size when there's
// already a backlog at boot.
func (o *Orchestrator) Start(ctx context.Context) {
	go func() {
		o.apply(ctx)
		ticker := time.NewTicker(o.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				o.apply(ctx)
			}
		}
	}()
}

// Snapshot is the read-only view of the orchestrator's current decision —
// used by GET /admin/mail/status.
type Snapshot struct {
	Backlog int `json:"backlog"`
	Workers int `json:"workers"`
	Min     int `json:"min"`
	Max     int `json:"max"`
	Step    int `json:"step"`
}

// Current returns the latest sampled state.
func (o *Orchestrator) Current() Snapshot {
	return Snapshot{
		Backlog: int(o.backlog.Load()),
		Workers: int(o.current.Load()),
		Min:     o.cfg.Min,
		Max:     o.cfg.Max,
		Step:    o.cfg.Step,
	}
}

func (o *Orchestrator) apply(ctx context.Context) {
	backlog, err := o.store.CountActive(ctx)
	if err != nil {
		o.log.Warning("mail orchestrator: count active failed: %v", err)
		return
	}
	o.backlog.Store(int32(backlog))

	desired := backlog / o.cfg.Step
	if backlog > 0 && desired < o.cfg.Min {
		desired = o.cfg.Min
	}
	if desired < o.cfg.Min {
		desired = o.cfg.Min
	}
	if desired > o.cfg.Max {
		desired = o.cfg.Max
	}

	if int(o.current.Load()) == desired {
		return
	}
	if err := o.queue.SetWorkers(iam_mail.QueueNameOutbound, desired); err != nil {
		o.log.Warning("mail orchestrator: SetWorkers(%d) failed: %v", desired, err)
		return
	}
	o.current.Store(int32(desired))
	o.log.Info("mail orchestrator: backlog=%d → workers=%d (min=%d max=%d step=%d)",
		backlog, desired, o.cfg.Min, o.cfg.Max, o.cfg.Step)
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// ContextBagKeyOrchestrator is where main.go stows the running orchestrator
// so the /admin/mail/status handler can fetch its snapshot.
const ContextBagKeyOrchestrator = "mail.Orchestrator"
