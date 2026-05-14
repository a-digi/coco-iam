package consumer

import (
	"context"
	"time"

	"github.com/a-digi/coco-iam/src/mail/store"
	"github.com/a-digi/coco-logger/logger"
)

// retentionInterval is how often the mail_outbound retention sweep runs.
const retentionInterval = 10 * time.Minute

// retentionAge is the maximum age terminal rows are retained before being
// deleted from mail_outbound. Queue payload files live under the queue's
// own retention (250k global cap) and are cleaned separately.
const retentionAge = 30 * 24 * time.Hour

// StartRetention launches the goroutine that periodically prunes terminal
// mail_outbound rows older than retentionAge. Runs until ctx is cancelled.
func StartRetention(ctx context.Context, st *store.Store, log logger.Logger) {
	go func() {
		ticker := time.NewTicker(retentionInterval)
		defer ticker.Stop()
		// Run once shortly after boot rather than waiting a full interval.
		sweep(st, log)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep(st, log)
			}
		}
	}()
}

func sweep(st *store.Store, log logger.Logger) {
	n, err := st.PruneTerminal(retentionAge)
	if err != nil {
		log.Warning("mail retention: prune failed: %v", err)
		return
	}
	if n > 0 {
		log.Info("mail retention: pruned %d terminal rows older than %s", n, retentionAge)
	}
}
