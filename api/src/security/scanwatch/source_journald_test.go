package scanwatch

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/a-digi/coco-logger/logger"
)

func drain(t *testing.T, ch <-chan string, want []string) {
	t.Helper()
	var got []string
	deadline := time.After(time.Second)
	for len(got) < len(want) {
		select {
		case line, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed early, got %v, want %v", got, want)
			}
			got = append(got, line)
		case <-deadline:
			t.Fatalf("timed out waiting for lines, got %v, want %v", got, want)
		}
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("line %d = %q, want %q", i, got[i], w)
		}
	}
}

func fakeJournaldReader(lines string) journaldReaderFunc {
	return func(ctx context.Context, log logger.Logger) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(lines)), nil
	}
}

func TestJournaldSource_StreamsLinesFromReader(t *testing.T) {
	s := newJournaldSource(nil, fakeJournaldReader("line one\nline two\n"))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	drain(t, s.Lines(), []string{"line one", "line two"})
}

func TestJournaldSource_ClosesChannelWhenReaderExhausted(t *testing.T) {
	s := newJournaldSource(nil, fakeJournaldReader("only line\n"))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	drain(t, s.Lines(), []string{"only line"})

	select {
	case _, ok := <-s.Lines():
		if ok {
			t.Fatal("expected the channel to be closed once the reader is exhausted")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the channel to close")
	}
}

func TestJournaldSource_MetadataMethods(t *testing.T) {
	s := newJournaldSource(nil, fakeJournaldReader(""))
	if s.Name() != "journald" {
		t.Fatalf("Name() = %q, want %q", s.Name(), "journald")
	}
	if !s.Available() {
		t.Fatal("Available() = false, want true")
	}
	if s.Detail() != "" {
		t.Fatalf("Detail() = %q, want empty", s.Detail())
	}
}
