package scanwatch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/a-digi/coco-logger/logger"
)

// journaldReaderFunc is the seam that lets tests substitute a fake
// stream of lines instead of really starting journalctl — same
// injectable-seam pattern as firewall's commandRunner.
type journaldReaderFunc func(ctx context.Context, log logger.Logger) (io.ReadCloser, error)

type journaldSource struct {
	log        logger.Logger
	readerFunc journaldReaderFunc
	lines      chan string
}

// detectJournald returns nil (not a Source) when journalctl isn't on
// PATH, so Detect can fall through to the next candidate.
func detectJournald(log logger.Logger, lookPath lookPathFunc) Source {
	if _, err := lookPath("journalctl"); err != nil {
		return nil
	}
	return newJournaldSource(log, realJournaldReader)
}

func newJournaldSource(log logger.Logger, readerFunc journaldReaderFunc) *journaldSource {
	return &journaldSource{log: log, readerFunc: readerFunc, lines: make(chan string, lineChanBuffer)}
}

// realJournaldReader streams the kernel ring buffer (-k) from now
// onward (-f --since now — no historical backlog), stripping
// journald's own metadata (-o cat) so callers see exactly the raw line
// iptables' LOG target produced, the same text a bare syslog file
// would contain. ctx cancellation kills the subprocess via
// CommandContext; the reap goroutine only warns on an unexpected exit
// (ctx.Err() == nil), not on the expected kill-on-shutdown case.
func realJournaldReader(ctx context.Context, log logger.Logger) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "journalctl", "-k", "-f", "-o", "cat", "--since", "now")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("scanwatch: journalctl stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("scanwatch: journalctl start: %w", err)
	}
	go func() {
		if err := cmd.Wait(); err != nil && log != nil && ctx.Err() == nil {
			log.Warning("scanwatch: journalctl exited unexpectedly: %v", err)
		}
	}()
	return stdout, nil
}

func (s *journaldSource) Start(ctx context.Context) error {
	reader, err := s.readerFunc(ctx, s.log)
	if err != nil {
		return err
	}
	go scanLinesToChan(reader, s.lines)
	return nil
}

// scanLinesToChan copies complete lines from r to out until r is
// exhausted (EOF, or the process behind it was killed via ctx),
// closing out and r when done. A full channel drops the line rather
// than blocking — a burst of scan traffic must never stall the reader.
func scanLinesToChan(r io.ReadCloser, out chan<- string) {
	defer close(out)
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case out <- scanner.Text():
		default:
		}
	}
}

func (s *journaldSource) Lines() <-chan string { return s.lines }
func (s *journaldSource) Name() string         { return "journald" }
func (s *journaldSource) Available() bool      { return true }
func (s *journaldSource) Detail() string       { return "" }
