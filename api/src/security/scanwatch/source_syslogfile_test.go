package scanwatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEmitCompleteLines_SplitsOnNewline(t *testing.T) {
	s := newSyslogFileSource(nil, "unused")
	s.partial = []byte("first\nsecond\nthird-no-newline-yet")

	s.emitCompleteLines()

	drain(t, s.lines, []string{"first", "second"})
	if string(s.partial) != "third-no-newline-yet" {
		t.Fatalf("partial = %q, want the trailing incomplete line preserved", s.partial)
	}
}

// TestEmitCompleteLines_LineSplitAcrossTwoAppends is the case a naive
// per-tick bufio.Reader approach gets wrong: a line's bytes arrive
// split across two separate reads (poll ticks), and the trailing
// partial bytes from the first read must combine correctly with the
// second read's bytes rather than being silently dropped.
func TestEmitCompleteLines_LineSplitAcrossTwoAppends(t *testing.T) {
	s := newSyslogFileSource(nil, "unused")

	s.partial = append(s.partial, []byte("coco-portscan: SRC=203.0.113.7 DPT=")...)
	s.emitCompleteLines()
	select {
	case line := <-s.lines:
		t.Fatalf("no complete line should have been emitted yet, got %q", line)
	default:
	}

	s.partial = append(s.partial, []byte("22\n")...)
	s.emitCompleteLines()
	drain(t, s.lines, []string{"coco-portscan: SRC=203.0.113.7 DPT=22"})
}

func TestSyslogFileSource_TailsAppendedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages")
	if err := os.WriteFile(path, []byte("old line before start\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	s := newSyslogFileSource(nil, path)
	// Poll fast enough for a sub-second test without being flaky.
	origInterval := syslogPollInterval
	syslogPollInterval = 20 * time.Millisecond
	defer func() { syslogPollInterval = origInterval }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := f.WriteString("new line after start\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	f.Close()

	drain(t, s.Lines(), []string{"new line after start"})
}

func TestSyslogFileSource_DetectsRotationAndReopens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	s := newSyslogFileSource(nil, path)
	origInterval := syslogPollInterval
	syslogPollInterval = 20 * time.Millisecond
	defer func() { syslogPollInterval = origInterval }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// logrotate-style rotation: rename the old file away, create a
	// fresh one at the same path.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatalf("rename (simulated rotation): %v", err)
	}
	if err := os.WriteFile(path, []byte("line in rotated-in file\n"), 0644); err != nil {
		t.Fatalf("create rotated-in file: %v", err)
	}

	drain(t, s.Lines(), []string{"line in rotated-in file"})
}
