package scanwatch

import (
	"os"
	"testing"
)

func fakeLookPath(found bool) lookPathFunc {
	return func(file string) (string, error) {
		if found {
			return "/usr/bin/" + file, nil
		}
		return "", os.ErrNotExist
	}
}

func fakeStat(found bool) statFunc {
	return func(name string) (os.FileInfo, error) {
		if found {
			return nil, nil // detect only checks the error, never the FileInfo itself
		}
		return nil, os.ErrNotExist
	}
}

func TestDetect_PrefersJournaldWhenAvailable(t *testing.T) {
	s := detect(nil, "/var/log/messages", fakeLookPath(true), fakeStat(true))
	if s.Name() != "journald" {
		t.Fatalf("Name() = %q, want %q (journald should win even when a syslog file also exists)", s.Name(), "journald")
	}
	if !s.Available() {
		t.Fatal("Available() = false, want true")
	}
}

func TestDetect_FallsBackToSyslogFile(t *testing.T) {
	s := detect(nil, "/var/log/messages", fakeLookPath(false), fakeStat(true))
	if s.Name() != "syslog_file" {
		t.Fatalf("Name() = %q, want %q", s.Name(), "syslog_file")
	}
	if !s.Available() {
		t.Fatal("Available() = false, want true")
	}
}

func TestDetect_NoopWhenNeitherAvailable(t *testing.T) {
	s := detect(nil, "/var/log/messages", fakeLookPath(false), fakeStat(false))
	if s.Name() != "none" {
		t.Fatalf("Name() = %q, want %q", s.Name(), "none")
	}
	if s.Available() {
		t.Fatal("Available() = true, want false")
	}
	if s.Detail() == "" {
		t.Fatal("Detail() should explain why nothing was found")
	}
}
