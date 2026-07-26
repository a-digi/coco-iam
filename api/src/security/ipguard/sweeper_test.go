package ipguard

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/a-digi/coco-server/server/security"
)

func TestSweeper_PrunesExpiredBanOnTick(t *testing.T) {
	g := newTestGuard(t, testConfig())
	if err := g.Ban("203.0.113.7", "global", "test", -time.Minute, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}

	sweeper := NewSweeperWithInterval(g, nil, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sweeper.Run(ctx)

	bans, err := g.ListBans()
	if err != nil {
		t.Fatalf("ListBans() error = %v", err)
	}
	if len(bans) != 0 {
		t.Fatalf("ListBans() after sweep = %+v, want empty (expired ban should be pruned)", bans)
	}

	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/x"), nil, &security.Route{Path: "/x"}); err != nil {
		t.Fatalf("expired ban should no longer block after sweep, error = %v", err)
	}
}

func TestSweeper_LeavesActiveBanAlone(t *testing.T) {
	g := newTestGuard(t, testConfig())
	if err := g.Ban("203.0.113.7", "global", "test", time.Hour, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}

	sweeper := NewSweeperWithInterval(g, nil, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sweeper.Run(ctx)

	bans, err := g.ListBans()
	if err != nil {
		t.Fatalf("ListBans() error = %v", err)
	}
	if len(bans) != 1 {
		t.Fatalf("ListBans() after sweep = %+v, want the still-active ban untouched", bans)
	}
}

func TestSweeper_StopsWhenContextCancelled(t *testing.T) {
	g := newTestGuard(t, testConfig())
	sweeper := NewSweeperWithInterval(g, nil, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sweeper.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not return promptly after context cancellation")
	}
}
