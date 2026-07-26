package scanwatch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/a-digi/coco-logger/logger"
)

// syslogPollInterval bounds how stale a scan alert can be on the
// syslog-file path (Alpine, no journald to -f) — cheap enough to poll
// often since each tick is just a non-blocking Read plus an os.Stat.
// A var (not const) so tests can shorten it instead of waiting out a
// real production-length interval.
var syslogPollInterval = 2 * time.Second

// syslogFileSource tails path by polling — no inotify dependency, so
// it works identically across every Linux libc/kernel combination
// this feature targets (Ubuntu glibc, Alpine musl). Rotation
// (logrotate replacing the file) is detected via os.SameFile against
// a fresh os.Stat each tick, not by fixed handle: once detected, the
// old handle is closed and a new one opened from the start of the
// rotated-in file.
type syslogFileSource struct {
	log     logger.Logger
	path    string
	lines   chan string
	partial []byte // bytes read but not yet terminated by \n, carried to the next tick
}

func newSyslogFileSource(log logger.Logger, path string) *syslogFileSource {
	return &syslogFileSource{log: log, path: path, lines: make(chan string, lineChanBuffer)}
}

func (s *syslogFileSource) Start(ctx context.Context) error {
	f, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("scanwatch: open %s: %w", s.path, err)
	}
	// Start from the tail — this feature cares about scans from now
	// on, not replaying the file's entire history on every restart.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		_ = f.Close()
		return fmt.Errorf("scanwatch: seek %s: %w", s.path, err)
	}
	go s.tail(ctx, f)
	return nil
}

// tail reads directly via f.Read rather than through a bufio.Reader
// kept alive across ticks — a bufio.Reader can read ahead past what's
// actually available and there is no safe way to "un-read" those
// bytes back into the file for the next tick, so a naive
// ReadString('\n')-per-tick approach silently drops the tail end of
// a line that straddles two polls. Reading raw bytes into s.partial
// and only ever consuming up to the last '\n' avoids that entirely.
func (s *syslogFileSource) tail(ctx context.Context, f *os.File) {
	defer close(s.lines)
	defer f.Close()

	fi, _ := f.Stat()
	ticker := time.NewTicker(syslogPollInterval)
	defer ticker.Stop()
	buf := make([]byte, 32*1024)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				n, err := f.Read(buf)
				if n > 0 {
					s.partial = append(s.partial, buf[:n]...)
					s.emitCompleteLines()
				}
				if err != nil {
					break
				}
			}
			f, fi = s.reopenIfRotated(f, fi)
		}
	}
}

// reopenIfRotated compares the currently-open file's identity against
// a fresh stat of the path — logrotate replaces the file at that path
// with a new inode, which os.SameFile detects portably (no raw
// syscall.Stat_t handling needed). A rotated-in file starts fresh, so
// any carried-over partial bytes from the old file are dropped rather
// than incorrectly prefixed onto the new file's first line.
func (s *syslogFileSource) reopenIfRotated(f *os.File, fi os.FileInfo) (*os.File, os.FileInfo) {
	newInfo, err := os.Stat(s.path)
	if err != nil || fi == nil || os.SameFile(fi, newInfo) {
		return f, fi
	}
	newF, err := os.Open(s.path)
	if err != nil {
		if s.log != nil {
			s.log.Warning("scanwatch: reopen %s after rotation: %v", s.path, err)
		}
		return f, fi
	}
	_ = f.Close()
	s.partial = s.partial[:0]
	return newF, newInfo
}

func (s *syslogFileSource) emitCompleteLines() {
	for {
		idx := bytes.IndexByte(s.partial, '\n')
		if idx < 0 {
			return
		}
		line := string(s.partial[:idx])
		s.partial = s.partial[idx+1:]
		select {
		case s.lines <- line:
		default:
		}
	}
}

func (s *syslogFileSource) Lines() <-chan string { return s.lines }
func (s *syslogFileSource) Name() string         { return "syslog_file" }
func (s *syslogFileSource) Available() bool      { return true }
func (s *syslogFileSource) Detail() string       { return "" }
